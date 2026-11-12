# image-relevance

**AI Image Understanding & Content Matching Engine**<br>
Public repository: `github.com/Nezzy-joe/image-relevance`<br>
Origin: FlyRank Backend AI Engineering Capstone

image-relevance is a Go backend that accepts image uploads, extracts structured visual metadata, embeds original image content and article text, and ranks compatible image candidates. It abstains when the subject guard, confidence floor, or similarity floor rejects every candidate.

## Implementation status

### Implemented in source

- Go HTTP API using Chi, request IDs, structured access logs, panic recovery, bounded timeouts, and graceful shutdown.
- PostgreSQL connection pooling, pgvector extension, and embedded migrations under `internal/database/migrations/`.
- `GET /health` process liveness and `GET /ready` PostgreSQL readiness.
- Idempotent development tenant seed command: `cmd/seed`.
- Development-only JPEG/PNG upload, content validation, SHA-256 deduplication, persistent file storage, and PostgreSQL job creation.
- PostgreSQL worker using `FOR UPDATE SKIP LOCKED`, bounded concurrency, retry backoff, job status tracking, and graceful cancellation.
- Gemini adapter for structured image analysis, article subject resolution, and 768-dimensional image and article embeddings. Provider usage, available token/latency data, and a list-price cost estimate (when the model and usage have a verified rate) are recorded; this estimate is not a billed amount.
- Cosine candidate ranking, phrase-aware subject compatibility checks, uncertainty/similarity rejection reasons, and `no_confident_match` abstention.
- Append-only human review actions.
- Evaluation CLI that validates a licensed image manifest and labeled article set before calculating metrics from current matching results.

### Tested independently

- Google model metadata reports `gemini-3.1-flash-lite` available with `generateContent` support. One minimal text request and one structured JPEG classification returned HTTP 200; the image JSON passed local schema validation.
- Real JPEG and article text calls to `gemini-embedding-2` returned 768-dimensional vectors.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, and `docker compose config --quiet` pass.

### Tested end to end — 2026-11-12

- On a separate fresh Compose database volume, the bootstrap imported and processed 40 visually reviewed, attributed Commons JPEGs and created/reused 11 source-derived article posts. It generated database-specific UUID manifests and ran the existing evaluation CLI.
- The fresh-database evaluation reported 40 images in four curated categories and 11 articles: top-1 precision `1.0`, recommendation coverage `0.9090909090909091` (10/11), accepted-recommendation precision `1.0`, zero incorrect accepted matches, and one abstention.
- The live review route returned an accepted red fox suggestion and a rejected gray wolf candidate. Both were reviewed with notes; PostgreSQL retained the original AI accept/reject decisions and two review audit rows.
- `/health` and `/ready` returned HTTP 200. All 40 image vectors and 11 article vectors were persisted at 768 dimensions. PostgreSQL and pgvector remained available in the isolated database.

### Still outstanding

- The image and article relevance labels have one visual reviewer and have not received independent human adjudication. Treat the measured scores as a real demonstration result, not a final quality claim about broad real-world performance.
- Evaluation UUIDs are generated per database under the ignored `evaluation-data/curated/runtime/` directory. `scripts/bootstrap-evaluation.py` seeds the development tenant, imports/reuses images, waits for worker processing, creates/reuses the 11 source-derived posts, writes fresh mappings and runs the evaluation CLI.
- Tenant IDs scope matching and review queries, but routes always select one seeded development tenant. There is no caller identity, tenant selection, or production authentication boundary. Production multi-tenant isolation is not implemented or tested.
- Review audit rows store decision, comment, and time, but do not identify an authenticated reviewer.
- Gemini text-embedding responses returned no token counts in this run, so those calls have no cost estimate. Recorded list-price estimates are not invoices and exclude free tier, credits, and discounts.
- The standard Docker build requires registry access for its `golang:1.26.0-alpine3.23` build image. Docker Hub connectivity failed during the recorded check; do not treat the local scratch-image run as a clean registry-backed build.

Image, post, matching, and review routes are development-only and use the seeded tenant. Do not expose this API to untrusted networks.

## Run locally

Requirements: Go 1.26 or compatible newer Go, Docker Engine, Docker Compose v2, and Python 3 for the evaluation bootstrap (standard library only).

```sh
cp .env.example .env
docker compose up --build -d
docker compose exec -T api /seed
```

The default Compose stack starts PostgreSQL and the API. It keeps the existing local `flyrank` database/user defaults, binds PostgreSQL to localhost port `15432`, and retains the existing `postgres_data` volume. Do not change the Compose project identity for an existing database unless you intentionally migrate its volume. The default local credentials are for development only.

