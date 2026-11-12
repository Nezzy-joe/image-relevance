# Build log

## Phase 0 and Phase 1 — 2026-11-11

### AI-assisted work

- Used AI assistance to inspect the empty workspace, draft the Phase 0 design artifacts, scaffold the Go module, and implement the Phase 1 API, database wiring, middleware, Compose setup, tests, and README.
- No external AI provider was integrated or called by the application in this phase.

### Suggestions corrected during implementation

- The first HTTP test harness used `httptest.NewServer`, which requires binding a local TCP socket. The environment blocks that operation in sandboxed tests, so the tests now invoke the router with `httptest.NewRequest` and `httptest.NewRecorder` while still exercising the HTTP handler/middleware chain.
- The initial host database mapping used port 5432, which was already occupied. A retry on 5433 was also occupied. Compose now defaults to localhost port 15432 while the API container continues to connect to PostgreSQL on the internal service port 5432.
- The first Docker builder used the Debian Go image. Pulling its large layers was slow in this environment, so the builder uses the smaller Go 1.26.0 Alpine 3.23 image. The final runtime remains a non-root distroless static image, and the Go binary is built with CGO disabled.

### Engineering decisions

- The initial scaffold recorded `flyrank-capstone-image-relevance`; the user confirmed `github.com/Nezzy-joe/image-relevance` as the intended public repository. The Go module and project metadata now use that confirmed path. The provided workspace is not an initialized Git repository, so this work did not create or alter Git history.
- Embedded SQL migrations in the API binary so the container does not need a separate migration mount or command. The API applies migrations before accepting requests.
- Made `/health` independent of PostgreSQL and made `/ready` check the database with a deadline. Public error bodies contain stable codes and do not expose database or panic details.
- Added request IDs, structured `slog` access records, request timeouts, and graceful signal shutdown to the process wiring.
- Kept the schema migration small for Phase 1: it enables pgvector and creates the tenant table. Feature tables will arrive with the phases that own their workflows.

## Pre-publication Phase 1 closeout — 2026-11-11

- Added `capstone.yaml` with the working Compose run command, the real seed command, the Go test command, the local API base URL, and the implemented health/readiness probes.
- Added `cmd/seed`, a development-only, idempotent tenant seed using the existing `tenants` schema. It uses a transaction-scoped advisory lock and creates no image, post, or embedding data. The Docker image includes the one-shot `/seed` executable.
- Changed visible project/API titles to `image-relevance`; retained FlyRank as the capstone origin. The old scaffold name above remains historical context.
- Inspected the live local database before editing. Kept its `flyrank` database/user defaults and existing Compose volume to preserve data; no database, container volume, or persistent data was deleted or recreated.
- No Phase 2 functionality or additional Go dependencies were introduced.

## Capstone implementation sprint — 2026-11-11

- Added an additive core migration for image records and analysis, 768-dimensional image/post vectors, background jobs, provider usage, match suggestions, and append-only review actions. Applied migration 2 to the existing database; verified it is not dirty and kept the original PostgreSQL container and volume.
- Added development-only JPEG/PNG upload validation, SHA-256 deduplication and shared-volume storage; a separate Go worker claims PostgreSQL jobs with `FOR UPDATE SKIP LOCKED`, records provider usage, persists validated analysis/vectors, and retries failures with capped exponential backoff.
- Added a Google GenAI SDK adapter for `gemini-3.8-flash` structured classification/subject resolution and `gemini-embedding-2` 768-dimension text embeddings. Official Google model docs list these model IDs and support structured outputs/multimodal input; the live embedding request succeeded. Live generation returned HTTP 503 `UNAVAILABLE`, so no real classification result is claimed.
- Added cosine candidate retrieval, an exact normalized subject mismatch guard, uncertainty and confidence/similarity rejections, no-confident-match abstention, and an append-only review route.
- Added an evaluation CLI and manifest validation. No licensed 40-image/10-article corpus exists in the workspace, so no benchmark metrics are reported.
- Added verified list-price estimates when provider token usage is present for configured models; other model/rate cases retain a `NULL` cost rather than inventing a value.
- Updated README, architecture, design, OpenAPI, ERD, capstone manifest, build log, evidence, and evaluation instructions to distinguish implemented source from live-verified behavior and remaining blockers.
- No Git repository, remote, commit, or push was created.

## Final capstone completion pass — 2026-11-12

