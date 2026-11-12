# image-relevance — verification evidence

> **Latest status:** The release-preparation evidence below supersedes older runtime counts and evaluation summaries in this historical log. Earlier sections record dated checkpoints. UUIDs shown in old checkpoints are redacted and must not be used as dataset identifiers.

## Fresh-database release verification — 2026-11-12

### Reproducible bootstrap

`scripts/bootstrap-evaluation.py` was run against a separate Compose project with its own `image-relevance-repro-final_postgres_data` volume. The original default project volume was not mounted, deleted, or recreated. The initial run against the isolated database seeded the development tenant, processed the curated corpus with the configured Gemini provider, persisted image analysis and embeddings, created the source-derived posts, generated per-database UUID mappings, and ran the evaluation CLI. After a narrow singular/plural normalization test was added for the four dataset species, the bootstrap was rerun against that database; it identified/reused the complete dataset and generated fresh runtime mappings.

The final bootstrap output was:

```text
Seed complete: development tenant already exists; no changes made.
Imported or identified 40 curated images.
Images processed: 40/40
Post reused: fox_overview
Post reused: fox_winter
Post reused: fox_adaptation
Post reused: wolf_pack
Post reused: wolf_kobuk
Post reused: wolf_voyageurs
Post reused: cheetah_ecosystems
Post reused: cheetah_movement
Post reused: eagle_life
Post reused: eagle_protection
Post reused: geology_control
Runtime UUID mappings written under evaluation-data/curated/runtime.
```

The fresh-database CLI output was:

```json
{
  "image_count": 40,
  "category_count": 4,
  "article_count": 11,
  "top1_precision": 1,
  "recommendation_coverage": 0.9090909090909091,
  "accepted_recommendation_precision": 1,
  "incorrect_accepted_matches": 0,
  "abstentions": 1
}
```

`top1_precision=1` represents 10/10 correct top-1 results among the ten relevant posts. Coverage is 10/11 because the geology control abstained. Runtime manifests are generated per database under an ignored directory; stable sources are filenames and source keys rather than database UUIDs.
Each image and article-to-image label received one manual visual review; the labels have not been independently adjudicated and the scores are demonstration evidence only.

### Dataset packaging

