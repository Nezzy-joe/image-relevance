# Database schema

The migrations embedded in the API and worker are stored at `internal/database/migrations/`. `000001_initial.up.sql` enables pgvector and creates `tenants`. `000002_core.up.sql` adds the current image, post, job, usage, matching, and review tables. Migrations are additive; no table or existing tenant data is dropped.

```mermaid
erDiagram
    tenants ||--o{ images : owns
    tenants ||--o{ posts : owns
    tenants ||--o{ jobs : schedules
    tenants ||--o{ ai_usage : incurs
    tenants ||--o{ match_suggestions : owns
    tenants ||--o{ review_actions : owns
    images ||--o| image_analysis : analyzed_as
    images ||--o| image_embeddings : embedded_as
    images ||--o{ jobs : processed_by
    posts ||--o{ match_suggestions : ranked_for
    images ||--o{ match_suggestions : candidate
    match_suggestions ||--o{ review_actions : reviewed_by
    posts ||--o{ ai_usage : generates
    images ||--o{ ai_usage : generates

    tenants { uuid id PK; text name; text status }
    images { uuid id PK; uuid tenant_id FK; char sha256; text storage_key; text status }
    image_analysis { uuid image_id PK; text subject; text subject_key; jsonb attributes; float confidence; boolean uncertain }
    image_embeddings { uuid image_id PK; uuid tenant_id FK; vector_768 embedding; text model }
    posts { uuid id PK; uuid tenant_id FK; text subject; float subject_confidence; vector_768 embedding; text embedding_model }
    jobs { uuid id PK; uuid tenant_id FK; uuid image_id FK; text status; integer attempts; timestamptz available_at }
    ai_usage { uuid id PK; uuid tenant_id FK; text provider; text model; text operation; bigint input_tokens; bigint output_tokens; numeric cost_usd }
    match_suggestions { uuid id PK; uuid post_id FK; uuid image_id FK; integer rank; float similarity; boolean accepted; text rejection_reason }
    review_actions { uuid id PK; uuid suggestion_id FK; text decision; text comment; timestamptz created_at }
```

## Data behavior

- Image bytes are stored in the shared `image_data` Docker volume by SHA-256 key; PostgreSQL stores checksum, path key, MIME type, size, and lifecycle state.
- Image and post vectors use the same configured Gemini embedding model and 768 dimensions. pgvector's cosine distance orders candidate retrieval.
- Image classification and subject-resolution confidence are persisted as provider-returned values. Low-confidence images are marked `needs_review` and are excluded from automatic suggestions.
- Job claiming uses `FOR UPDATE SKIP LOCKED`; the database lock is released before provider requests. Retries use capped exponential backoff.
- Every provider call in the image and post workflows writes an `ai_usage` row with available tokens, latency, status, and cost basis. USD cost is estimated only for configured model rates and available token counts; otherwise it remains `NULL`.
- Match suggestions are append-only per run. Review actions are appended and do not rewrite the AI decision.
- The current API binds development data to the seeded tenant. Production tenant authentication and authorization are not implemented.
