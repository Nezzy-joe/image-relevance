# image-relevance architecture

**Description:** AI Image Understanding & Content Matching Engine<br>
**Origin:** FlyRank Backend AI Engineering Capstone

The current implementation is a Go modular monolith with separate API and worker processes, PostgreSQL/pgvector persistence, and one Gemini provider adapter. Development business routes use the seeded development tenant; tenant authentication is not yet implemented.

```mermaid
flowchart LR
    Client --> API[Chi API]
    API --> Middleware[Request ID / access log / recovery]
    Middleware --> ImageService[Image upload service]
    Middleware --> MatchService[Post and matching service]
    ImageService --> Disk[(Shared image volume)]
    ImageService --> PG[(PostgreSQL + pgvector)]
    MatchService --> Gemini[Gemini API]
    MatchService --> PG
    PG --> Queue[PostgreSQL jobs]
    Queue --> Worker[Go worker]
    Worker --> Disk
    Worker --> Gemini
    Worker --> PG
```

## Implemented request and processing paths

1. `POST /v1/images` validates JPEG/PNG bytes, size and dimensions; calculates SHA-256; stores the bytes by content hash; creates an image row and `analyze_image` job in PostgreSQL.
2. The worker claims jobs with `SELECT ... FOR UPDATE SKIP LOCKED`, commits the claim before provider requests, and limits concurrency. It records provider usage, validates structured metadata and embedding dimensions, then persists results and job/image states transactionally.
3. `POST /v1/posts` asks Gemini to resolve the article subject and embed the article text using the same embedding model/dimensions as the original image content.
4. `POST /v1/posts/{postID}/matches` retrieves nearest direct-image vectors with cosine distance. A phrase-aware subject-key guard, image uncertainty, article subject confidence, and similarity threshold determine eligibility. Rejected candidates carry a reason. The service returns `no_confident_match` instead of forcing a result.
5. `POST /v1/matches/{suggestionID}/review` appends a review action. It does not alter the stored AI suggestion.

The business routes are registered only for `APP_ENV=development`. The production environment still lacks tenant authentication and must not expose these routes.

## Runtime processes

- `cmd/api` loads configuration, opens a pgx pool, applies embedded migrations, and serves HTTP with request IDs, structured logs, recovery, bounded request timeouts, and graceful shutdown.
- `cmd/seed` idempotently creates the development tenant and refuses other environments.
- `cmd/worker` requires `GEMINI_API_KEY`, applies migrations, and runs the PostgreSQL-backed queue. It is behind Compose's `ai` profile so the API/database foundation can run without provider credentials.
- `cmd/evaluate` validates required image/article manifest fields and supplied source/license metadata, checks image rows in PostgreSQL, executes actual matching, and prints calculated metrics. Metadata validation does not independently establish legal usage rights.

The host-run API defaults to `127.0.0.1:8080`. Compose overrides the container listener to `0.0.0.0:8080` for internal networking while publishing the host port only on `127.0.0.1:8080`. The HTTP write timeout defaults to 65 seconds; each Gemini request is limited to 25 seconds. Configuration rejects a write timeout that cannot accommodate two sequential provider calls plus a 10-second response margin, which covers article subject resolution followed by embedding without removing finite timeouts.

## Implemented frontend

The standalone React/TypeScript app in `web/` provides the responsive workspace shell, live overview, and image library. It uses TanStack Query for API state, Zod to validate client responses, and a Vite `/api` proxy to the loopback Go API. The Vite server binds to `127.0.0.1:5173` by default. No Gemini key or database URL is used by the browser.

The overview reads `/health`, `/ready`, `/v1/images`, and `/v1/matches`. The history read service is wired independently of the Gemini-backed matching operations, so saved decisions remain readable when the provider key is absent. The image library uploads through `POST /v1/images`, polls `GET /v1/images/{imageID}`, lists filtered pages from `GET /v1/images`, and loads previews through the tenant-scoped content route. Content paths are selected by the backend and never returned to the browser.

Matching Studio creates posts and runs matching through the API, rendering accepted and rejected candidates, mismatch explanations, and `NO_CONFIDENT_MATCH` responses. Decision Inspector presents source article details, candidate metadata, scores, and the original backend decision. Review Centre lists persisted decisions, submits approval or rejection with an optional note, and reads append-only review history. The frontend tests mock the HTTP boundary; live PostgreSQL and Gemini evidence is reported separately in `EVIDENCE.md`. The current business API remains development-tenant-only and has no authenticated reviewer or tenant selection.

## Final capstone demonstration