The 40 local curated JPEGs total `7,656,543` bytes (`7.3 MiB`; `du -sh` reported `7.4M`). They remain in the workspace and are excluded from the release index. Each `sources.csv` row now records a SHA-256 digest, source page, author/credit, license, and license URL. The missing-file downloader uses each Wikimedia Commons source page to form a full-file redirect URL; Wikimedia documents this redirect pattern for retrieving original media ([Commons technical reuse documentation](https://commons.wikimedia.org/wiki/Commons:Reusing_content_outside_Wikimedia/technical)).

Local validation returned:

```text
Dataset verified: 40 JPEGs; downloaded 0; existing files preserved.
source_rows=40 resolvable_commons_redirect_urls=40 all_https=True
```

The download path was not exercised because all source files were already present; the recorded run verifies local checksums and URL construction only.

### Repeated API and database demonstrations

Independent `curl` requests to the isolated API returned:

```text
GET /health HTTP 200
{"status":"ok"}
GET /ready HTTP 200
{"checks":{"database":"ok"},"status":"ready"}
```

The image detail API returned HTTP 200 for a processed real fox JPEG with persisted metadata: subject `Red Fox`, category `Mammal`, six attributes, confidence `0.98`. `POST /v1/posts/{postID}/matches` returned HTTP 200 and an accepted `Red fox` candidate; it also rejected the cross-species `Mexican gray wolf` candidate. Both API review calls returned HTTP 201: the accepted fox suggestion was approved with a reviewer comment, and the AI-rejected wolf candidate was rejected with a reviewer comment. The unrelated Yellowstone geology article returned HTTP 200 with `status=no_confident_match` and zero accepted suggestions.

Read-only aggregates from the isolated PostgreSQL database:

```text
pgvector extension version: 0.8.7
images: 40 total, 40 processed
image_embeddings: 40 total, 40 at vector(768)
posts: 11 total, 11 at vector(768)
review audit: approved | original AI accepted=true | comment persisted
review audit: rejected | original AI accepted=false | comment persisted
```

The review rows demonstrate that reviewer actions append without changing the saved AI `match_suggestions.accepted` decisions.

### Provider usage records

The isolated database contained these provider-usage groups:

| Operation | Model | Successful rows | Failed rows | Available usage metadata |
| --- | --- | ---: | ---: | --- |
| Image classification | `gemini-3.1-flash-lite` | 40 | 6 | Successful rows had input/output token counts, latency and a list-price estimate; failed rows had latency but no token counts or estimate. |
| Image embedding | `gemini-embedding-2` | 40 | 0 | Latency and per-image list-price estimate; the provider returned no token counts. |
| Article subject resolution | `gemini-3.1-flash-lite` | 11 | 3 | Successful rows had input/output token counts, latency and a list-price estimate; failed rows had latency but no token counts or estimate. |
| Article text embedding | `gemini-embedding-2` | 11 | 0 | Latency recorded; no provider token counts, so no cost estimate was recorded. |

These costs are estimates, not billed amounts. Usage rows are attributable by tenant, provider, model, operation and image/post where available. No credentials or secret values are included in this evidence.

### Docker, security, and legal status

The registry-backed standard startup was retried in the isolated Compose project:

```text
docker compose --project-name image-relevance-repro-final --file docker-compose.yml --file /tmp/image-relevance-repro.override.yml --project-directory "$PWD" --profile ai up --build -d
ERROR: failed to resolve source metadata for docker.io/library/golang:1.26.0-alpine3.23
dial tcp [2600:1f18:2148:bc01:5963:e996:adbf:5fec]:443: connect: network is unreachable
```

The clean registry-backed build remains unverified in this environment. Docker Hub access is required to pull the Go build image; Gemini processing also needs Google API network access. This attempt did not target the original Compose project or its `postgres_data` volume. A locally available scratch image was used for the isolated API/worker/database demonstrations; that is not evidence of a clean registry-backed build.

The local `.env` remained ignored and mode `0600`. Credential checks were boolean-only; no API key or private key material was found outside the ignored `.env`. `.gitignore` excludes env files except `.env.example`, private-key/certificate files, credential/secret directories, local database files, runtime evaluation UUID manifests and build outputs. The curated image set retains Wikimedia source, creator/credit, license and license URL attribution.

Sanitized scan summary:

```text
env_file_present=true gemini_key_present=true env_mode=0o600
secret_scan_files=61 api_key_outside_env=false private_key_or_service_credential_files=0
gitignore_env_template_exception=true runtime_uuid_directory_ignored=true
curated_images=40 four_categories=4 image_files_present=true source_license_metadata_complete=true
article_source_rows=11 article_json_records=11 stable_data_contains_uuid=false
```

The original Compose database container was already stopped when this verification resumed. Its existing `aiimageunderstandingcontentmatchingengine_postgres_data` volume remained present and was not mounted by the isolated test project. The isolated database had a separate `image-relevance-repro-final_postgres_data` volume; its API and worker remained running for review.

Business routes currently use one seeded development tenant and do not authenticate callers. This is not production multi-tenant authentication. `LICENSE` contains the MIT text and names `Nezzy-joe` as the copyright holder; legal ownership confirmation remains an owner-side publication check.

### Release checks

`go test ./...`, `go test -race ./...`, `go vet ./...`, and `docker compose config --quiet` passed in the verification run for the plural-normalization change. The isolated stack had a healthy PostgreSQL service and running API/worker processes during live checks. That acceptance verification did not create a remote or push; local Git release preparation is recorded separately.

All sections below are dated historical checkpoints. Their statements describe only the state at that checkpoint and are superseded by the fresh-database release verification at the top of this file.

The Phase 1 snapshot was captured on 2026-11-11. Later-phase behavior was outside that snapshot's scope and is documented in the newer release evidence above.

## Toolchain

```text
go version go1.26.0 linux/amd64
Docker version 29.8.0, build 88096ef
Docker Compose version v5.5.1
```

## Go checks

`gofmt -w cmd/seed/main.go internal/config/config.go` completed successfully.

`GOCACHE=/tmp/image-relevance-gocache go mod tidy` exited 0.

`GOCACHE=/tmp/image-relevance-gocache go test ./...`:

```text
?   github.com/Nezzy-joe/image-relevance/cmd/api [no test files]
?   github.com/Nezzy-joe/image-relevance/cmd/seed [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/config (cached)
?   github.com/Nezzy-joe/image-relevance/internal/database [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/httpapi (cached)
ok  github.com/Nezzy-joe/image-relevance/internal/middleware (cached)
```

`GOCACHE=/tmp/image-relevance-gocache go test -race ./...`:

```text
?   github.com/Nezzy-joe/image-relevance/cmd/api [no test files]
?   github.com/Nezzy-joe/image-relevance/cmd/seed [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/config (cached)
?   github.com/Nezzy-joe/image-relevance/internal/database [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/httpapi (cached)
ok  github.com/Nezzy-joe/image-relevance/internal/middleware (cached)
```

`GOCACHE=/tmp/image-relevance-gocache go vet ./...` exited 0 with no diagnostics.

`docker compose config >/dev/null` exited 0. Its rendered output was discarded to avoid printing development credentials.

## Compose and database

`python3 -c 'import yaml; m=yaml.safe_load(open("capstone.yaml")); required={"run","seed","test","base_url","endpoints_to_probe"}; missing=required-set(m); assert not missing, missing; assert len(m["endpoints_to_probe"]) == 2; print("capstone.yaml: valid YAML; required keys and two endpoint probes present")'`:

```text
capstone.yaml: valid YAML; required keys and two endpoint probes present
```

`docker compose up --build -d` exited 0. It built `image-relevance-api:local`, left the PostgreSQL container running, and started the rebuilt API. No database or volume removal/recreation command was run.

The PostgreSQL container ID was unchanged before and after the image build, and its existing project-scoped `postgres_data` volume remained attached. Local Docker identifiers are omitted from this public-facing evidence.

The documented seed command, run twice with `docker compose exec -T api /seed`, returned:

```text
Starting image-relevance development tenant seed.
Seed complete: created the development tenant.

Starting image-relevance development tenant seed.
Seed complete: development tenant already exists; no changes made.
```

The first run changed only one row in the existing `tenants` table. No image or post tables exist in the Phase 1 schema; no image, post, or embedding records were created.

A non-mutating production-mode check returned the expected refusal before connecting to PostgreSQL:

```text
Starting image-relevance development tenant seed.
Seed failed: refusing to seed a development tenant unless APP_ENV=development
```

Host port check:

```text
$ pg_isready -h 127.0.0.1 -p 15432 -U flyrank -d flyrank
127.0.0.1:15432 - accepting connections

$ docker compose port db 5432
127.0.0.1:15432
```

`docker compose port db 5432` on the final rebuilt stack returned `127.0.0.1:15432`.

Read-only database query:

```sql
SELECT current_database() AS database_name,
       current_setting('server_version') AS postgres_version,
       (SELECT extversion FROM pg_extension WHERE extname = 'vector') AS pgvector_version,
       to_regclass('public.tenants') IS NOT NULL AS tenants_table_exists;
SELECT version, dirty FROM schema_migrations;
```

```text
 database_name |        postgres_version         | pgvector_version | tenants_table_exists
---------------+---------------------------------+------------------+----------------------
 flyrank       | 17.11 (Debian 17.11-1.pgdg12+2) | 0.8.7            | t
(1 row)

 version | dirty
---------+-------
       1 | f
(1 row)
```

Final post-rebuild checks returned:

```text
database=flyrank
tenant_rows=1
seed_tenant=image-relevance development tenant,status=active
pgvector=0.8.7
migration=1,dirty=false
```

The existing `flyrank` database/user and project-scoped volume name are retained for local data compatibility. Renaming them would require a deliberate database/volume migration; none was attempted.

## Historical capstone completion checkpoint — 2026-11-12 (superseded by release evidence above)

This section supersedes the earlier sprint notes that said image labels, real evaluation data, human review, or successful Gemini generation were unavailable. The database volume was kept in place and existing records were not deleted.

### Dataset and persisted records

- `evaluation-data/curated/assets/` contains 40 JPEGs, ten each for red fox, gray wolf, cheetah, and bald eagle. All 40 file entries in `sources.csv` have a source URL, creator/credit, license, and license URL. Ten visually miscategorized/non-photographic entries from the original pool were excluded; the raw pool was left intact. See `ATTRIBUTION.md` for full credits and license links.
- One reviewer visually checked the images and one category-level example image for each source-derived positive article. The ten animal posts are sourced summaries of National Park Service, U.S. Fish & Wildlife Service, and Cheetah Conservation Fund pages listed in `article_sources.csv`. An eleventh Yellowstone geology post is the no-suitable control. Labels have not been independently human-adjudicated.
- The API accepted 40 uploads (HTTP 202); all 40 curated image IDs reached `processed` with `uncertain=false`. Read-only PostgreSQL checks returned `curated_processed=40`, `successful_classified_ids=40`, `successful_image_embedding_ids=40`, and `failed_job_on_curated=0`. Each image has its own metadata row and direct-image `vector(768)` embedding.
- Eleven evaluation posts were persisted with text embeddings. Each post has successful, linked subject-resolution and text-embedding usage rows. A separate malformed structured subject response was recorded as failed and did not create a post.

### Evaluation CLI

Command:

```sh
docker compose exec -T api /evaluate \
  -images /evaluation-data/curated/images.csv \
  -articles /evaluation-data/curated/articles.csv
```

Actual JSON output:

```json
{
  "image_count": 40,
  "category_count": 4,
  "article_count": 11,
  "top1_precision": 1,
  "recommendation_coverage": 0.9090909090909091,
  "accepted_recommendation_precision": 1,
  "incorrect_accepted_matches": 0,
  "abstentions": 1
}
```

The manifest contains ten relevant articles and one `no_suitable` control. Thus the CLI's top-1 precision corresponds to 10/10 correct relevant top-1 results; coverage is 10/11. A database aggregate over the first evaluation suggestion batch counted 220 returned candidate records, 73 AI-accepted and 147 rejected; all 73 accepted candidates were expected-label images (73/73 precision), with zero incorrect accepted matches. The unrelated control abstained. These are the observed results for this curated, single-review dataset, not independently adjudicated quality claims.

### Human-review workflow

`POST /v1/posts/[runtime UUID omitted]/matches` returned HTTP 200 and exposed suggestion IDs, similarity, and AI decisions. The selected red fox suggestion (`[runtime UUID omitted]`, similarity `0.4340205648`, AI accepted) was reviewed through `POST /v1/matches/[runtime UUID omitted]/review`; it returned HTTP 201, decision `approved`, and a reviewer note. The gray wolf candidate (`[runtime UUID omitted]`) was returned AI-rejected for a subject mismatch and reviewed through its review endpoint; HTTP 201 recorded decision `rejected` and a reviewer note.

The follow-up database query showed one `review_actions` row for each suggestion. The red fox row still has `match_suggestions.accepted=true`; the wolf row still has `accepted=false`. Review actions append to history and do not overwrite AI decisions. The current schema does not store an authenticated reviewer identity.

### Gemini usage and cost estimates

Every observed request created an `ai_usage` row with provider, model, operation, outcome, and latency. Successful image requests are linked to image IDs; successful post requests are linked to post IDs. One failed structured subject call has no `post_id` because no post was created, but its tenant, operation, model, outcome, token metadata, latency, and estimate are recorded.

| Operation | Recorded calls | Usage and latency | Recorded cost |
|---|---:|---|---:|
| `gemini-3.1-flash-lite` image classification | 40 successful; 3 provider failures | Success: 45,310 input and 8,373 output tokens total; mean 6,780 ms. Failures had no usage tokens; mean 25,008 ms. | Successful calls: estimated `$0.02388700`; failed calls: no estimate |
| `gemini-embedding-2` image embedding | 40 successful | Token counts were not returned; mean 2,681 ms | Estimated `$0.00480000` using `$0.00012` per image |
| `gemini-3.1-flash-lite` subject resolution | 11 successful; 1 invalid structured response | Success: 993 input and 1,416 output tokens total; mean 4,081 ms. Failed response: 88 input and 497 output tokens; 4,227 ms. | Success: estimated `$0.00237225`; failed response: estimated `$0.00076750` |
| `gemini-embedding-2` text embedding | 11 successful | SDK returned no input token counts; mean 569 ms | No estimate recorded |

Recorded estimates total `$0.03182675` for calls with usable usage metadata or a verified per-image rate. This is not a billed amount; it excludes free tier, credits, and discounts. The text embeddings and three failed generation calls have unknown cost because the provider returned no token usage. Rates were checked against [Google's Gemini API pricing](https://ai.google.dev/gemini-api/docs/pricing) on 2026-11-12.

### Tenant and release security review

- Image, post, match, and review services resolve one fixed `image-relevance development tenant`. Matching SQL filters the post, embedding, and image by the same tenant; review insertion filters the suggestion by the resolved tenant. Image retrieval and upload deduplication also use that tenant.
- The worker carries `tenant_id` from service-created jobs into embedding and usage writes. The claim path relies on globally unique image IDs and service-created job consistency; this should be tightened with composite tenant/image constraints before production multi-tenant use.
- No route authenticates a caller or accepts an authenticated tenant context. There is one seeded tenant, so production multi-tenant isolation is not implemented or tested.
- `.env` remained mode `0600`. Boolean-only checks returned `API_KEY_PRESENT=true` and `WORKER_KEY_PRESENT=true`; no credential value was displayed. A scan for common API-key/private-key patterns outside local `.env` found zero; no private-key files were present. This pattern scan is not a substitute for a full secret scanner.

### Final checks and runtime

`GOCACHE=/tmp/image-relevance-gocache go test ./...`, `GOCACHE=/tmp/image-relevance-gocache go test -race ./...`, and `GOCACHE=/tmp/image-relevance-gocache go vet ./...` all exited 0. The default Go cache path was read-only in this environment, so the writable `/tmp` cache was used. `docker compose config --quiet` exited 0.

The normal `docker compose --profile ai up --build -d` attempt could not resolve `registry-1.docker.io` (`DNS server misbehaving`). To verify the current source despite that environment failure, the Go commands were built locally with `CGO_ENABLED=0` into a temporary scratch-image context; `docker compose --profile ai up -d --no-build` then recreated only the API and worker. Compose reported PostgreSQL `Running`, and its container ID remained `d5185035413e1307c33e11a4ebf38810798b6360381bf778b71d9a23ed71cba5`; its attached volume remained `aiimageunderstandingcontentmatchingengine_postgres_data`.

Final services: API running, worker running, PostgreSQL healthy. The database reported `flyrank`, pgvector `0.8.7`, migration `2:false`, one tenant, 43 total images, 13 total posts, and two review-action rows. The host database port remains `127.0.0.1:15432`.

Actual endpoint checks:

```text
GET /health: HTTP 200 — {"status":"ok"}
GET /ready: HTTP 200 — {"checks":{"database":"ok"},"status":"ready"}
```

The registry-backed clean Compose build remains to be repeated when Docker Hub DNS is available. No Git repository, remote, commit, or push was created.

## Live HTTP checks

`curl -sS --max-time 8 -w '\nHTTP_STATUS=%{http_code}\n' http://127.0.0.1:8080/health`:

```text
{"status":"ok"}
HTTP_STATUS=200
```

`curl -sS --max-time 8 -w '\nHTTP_STATUS=%{http_code}\n' http://127.0.0.1:8080/ready`:

```text
{"checks":{"database":"ok"},"status":"ready"}
HTTP_STATUS=200
```

## Not yet evidenced in the Phase 1 snapshot

Image ingestion, background jobs, AI provider calls/usage, embeddings, matching, review, and evaluation belong to later phases and have no Phase 1 runtime evidence.

## Capstone sprint verification — 2026-11-11

### Source checks

`gofmt -w $(find cmd internal -name '*.go' -type f)` completed.

`GOCACHE=/tmp/image-relevance-gocache GOTOOLCHAIN=local go mod tidy` exited 0. The Google GenAI SDK is a direct module dependency.

`GOCACHE=/tmp/image-relevance-gocache GOTOOLCHAIN=local go test ./...` exited 0:

```text
?   github.com/Nezzy-joe/image-relevance/cmd/api             [no test files]
?   github.com/Nezzy-joe/image-relevance/cmd/evaluate        [no test files]
?   github.com/Nezzy-joe/image-relevance/cmd/seed             [no test files]
?   github.com/Nezzy-joe/image-relevance/cmd/worker           [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/config
?   github.com/Nezzy-joe/image-relevance/internal/database    [no test files]
ok  github.com/Nezzy-joe/image-relevance/internal/domain
ok  github.com/Nezzy-joe/image-relevance/internal/evaluation
ok  github.com/Nezzy-joe/image-relevance/internal/httpapi
ok  github.com/Nezzy-joe/image-relevance/internal/ingestion
ok  github.com/Nezzy-joe/image-relevance/internal/matching
ok  github.com/Nezzy-joe/image-relevance/internal/middleware
ok  github.com/Nezzy-joe/image-relevance/internal/providers/gemini
ok  github.com/Nezzy-joe/image-relevance/internal/worker
```

`GOCACHE=/tmp/image-relevance-gocache GOTOOLCHAIN=local go test -race ./...` exited 0 for all test packages. `GOCACHE=/tmp/image-relevance-gocache GOTOOLCHAIN=local go vet ./...` exited 0 with no diagnostics. `docker compose config --quiet` exited 0. PyYAML parsed `capstone.yaml` and `schemas/openapi.yaml`; the capstone manifest contains the required five keys.

### Preserved database and Compose runtime

`docker compose --profile ai up -d --build api worker` exited 0 and started the API and Gemini worker. The exact capstone manifest command, `docker compose --profile ai up --build -d`, also exited 0. Compose reused the original PostgreSQL container (same container ID before and after) and the same `aiimageunderstandingcontentmatchingengine_postgres_data` volume; neither was removed or recreated. The API, worker, and PostgreSQL all reported running; PostgreSQL was healthy on host port `15432`.

The `.env` file is mode `0600`. A value-free Compose inspection confirmed a non-empty `GEMINI_API_KEY` environment entry in both running API and worker containers. No key value was printed.

`curl -sS --max-time 8 -w '\\nHTTP_STATUS=%{http_code}\\n' http://127.0.0.1:8080/health`:

```text
{"status":"ok"}
HTTP_STATUS=200
```

`curl -sS --max-time 8 -w '\\nHTTP_STATUS=%{http_code}\\n' http://127.0.0.1:8080/ready`:

```text
{"checks":{"database":"ok"},"status":"ready"}
HTTP_STATUS=200
```

Read-only database verification:

```text
database=flyrank pgvector=0.8.7 migration=2:false tenant_count=1 images=true posts=true jobs=true
vector_dimensions=vector(768)
```

The documented `docker compose exec -T api /seed` returned `development tenant already exists; no changes made.`

Development route smoke checks used invalid or nonexistent inputs and did not call Gemini or create rows:

```text
POST /v1/images without file: HTTP 400, code=invalid_upload
POST /v1/posts with {}: HTTP 400, code=invalid_post
POST /v1/posts/{zero UUID}/matches: HTTP 404, code=not_found
```

### Real Gemini checks

- Authenticated model metadata for `gemini-3.8-flash` returned HTTP `200` and listed `generateContent` as supported.
- The opt-in Google GenAI Go SDK embedding check passed: `model=gemini-embedding-2 dimensions=768`.
- The opt-in SDK generation check returned HTTP `503`, status `UNAVAILABLE` for `gemini-3.8-flash`. A separate low-latency Flash-Lite attempt also returned `503`. The API key is accepted by the metadata and embedding endpoints, but vision/text generation could not be verified due to provider availability. The image classification worker therefore has not produced real metadata in this runtime.

No fake image-analysis or matching rows were inserted. The workspace contains no licensed benchmark corpus, so the evaluation CLI was unit-tested but has not emitted project metrics. It requires at least 40 licensed images across four categories and 10 labeled articles before calculating actual scores.

### Current acceptance status

The code and deterministic tests cover image validation/metadata parsing, worker retry policy, exact fox-versus-wolf/dog subject rejection, uncertainty/similarity rejection, append-only review persistence logic, and evaluation metric calculations. End-to-end batch classification, persisted live image embeddings, full fox/wolf/no-suitable API scenarios, AI usage rows from successful generation, and benchmark metrics remain unverified until Gemini generation is available and a licensed dataset is supplied.

## Continuation verification — 2026-11-12

The checks below were run after the previous sprint session. The stopped PostgreSQL container was started in place; its container identity and existing `postgres_data` volume were preserved. No image, post, job, or match data was added.

`docker compose config --quiet` exited 0. A value-free inspection confirmed that the stopped API and worker container configurations each contain a non-empty `GEMINI_API_KEY` entry; no value was printed.

Database checks:

```text
pg_isready: /var/run/postgresql:5432 - accepting connections
database=flyrank pgvector=0.8.7 migration=2
tables=tenants,images,posts
tenants=1 images=0 posts=0 jobs=0 match_suggestions=0
```

For endpoint verification, a temporary API-only container was run with `GEMINI_API_KEY` explicitly blank, while the worker remained stopped. The existing database was used for readiness:

```text
GET /health: {"status":"ok"} — HTTP 200
GET /ready: {"checks":{"database":"ok"},"status":"ready"} — HTTP 200
```

`env GOCACHE=/tmp/image-relevance-gocache go test ./...`, `env GOCACHE=/tmp/image-relevance-gocache go test -race ./...`, and `env GOCACHE=/tmp/image-relevance-gocache go vet ./...` all exited 0. The default Go build cache was read-only in this environment, so these checks used the writable temporary cache.

The live Gemini Embedding 2 check passed in the prior session with 768 dimensions. The latest generation attempt returned HTTP 504 (`DEADLINE_EXCEEDED`); a separate Interactions API connectivity attempt returned HTTP 503 without model output. No new provider-backed image or article data was created during that checkpoint. At that time `evaluation-data/` contained zero files, and no benchmark metrics were available.

## Gemini integration resume — 2026-11-12

### Services and credentials

The ignored `.env` file remained mode `0600`; a presence-only check returned `GEMINI_API_KEY_present=true`. Value-free container inspection returned `GEMINI_API_KEY_present=true` for the recreated API and worker configurations. PostgreSQL stayed on its original container ID and the existing `aiimageunderstandingcontentmatchingengine_postgres_data` volume; it remained healthy and accepted connections. No Compose command rendered the resolved environment.

The first application image rebuild failed because Docker could not reach `gcr.io` over the available network (`network is unreachable` for its IPv6 endpoint). API and worker containers were recreated from the existing local image without recreating dependencies. The updated source-built worker was then run as a one-off Compose container with the Compose environment and image-data volume; the standard worker container is stopped. `docker compose config --quiet` passed.

`GET /health` returned `{"status":"ok"}` with HTTP 200. `GET /ready` returned `{"checks":{"database":"ok"},"status":"ready"}` with HTTP 200.

The first JPEG upload returned HTTP 500 because the empty image-data volume was root-owned while the API runs as UID 65532. The separate image-data directory was changed to owner `65532:65532` (it held zero files); the PostgreSQL volume was not touched. The upload then returned HTTP 202 with an image and job ID. The Dockerfile now creates the image storage directory with UID/GID 65532 for fresh volumes; the rebuilt image remains pending registry network access.

### Gemini generation and image classification

The replacement key was used only from process memory; no value was printed or recorded.

| Model / operation | Attempt | HTTP | Latency | Result | Safe error |
| --- | ---: | ---: | ---: | --- | --- |
| `gemini-3.8-flash` text generation | 1 | 503 | 2,056 ms | Failed | `UNAVAILABLE` |
| `gemini-3.8-flash` text generation | 2 | 503 | 1,685 ms | Failed | `UNAVAILABLE` |
| `gemini-3.8-flash` text generation | 3 | 503 | 2,480 ms | Failed | `UNAVAILABLE` |
| `gemini-3.7-flash` fallback | 1 | 503 | 2,765 ms | Failed | `UNAVAILABLE` |
| `gemini-3.7-flash` fallback | 2 | 503 | 2,004 ms | Failed | `UNAVAILABLE` |
| `gemini-3.7-flash` fallback | 3 | no response | 20,428 ms | Failed | `TIMEOUTERROR` |

The real JPEG classification request timed out after 25,050 ms without an HTTP response. The worker's five bounded attempts all failed with its safe code `provider_request_failed`. The persisted image is `failed`; the job is `failed` after five attempts. PostgreSQL contains five failure usage records for this image, zero `image_analysis` rows, and zero `image_embeddings` rows. No metadata was fabricated or persisted.

### Embeddings and mismatch guard

The opt-in Google GenAI integration tests succeeded for one text embedding and the real JPEG image embedding using `gemini-embedding-2`; each returned 768 dimensions. Source now exposes `EmbedImage`, and the worker calls it with the uploaded bytes after successful image classification. The live classifier failure stopped the worker before that persistence stage. The post-creation path continues to generate and persist text embeddings after subject resolution; generation unavailability prevented a post row from being created.

A live, read-only PostgreSQL pgvector ranking test used the real fox image embedding plus text embeddings and the existing subject guard:

```text
red_fox cosine=0.4601 decision=accepted
gray_wolf cosine=0.3417 decision=rejected
bicycle_maintenance cosine=0.1920 decision=rejected
```

The red fox candidate ranked first. Gray wolf was rejected by the subject mismatch guard, and the unrelated article was rejected. This smoke check used transient vectors and did not write posts, image metadata, match suggestions, or benchmark metrics. Deterministic guard tests also pass for red fox acceptance, gray wolf rejection, and unrelated-article abstention.

### Licensed image source set and evaluation status

The local ignored directory `evaluation-data/assets/` now contains 40 real JPEGs from Wikimedia Commons, with ten query-grouped assets each for red fox, gray wolf, cheetah, and bald eagle. `sources.csv` records each unique file page, author/credit when provided, and its individual license metadata; `ATTRIBUTION.md` describes the set. The per-file license was obtained from Commons' machine-readable image metadata. The assets are source material, not confirmed labels: they have not all been visually reviewed, ingested, or classified.

The evaluation CLI still requires at least 40 processed, non-uncertain database images across four categories plus at least 10 real post rows labeled across positive, mismatch-guard, and no-suitable scenarios. That condition is not met. No evaluation metric is reported.

## Historical integration checkpoint before curated-label verification — 2026-11-12

This section supersedes the pending integration status in the earlier checkpoints above. The local key remained in the ignored `.env` file with mode `0600`; all credential checks were boolean-only. No rendered Compose environment was displayed. The original PostgreSQL service and persistent volume were retained.

### Model verification

Authenticated model metadata for `gemini-3.1-flash-lite` returned HTTP 200 and listed `generateContent` as supported. Google's current model documentation lists text, image input, and structured outputs for this stable model: [Gemini 3.1 Flash-Lite model page](https://ai.google.dev/gemini-api/docs/models/gemini-3.1-flash-lite) and [structured outputs guide](https://ai.google.dev/gemini-api/docs/structured-output).

```text
credential_present=true
model_metadata_http_status=200 latency_ms=921 model_available=true supports_generate_content=true
minimal_text attempt=1 http_status=200 latency_ms=18032 success=true safe_error=none
jpeg_classification attempt=1 http_status=200 latency_ms=23750 success=true safe_error=none
local_schema_validation=true
```

The structured JPEG result identified `red fox`, category `animal`, caption `A red fox standing in the snow.`, confidence `1.0`, and five visible attributes. No API key was printed or stored in this evidence.

### Docker, database, and API

`docker compose --profile ai up --build -d` passed using a static `scratch` runtime with CA certificates copied from the Go build stage. This avoids the unavailable `gcr.io` pull. The API and standard worker were recreated; the existing PostgreSQL container stayed running and healthy, and the project volume remained `aiimageunderstandingcontentmatchingengine_postgres_data`. The API and worker configurations each had a non-empty key (boolean check only) and `GEMINI_VISION_MODEL=gemini-3.1-flash-lite`. A stale one-off worker was stopped so only the standard worker claims jobs.

```text
database=flyrank
postgres=17.11
pgvector=0.8.7
migration=2,dirty=false
health: {"status":"ok"} — HTTP 200
ready: {"checks":{"database":"ok"},"status":"ready"} — HTTP 200
```

### Persisted image and article data

The standard worker processed two real, visually checked Commons images. Both image rows, analysis records, and direct-image vectors belong to the same seeded development tenant; each vector is `vector(768)` from `gemini-embedding-2`.

| Asset | Image ID | Source license | Persisted subject | Vision model | Image vector |
| --- | --- | --- | --- | --- | --- |
| `red_fox-03.jpg` | `[runtime UUID omitted]` | CC BY-SA 2.0, Gregory “Slobirdr” Smith | `red fox`, confidence 0.98 | `gemini-3.1-flash-lite` | 768 dimensions |
| `gray_wolf-05.jpg` | `[runtime UUID omitted]` | CC BY 2.0, Raed Mansour | `gray wolf`, confidence 0.95 | `gemini-3.1-flash-lite` | 768 dimensions |

The image vector is computed from original JPEG bytes. The vision caption remains text in `image_analysis.caption`; it is not used as the image vector. The red fox demonstration post and unrelated bicycle-maintenance post were each saved with a 768-dimensional article-text embedding.

One older red fox record remains in the preserved database from the earlier one-off worker run; its recorded generation model is `gemini-3.8-flash`. The two assets in the table above were processed after the standard worker was recreated with `gemini-3.1-flash-lite`. The stale one-off container was stopped and removed; its database record and the PostgreSQL volume were preserved.

### Matching acceptance probes

`POST /v1/posts` returned HTTP 201 for the red fox demo article (`[runtime UUID omitted]`). The resolved subject was `red fox hunting behavior` at confidence 0.90. A phrase-aware subject guard was added so a specific article phrase remains compatible with its underlying `red fox` image subject while `gray wolf` remains rejected.

```text
red fox article match: HTTP 200, status=matched
  red fox image similarity=0.4540 accepted=true
  second red fox image similarity=0.4212 accepted=true
  gray wolf image similarity=0.3522 accepted=false, reason=subject mismatch

unrelated bicycle article: POST /v1/posts HTTP 201, id=[runtime UUID omitted]
unrelated match: HTTP 200, status=no_confident_match
  top candidate cosine similarity=0.2246, accepted=false
```

The API's PostgreSQL records show all three image rows with analysis and 768-dimensional image vectors, both post rows with 768-dimensional text vectors, and six persisted match decisions. SQL filters candidate posts, images, and embeddings by tenant ID. The current routes still resolve only the seeded development tenant; tenant authentication and caller-level tenant selection are not implemented.

### Evaluation dataset review

`sources.csv` lists 40 existing JPEGs, ten per query group, all present on disk, with Commons source pages and license labels. A Wikimedia Commons API check returned all 40 file pages and matched all 40 license labels with zero mismatches. The license labels count 7 CC BY 2.0, 1 CC BY-SA 2.0, 1 CC BY-SA 3.0, 24 CC BY-SA 4.0, 5 CC0, and 2 Public domain. Visual review of all four contact sheets found incorrect category content, including aircraft and museum artifacts in the cheetah group, non-bald eagle species in the bald eagle group, and other canids/artwork among some fox and wolf results. These are source-search results, not confirmed labels. No ten article-to-image mappings were fabricated, and the evaluation CLI was not run against this unverified set. No benchmark metrics are claimed.

### Code verification

After the last code changes:

```text
gofmt: completed for modified Go files
go test ./...: passed
go vet ./...: passed
docker compose config --quiet: passed
```

The race suite and final database-volume identity check are recorded after the final pass below.

Final verification after the last source changes:

```text
go test ./...: passed
go test -race ./...: passed
go vet ./...: passed
docker compose config --quiet: passed
docker compose --profile ai up --build -d: passed; API, worker, and DB running
PostgreSQL health: healthy; original container identity unchanged
PostgreSQL data volume: aiimageunderstandingcontentmatchingengine_postgres_data
GEMINI_API_KEY presence: true in API and worker configurations (value not displayed)
GEMINI_VISION_MODEL: gemini-3.1-flash-lite in API and worker configurations
.env mode: 0600
GET /health: HTTP 200
GET /ready: HTTP 200
```

Final read-only PostgreSQL counts were `image_embeddings=3` rows with 768 dimensions, `posts=2` rows with 768 dimensions, and `match_suggestions=6`. The migration state remained `2:false`, pgvector remained `0.8.7`, and all persisted image vectors matched the development tenant ID associated with their image rows.
