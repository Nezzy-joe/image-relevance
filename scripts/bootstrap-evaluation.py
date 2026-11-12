#!/usr/bin/env python3
"""Import the curated corpus, create/reuse its posts, write runtime UUID maps, and evaluate."""

from __future__ import annotations

import argparse
import concurrent.futures
import csv
import json
import os
import pathlib
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid


ROOT = pathlib.Path(__file__).resolve().parents[1]
DATA = ROOT / "evaluation-data" / "curated"
RUNTIME = DATA / "runtime"
TENANT_NAME = "image-relevance development tenant"


def compose_prefix(args: argparse.Namespace) -> list[str]:
    command = ["docker", "compose"]
    if args.project_name:
        command.extend(["--project-name", args.project_name])
    for compose_file in args.compose_file:
        command.extend(["--file", str(compose_file)])
    command.extend(["--project-directory", str(ROOT)])
    return command


def compose_run(args: argparse.Namespace, *command: str, capture: bool = False) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [*compose_prefix(args), *command],
        cwd=ROOT,
        check=True,
        text=True,
        capture_output=capture,
    )


def read_local_compose_value(key: str, fallback: str) -> str:
    """Read only a non-secret setting from .env; never parse or print credential values."""
    env_path = ROOT / ".env"
    if not env_path.is_file():
        return os.environ.get(key, fallback)
    for line in env_path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith(key + "="):
            value = stripped.split("=", 1)[1].strip().strip('"').strip("'")
            return value or fallback
    return os.environ.get(key, fallback)