The curated Commons dataset has 40 visually reviewed images across four species categories. Evaluation categories come from the reviewed manifest; the model's broader `image_analysis.category` values (such as `mammal` or `bird`) are not treated as fine-grained species labels. Ten source-derived animal posts have category-level positive labels, and an unrelated geology post is the no-suitable control. `scripts/bootstrap-evaluation.py` regenerates ignored runtime UUID manifests for the current database from stable filenames and source keys.

The 2026-11-12 fresh isolated-database evaluation returned 10/10 correct top-1 matches across the ten relevant posts, top-1 precision `1.0`, 10/11 recommendation coverage, accepted-recommendation precision `1.0`, zero incorrect accepted matches, and one abstention. The image/category labels have a single visual reviewer and no independent adjudication, so these scores are demonstration evidence rather than a general quality claim.

Provider usage records are written for classification, subject resolution, and embedding requests. Records include provider/model/operation, outcome, latency, available token counts, and a cost basis. Costs are list-price estimates only when a verified rate and usage data are available; text-embedding responses in the fresh run supplied no token counts, so those rows have no estimate. See `EVIDENCE.md` for observed counts and totals.

## Data and package layout

```text
cmd/api/                    HTTP process wiring
cmd/seed/                   development tenant seed command
cmd/worker/                 background worker process
cmd/evaluate/               labeled evaluation CLI
scripts/download-curated-dataset.py Commons image download with SHA-256 verification
scripts/bootstrap-evaluation.py        corpus importer and runtime UUID manifest generator
internal/config/            environment parsing and safe defaults
internal/database/          pgx pool and embedded migrations
internal/database/migrations/ SQL migrations embedded in API and worker
internal/domain/            image metadata validation and subject keys
internal/evaluation/        licensed manifest validation and metrics
internal/httpapi/           operational, image, post, match, review routes
internal/ingestion/         image validation, storage, deduplication, job creation
internal/matching/          post creation, vector retrieval, mismatch guard, review
internal/middleware/        request ID, access logging, panic recovery
internal/providers/gemini/  Google GenAI client adapter and usage metadata
internal/worker/            queue claiming, retries, persistence, usage capture
```

Postgres migration files live at `internal/database/migrations/`. Image bytes use a separate persistent Docker volume shared by API and worker. The existing local `flyrank` database/user and `postgres_data` volume are retained; changing Compose project/database identity requires an explicit data migration.

Raw-image vectors are stored separately in `image_embeddings`; article-text vectors are stored on `posts`. Vision-generated caption text is stored in `image_analysis.caption` and is not used as a substitute for the direct-image vector.

## Migrations and cancellation

`database.Migrate` uses its context while pinging PostgreSQL. The current `golang-migrate` `m.Up()` API has no context parameter, so cancellation cannot interrupt a migration once `m.Up()` starts. Keep startup migrations short; before long-running changes, use database lock/statement timeouts or a separate migration procedure. No alternate migration framework is added.

## Current limits

- All routes resolve the same seeded development tenant. There is no authenticated caller identity, tenant selection, or production multi-tenant boundary; tenant predicates in matching/review are not production authentication.
- The worker carries tenant IDs from queued jobs and writes provider usage with those IDs. Tenant/image consistency for job data is based on service-created rows; production tenant authentication and cross-tenant integration tests are absent.
- Review history is append-only and stores decision/comment/time, but the current route does not persist an authenticated reviewer identity.
- Subject compatibility accepts matching normalized phrases when one subject is a specific form of the other (for example, `red fox hunting behavior` and `red fox`); broader taxonomy aliases and cross-language matching are not implemented.
- Similarity and confidence floors are configurable defaults and need calibration against a licensed labeled dataset.
- `gemini-3.1-flash-lite` generation and structured image classification, plus persisted image/text embeddings and fox/wolf/no-confident-match scenarios, were tested end to end locally. The curated dataset and benchmark run are documented above; independent human label review remains outstanding.
- Provider token usage, latency, status, and cost basis are persisted. A list-price USD estimate is calculated only for models with a verified rate and available token counts; it is not an invoice amount and excludes account discounts/free tier.
- The runtime evaluation CSVs contain UUIDs for the active database and are ignored by Git. Run the bootstrap script to identify/create the tenant, re-ingest or reuse assets, create/reuse source posts, regenerate the UUID maps, and rerun evaluation.
- A clean registry-backed Docker Compose image build could not be verified in this environment because Docker Hub connectivity failed. A local scratch image can run the stack, but this is not evidence of a clean registry-backed build.