- Curated a public-facing set of 40 licensed JPEGs across red fox, gray wolf, cheetah, and bald eagle. Retained existing Commons source metadata, excluded ten visually incorrect/non-photographic pool results, added ten replacement photos, and documented per-file attribution. The labels received one visual review and still need independent human adjudication.
- Uploaded the curated assets through the API, waited for all 40 records to reach processed/non-uncertain state, and persisted their structured metadata and `vector(768)` image embeddings. Created ten source-derived animal posts and one unrelated geology control through the post API; their text embeddings are persisted.
- Ran the evaluation CLI against the actual database IDs: 40 images, four categories, eleven posts, 10/10 correct relevant top-1 results, 10/11 coverage, 73/73 precision among accepted candidates, zero incorrect accepted matches, and one abstention.
- Exercised match inspection and the review endpoint. An AI-accepted red fox suggestion was approved with a note; an AI-rejected gray wolf candidate was rejected with a note. Two append-only review rows were verified while the original AI decisions remained unchanged.
- Audited Gemini usage records. Image classifications, image embeddings, subject resolutions, and text embeddings each recorded latency and outcome. Known paid-list estimates total `$0.03182675`; several calls have unknown cost because token usage was not returned. The figures are estimates, not billed costs.
- Updated `internal/providers/gemini` with the verified 3.1 Flash-Lite price rate and a unit test. Updated successful post persistence so the post row and its subject-resolution/text-embedding usage rows commit together. Updated evaluation image-set validation to count the four verified manifest categories rather than the provider's broad `mammal`/`bird` labels.
- Audited tenant filters and secret handling. Services still resolve a single development tenant; caller authentication and production multi-tenant isolation remain unimplemented. The local `.env` remained mode `0600`, and no secret values were printed.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, and `docker compose config --quiet` passed using a writable Go build cache under `/tmp`.
- Docker Hub DNS prevented `docker compose --profile ai up --build -d`. To verify the current source, compiled static binaries locally and built a temporary scratch image; `docker compose --profile ai up -d --no-build` started the API and worker while retaining the original PostgreSQL container and volume. Repeat a normal Compose build when registry DNS is available.
- Updated README, EVIDENCE, architecture, capstone manifest, and curated dataset documentation with measured results and remaining limits. No Git initialization, remote creation, commit, or push was performed.

## Fresh-database release preparation — 2026-11-12

- Added `scripts/bootstrap-evaluation.py` to seed or identify the development tenant, import/reuse the 40 attributed JPEGs, wait for real Gemini processing, create/reuse 11 source-derived posts, generate ignored per-database UUID manifests, and run the existing evaluation CLI. Verified it against a separate fresh Compose PostgreSQL volume without targeting the existing local volume.
- The fresh-database evaluation returned 40 images, four curated categories, 11 articles, top-1 precision 1.0, coverage 10/11, accepted-recommendation precision 1.0, zero incorrect accepted matches, and one abstention. A narrow plural normalization for the four curated species fixed one false abstention; Go unit, race, and vet checks passed.
- Repeated live API checks for stored classification metadata, 768-dimensional image/article embeddings, fox acceptance, wolf rejection, unrelated-content abstention, approval/rejection reviews, and unchanged AI decisions. PostgreSQL aggregates verified 40 image vectors, 11 article vectors, review history, and provider usage for successful and failed requests. Text embedding usage had no provider token counts or cost estimate.
- Retried `docker compose --profile ai up --build -d` in the isolated project. Docker could not resolve the `golang:1.26.0-alpine3.23` image because the registry connection was unavailable, so a clean registry-backed build remains unverified. The local scratch image was used for live runtime checks.
- Confirmed the `.env` is mode `0600` and ignored, with no API/private key material found outside that file. Reviewed dataset source and license attribution. The MIT license names `Nezzy-joe`; confirmation of legal ownership remains outstanding. No Git initialization, commit, remote or push occurred during the acceptance verification.

## GitHub release preparation — 2026-11-12

- The workspace had no Git repository. Initialized a new repository on `main` without replacing existing history.
- Measured the 40 curated JPEGs at 7,656,543 bytes. Kept the local files, added each SHA-256 to the source metadata, excluded JPEG assets and runtime UUID manifests from Git, and added `scripts/download-curated-dataset.py` for missing-file download and checksum verification.
- `python3 scripts/download-curated-dataset.py --verify-only` verified all 40 existing files without downloading or replacing them. The clean download path requires Wikimedia Commons network access.
- Updated README, DESIGN, ROADMAP, evaluation instructions, capstone manifest and evidence to distinguish implemented behavior, measured results, and remaining limits. Staged-file review, secret scan, and `git diff --cached --check` were completed before the initial local commit.
- The requested initial commit is local only; no remote or push was created. MIT ownership confirmation and a clean registry-backed Docker build remain release limitations.