For Gemini processing, put a valid key in the ignored local `.env` file, then start the optional worker profile:

```sh
docker compose --profile ai up --build -d
```

Compose passes the key through the worker/API environment; it is not written to logs or database records. Set `GEMINI_VISION_MODEL` and `GEMINI_EMBEDDING_MODEL` only to model IDs supported by your Google AI project.

The build needs Docker Hub access to pull its Go build image. Image classification, article processing, and embedding generation also need outbound access to the configured Google Gemini API. Do not paste keys into commands or commit `.env`.

## API quick start

Check operational endpoints:

```sh
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

Upload a JPEG or PNG (maximum 10 MiB by default). The API returns `202` and the worker processes it asynchronously:

```sh
curl -i -F 'file=@./image.jpg' http://localhost:8080/v1/images
curl -i http://localhost:8080/v1/images/IMAGE_UUID
```

Create a post and request matches (Gemini key required):

```sh
curl -i -H 'Content-Type: application/json' \
  -d '{"title":"Red fox behaviour","body":"An overview of red fox behaviour and habitat."}' \
  http://localhost:8080/v1/posts
curl -i -X POST http://localhost:8080/v1/posts/POST_UUID/matches
```

The match response includes accepted suggestions and rejected candidates with reasons. When nothing passes, it returns `status: "no_confident_match"`, an empty `suggestions` array, and an abstention reason. Review a recorded decision with `POST /v1/matches/{suggestionID}/review` and JSON `{"decision":"approved"}` or `{"decision":"rejected","comment":"..."}`. Review history is append-only.

See [docs/api-contract.md](docs/api-contract.md) and [schemas/openapi.yaml](schemas/openapi.yaml) for request and response contracts.

## Evaluation

The CLI requires actual processed images and labeled posts; it will not generate a benchmark or fabricate metrics. The curated source metadata, article sources, and author/license details are in [`evaluation-data/curated/`](evaluation-data/curated/). The 40 JPEGs total 7,656,543 bytes (about 7.3 MiB), so Git stores their source metadata and SHA-256 checksums rather than the image files. Stable dataset records use filenames and source keys; database UUIDs are created at runtime and ignored by Git.

- `images.csv`: `image_id,category,source_url,license`, at least 40 images across four categories.
- `articles.csv`: `post_id,expected_image_ids,negative_image_ids,scenario`, at least 10 posts including `positive`, `mismatch_guard`, and `no_suitable` cases.

Use `|` between multiple image IDs. Keep source and license information with the dataset and ensure its terms permit evaluation. On a fresh checkout, fetch missing JPEGs from their Commons source pages and verify their recorded checksums:

```sh
python3 scripts/download-curated-dataset.py
```

The downloader never overwrites an existing file and leaves `sources.csv` and `ATTRIBUTION.md` as the licensing record. Wikimedia Commons network access is required when a file is missing. After starting the stack and configuring the local Gemini key, run the reproducible import and evaluation:

```sh
python3 scripts/bootstrap-evaluation.py
```

The script calls the idempotent tenant seed, imports the 40 curated assets, waits for their real worker analysis and embeddings, creates or reuses the 11 source-derived posts, writes ignored runtime manifests, and runs `/evaluate`. To rerun only the CLI after bootstrap:

```sh
docker compose exec -T api /evaluate \
  -images /evaluation-data/curated/runtime/images.csv \
  -articles /evaluation-data/curated/runtime/articles.csv
```

The fresh isolated database run produced 10/10 correct top-1 results among the ten relevant posts, top-1 precision `1.0`, recommendation coverage `10/11`, accepted-recommendation precision `1.0`, zero incorrect accepted matches, and one abstention. The no-suitable control is excluded from the top-1 denominator and included in recommendation coverage. Labels are category-level and have one visual reviewer; metric definitions are in [docs/evaluation.md](docs/evaluation.md).

## Development checks

```sh
gofmt -w $(find cmd internal -name '*.go' -type f)
go mod tidy
go test ./...
go test -race ./...
go vet ./...
docker compose config --quiet
```

The Phase 1 database/user and persistent volume retain historical `flyrank` identifiers for local data compatibility. Renaming them requires a planned migration; this setup does not recreate or remove existing database storage.

See [ARCHITECTURE.md](ARCHITECTURE.md), [DESIGN.md](DESIGN.md), [docs/database-erd.md](docs/database-erd.md), [BUILDLOG.md](BUILDLOG.md), and [EVIDENCE.md](EVIDENCE.md) for implementation and verification details.
