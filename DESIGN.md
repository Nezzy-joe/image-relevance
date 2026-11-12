# image-relevance — design and status

**Description:** AI Image Understanding & Content Matching Engine<br>
**Public repository:** `github.com/Nezzy-joe/image-relevance`<br>
**Origin:** FlyRank Backend AI Engineering Capstone

## Mission

Understand image-library assets and recommend them for blog posts while rejecting explicit subject mismatches and abstaining when no candidate is safe to recommend.

## Implemented architecture

- **Go modular monolith:** Chi API and a separate Go worker share PostgreSQL-backed services and migrations.
- **PostgreSQL and pgvector:** pgx pool, embedded migrations, durable image jobs, analysis/usage rows, and 768-dimensional vectors.
- **Gemini only:** Google GenAI Go SDK adapter for structured image analysis, article subject resolution, and embeddings. There is no local-model fallback.
- **Fail-closed matching:** phrase-aware normalized subject compatibility runs after cosine retrieval; uncertain image classifications and low-confidence posts are rejected; a configurable similarity floor can abstain.
- **Append-only review:** approvals/rejections are recorded as new actions without rewriting the AI suggestion.
- **Evidence-led costs:** provider token counts and latency are stored when returned. USD list-price cost is estimated only when the model has a verified rate and usage counts; it is not represented as an invoice amount.

## Current limits

- API business routes are development-only because tenant API-key authentication is not implemented.
- Subject phrase compatibility accepts a more specific article phrase such as `red fox hunting behavior` for a `red fox` image, while rejecting gray wolves. It does not implement a taxonomy alias graph.
- The default confidence/similarity thresholds are configurable starting values, not calibrated benchmark thresholds.
- `gemini-3.1-flash-lite` minimal text generation and structured JPEG classification returned HTTP 200. Real fox and wolf image metadata and raw-image vectors, plus article text vectors, were persisted and exercised through the match route.
- The curated dataset contains 40 visually reviewed Commons JPEGs across red fox, gray wolf, cheetah, and bald eagle categories. Ten source-derived animal posts each have a manually checked positive example image; an unrelated geology article is the no-suitable control. The fresh-database CLI run returned top-1 precision 1.0, coverage 10/11, accepted-recommendation precision 1.0, zero incorrect accepted matches, and one abstention. One reviewer produced the labels; independent adjudication remains outstanding.
- The JPEGs total 7,656,543 bytes and are excluded from Git. `sources.csv` records each file's SHA-256, Commons source, author/credit, and license; `scripts/download-curated-dataset.py` downloads missing assets and verifies checksums without overwriting existing files.

## Verification decisions

- Use the official Google GenAI Go SDK, `google.golang.org/genai`, and currently documented Gemini model IDs; keep model identifiers configurable.
- Use 768 dimensions for direct raw-image and article-text embeddings in the shared model space, record model IDs with rows, and keep caption text separately in image analysis metadata.
- Run provider requests outside SQL transactions. Claim jobs with `FOR UPDATE SKIP LOCKED`, persist results in short transactions, and retry with capped exponential backoff.
- Keep the existing local `flyrank` database/user and persistent Compose volume stable. A rename requires a deliberate data migration.
- `golang-migrate`'s `m.Up()` accepts no context; the existing startup context cannot cancel a migration after it starts.
