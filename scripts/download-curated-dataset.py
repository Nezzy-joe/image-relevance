#!/usr/bin/env python3
"""Download missing curated Wikimedia JPEGs and verify their recorded SHA-256."""

from __future__ import annotations

import argparse
import csv
import hashlib
import os
import pathlib
import re
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


ROOT = pathlib.Path(__file__).resolve().parents[1]
DATA = ROOT / "evaluation-data" / "curated"
ASSETS = DATA / "assets"
SOURCES = DATA / "sources.csv"
REQUIRED_FIELDS = {"local_file", "source_url", "license", "license_url", "author", "credit", "sha256"}
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
MAX_FILE_BYTES = 30 * 1024 * 1024
USER_AGENT = "image-relevance-curated-dataset-fetch/1.0"


def digest(path: pathlib.Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            hasher.update(block)
    return hasher.hexdigest()


def source_rows() -> list[dict[str, str]]:
    with SOURCES.open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        missing = REQUIRED_FIELDS - set(reader.fieldnames or [])
        if missing:
            raise ValueError(f"sources.csv is missing required fields: {', '.join(sorted(missing))}")
        rows = list(reader)
    if not rows:
        raise ValueError("sources.csv contains no dataset rows")

    seen: set[str] = set()
    for row in rows:
        name = row["local_file"]
        if not name or pathlib.PurePath(name).name != name or name in seen:
            raise ValueError("sources.csv contains an invalid or duplicate local_file")
        seen.add(name)
        if not SHA256_RE.fullmatch(row["sha256"]):
            raise ValueError(f"sources.csv contains an invalid SHA-256 for {name}")
        for field in REQUIRED_FIELDS - {"sha256"}:
            if not row[field].strip():
                raise ValueError(f"sources.csv is missing {field} for {name}")
    return rows


def download_url(source_url: str, local_file: str) -> str:
    parsed = urllib.parse.urlsplit(source_url)
    if parsed.scheme != "https" or parsed.hostname != "commons.wikimedia.org":
        raise ValueError(f"source page is not an HTTPS Wikimedia Commons URL for {local_file}")
    prefix = "/wiki/"
    if not parsed.path.startswith(prefix):
        raise ValueError(f"source page does not use a Wikimedia file title for {local_file}")
    title = urllib.parse.unquote(parsed.path[len(prefix):])
    if not title.startswith("File:") or not title[5:]:
        raise ValueError(f"source page does not contain a File title for {local_file}")
    filename = urllib.parse.quote(title[5:], safe="")
    return f"https://commons.wikimedia.org/wiki/Special:Redirect/file/{filename}"


def fetch(url: str, destination: pathlib.Path, expected_hash: str) -> None:
    error: Exception | None = None
    for attempt in range(3):
        temporary: pathlib.Path | None = None
        try:
            request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
            with urllib.request.urlopen(request, timeout=45) as response:
                with tempfile.NamedTemporaryFile(dir=ASSETS, prefix=".dataset-download-", delete=False) as out:
                    temporary = pathlib.Path(out.name)
                    total = 0
                    first = b""
                    for block in iter(lambda: response.read(64 * 1024), b""):
                        total += len(block)
                        if total > MAX_FILE_BYTES:
                            raise ValueError("download exceeded the 30 MiB safety limit")
                        if len(first) < 2:
                            first += block[: 2 - len(first)]
                        out.write(block)
            if not temporary or first != b"\xff\xd8":
                raise ValueError("download is not a JPEG file")
            if digest(temporary) != expected_hash:
                raise ValueError("download SHA-256 does not match sources.csv")
            if destination.exists():
                raise FileExistsError("destination appeared during download; refusing to overwrite it")
            os.replace(temporary, destination)
            return
        except urllib.error.HTTPError as exc:
            error = RuntimeError(f"HTTP {exc.code}")
        except urllib.error.URLError as exc:
            error = RuntimeError(f"network error ({type(exc.reason).__name__})")
        except Exception as exc:
            error = exc
        finally:
            if temporary and temporary.exists():
                temporary.unlink()
        if attempt < 2 and isinstance(error, RuntimeError):
            time.sleep(2**attempt)
        else:
            break
    raise RuntimeError(f"could not fetch {destination.name}: {error}") from None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify-only", action="store_true", help="verify all local files without network access")
    args = parser.parse_args()

    rows = source_rows()
    ASSETS.mkdir(parents=True, exist_ok=True)
    downloaded = 0
    for row in rows:
        destination = ASSETS / row["local_file"]
        if destination.exists():
            if digest(destination) != row["sha256"]:
                raise RuntimeError(f"existing file checksum mismatch; refusing to overwrite: {destination.name}")
            continue
        if args.verify_only:
            raise RuntimeError(f"dataset file is missing: {destination.name}")
        fetch(download_url(row["source_url"], row["local_file"]), destination, row["sha256"])
        downloaded += 1

    print(f"Dataset verified: {len(rows)} JPEGs; downloaded {downloaded}; existing files preserved.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, ValueError, RuntimeError) as exc:
        raise SystemExit(f"Dataset download failed: {exc}") from None