def http_json(base_url: str, path: str, payload: dict | None = None, timeout: float = 90) -> tuple[int, dict]:
    body = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    headers = {} if payload is None else {"Content-Type": "application/json"}
    request = urllib.request.Request(base_url.rstrip("/") + path, data=body, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, json.loads(response.read())
    except urllib.error.HTTPError as error:
        # Provider and application response bodies are deliberately suppressed.
        raise RuntimeError(f"local API request failed with HTTP {error.code}: {path}") from None
    except (urllib.error.URLError, TimeoutError) as error:
        raise RuntimeError(f"local API request failed: {path} ({type(error).__name__})") from None


def wait_for_ready(base_url: str, deadline_seconds: int) -> None:
    deadline = time.monotonic() + deadline_seconds
    while time.monotonic() < deadline:
        try:
            status, payload = http_json(base_url, "/ready", timeout=5)
            if status == 200 and payload.get("status") == "ready":
                return
        except RuntimeError:
            pass
        time.sleep(2)
    raise RuntimeError("API readiness did not become healthy before the timeout")


def upload_one(base_url: str, row: dict[str, str]) -> dict[str, str]:
    path = DATA / "assets" / row["local_file"]
    content = path.read_bytes()
    boundary = "----image-relevance-" + uuid.uuid4().hex
    header = (
        f"--{boundary}\r\n"
        f"Content-Disposition: form-data; name=\"file\"; filename=\"{path.name}\"\r\n"
        "Content-Type: image/jpeg\r\n\r\n"
    ).encode("utf-8")
    body = header + content + f"\r\n--{boundary}--\r\n".encode("ascii")
    request = urllib.request.Request(
        base_url.rstrip("/") + "/v1/images/",
        data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
    )
    try:
        with urllib.request.urlopen(request, timeout=120) as response:
            payload = json.loads(response.read())
            if response.status not in (200, 202) or not payload.get("image_id"):
                raise RuntimeError(f"image upload returned an invalid response for {path.name}")
            return {**row, "image_id": payload["image_id"]}
    except urllib.error.HTTPError as error:
        raise RuntimeError(f"image upload failed with HTTP {error.code} for {path.name}") from None
    except urllib.error.URLError as error:
        raise RuntimeError(f"image upload failed for {path.name} ({type(error).__name__})") from None


def wait_for_images(base_url: str, images: list[dict[str, str]], timeout_seconds: int) -> None:
    pending = {row["image_id"]: row["local_file"] for row in images}
    deadline = time.monotonic() + timeout_seconds
    while pending and time.monotonic() < deadline:
        def check(image_id: str) -> tuple[str, str]:
            status, payload = http_json(base_url, f"/v1/images/{image_id}", timeout=15)
            if status != 200:
                raise RuntimeError(f"image status query returned HTTP {status}")
            return image_id, payload.get("status", "unknown")

        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
            statuses = list(executor.map(check, list(pending)))
        for image_id, status in statuses:
            if status == "processed":
                del pending[image_id]
            elif status in {"failed", "needs_review"}:
                raise RuntimeError(f"image processing ended as {status}: {pending[image_id]}")
            elif status not in {"queued", "processing"}:
                raise RuntimeError(f"unexpected image status {status}: {pending[image_id]}")
        print(f"Images processed: {len(images) - len(pending)}/{len(images)}", flush=True)
        if pending:
            time.sleep(2)
    if pending:
        raise RuntimeError(f"image processing timed out with {len(pending)} images still pending")


def find_existing_post(args: argparse.Namespace, title: str, body: str) -> str | None:
    database = read_local_compose_value("POSTGRES_DB", "flyrank")
    user = read_local_compose_value("POSTGRES_USER", "flyrank")
    quoted_title = "'" + title.replace("'", "''") + "'"
    quoted_body = "'" + body.replace("'", "''") + "'"
    quoted_tenant = "'" + TENANT_NAME.replace("'", "''") + "'"
    query = (
        "SELECT p.id::text FROM posts p JOIN tenants t ON t.id=p.tenant_id "
        f"WHERE t.name={quoted_tenant} AND t.status='active' "
        f"AND p.title={quoted_title} AND p.body={quoted_body} "
        "ORDER BY p.created_at LIMIT 1;"
    )
    result = compose_run(args, "exec", "-T", "db", "psql", "-U", user, "-d", database, "-Atc", query, capture=True)
    value = result.stdout.strip()
    return value or None


def create_or_find_posts(args: argparse.Namespace, base_url: str, articles: list[dict]) -> list[dict]:
    result_rows = []
    for article in articles:
        post_id = find_existing_post(args, article["title"], article["body"])
        created = False
        if post_id is None:
            for attempt in range(2):
                try:
                    status, payload = http_json(
                        base_url,
                        "/v1/posts",
                        {"title": article["title"], "body": article["body"]},
                        timeout=120,
                    )
                    if status != 201 or not payload.get("id"):
                        raise RuntimeError(f"post creation returned an invalid response for {article['source_key']}")
                    post_id = payload["id"]
                    created = True
                    break
                except RuntimeError as error:
                    # One bounded recovery attempt for a transient provider/API 500.
                    # Check persistence first so a lost HTTP response cannot duplicate a post.
                    if attempt != 0 or "HTTP 500" not in str(error):
                        raise
                    post_id = find_existing_post(args, article["title"], article["body"])
                    if post_id:
                        break
                    print(f"Transient post-provider failure for {article['source_key']}; retrying once.", flush=True)
                    time.sleep(2)
        result_rows.append({**article, "post_id": post_id})
        state = "created" if created else "reused"
        print(f"Post {state}: {article['source_key']}", flush=True)
    return result_rows


def write_runtime_manifests(images: list[dict[str, str]], articles: list[dict]) -> None:
    RUNTIME.mkdir(parents=True, exist_ok=True)
    category_images: dict[str, list[str]] = {}
    for image in images:
        category_images.setdefault(image["category"], []).append(image["image_id"])
    all_image_ids = [image["image_id"] for image in images]

    with (RUNTIME / "images.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["image_id", "category", "source_url", "license"])
        for image in images:
            writer.writerow([image["image_id"], image["category"], image["source_url"], image["license"]])

    with (RUNTIME / "articles.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["post_id", "expected_image_ids", "negative_image_ids", "scenario"])
        for article in articles:
            expected = category_images.get(article["category"], [])
            expected_set = set(expected)
            negative = [image_id for image_id in all_image_ids if image_id not in expected_set]
            writer.writerow([article["post_id"], "|".join(expected), "|".join(negative), article["scenario"]])

    mapping = {
        "images": {image["local_file"]: image["image_id"] for image in images},
        "posts": {article["source_key"]: article["post_id"] for article in articles},
    }
    (RUNTIME / "id-mapping.json").write_text(json.dumps(mapping, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api-url", default="http://localhost:8080")
    parser.add_argument("--project-name", help="Compose project name, for example an isolated test project")
    parser.add_argument("--compose-file", action="append", help="Compose file; repeat for an override")
    parser.add_argument("--ready-timeout", type=int, default=120)
    parser.add_argument("--image-timeout", type=int, default=900)
    args = parser.parse_args()
    if not args.compose_file:
        args.compose_file = [str(ROOT / "docker-compose.yml")]

    try:
        sources = list(csv.DictReader((DATA / "sources.csv").open(encoding="utf-8")))
        articles = json.loads((DATA / "articles.json").read_text(encoding="utf-8"))
        if len(sources) < 40 or len({row["category"] for row in sources}) < 4 or len(articles) < 10:
            raise RuntimeError("curated source files do not meet the 40-image/four-category/10-article minimum")
        source_by_file = {row["local_file"]: row for row in sources}
        for article in articles:
            if article["category"]:
                example = source_by_file.get(article["verified_positive_image_file"])
                if not example or example["category"] != article["category"]:
                    raise RuntimeError(f"verified positive example is missing or mislabeled: {article['source_key']}")
            elif article["scenario"] != "no_suitable":
                raise RuntimeError(f"only no_suitable articles may omit a category: {article['source_key']}")

        wait_for_ready(args.api_url, args.ready_timeout)
        print("Seeding or identifying the development tenant.", flush=True)
        compose_run(args, "exec", "--no-TTY", "api", "/seed")

        worker = compose_run(args, "ps", "-q", "worker", capture=True).stdout.strip()
        if not worker:
            raise RuntimeError("the AI worker is not running; start Compose with the ai profile")

        images: list[dict[str, str]] = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
            futures = [executor.submit(upload_one, args.api_url, row) for row in sources]
            for future in concurrent.futures.as_completed(futures):
                images.append(future.result())
        images.sort(key=lambda row: row["local_file"])
        print(f"Imported or identified {len(images)} curated images.", flush=True)
        wait_for_images(args.api_url, images, args.image_timeout)

        article_records = create_or_find_posts(args, args.api_url, articles)
        write_runtime_manifests(images, article_records)
        print(f"Runtime UUID mappings written under {RUNTIME.relative_to(ROOT)}.", flush=True)
        print("Running the evaluation CLI.", flush=True)
        compose_run(
            args,
            "exec",
            "--no-TTY",
            "api",
            "/evaluate",
            "-images",
            "/evaluation-data/curated/runtime/images.csv",
            "-articles",
            "/evaluation-data/curated/runtime/articles.csv",
        )
    except (OSError, subprocess.CalledProcessError, RuntimeError, KeyError, ValueError, json.JSONDecodeError) as error:
        print(f"Evaluation bootstrap failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
