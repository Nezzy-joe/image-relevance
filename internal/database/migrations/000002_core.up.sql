CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sha256 CHAR(64) NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png')),
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'processed', 'needs_review', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, sha256)
);

CREATE TABLE image_analysis (
    image_id UUID PRIMARY KEY REFERENCES images(id) ON DELETE CASCADE,
    subject TEXT NOT NULL CHECK (length(btrim(subject)) BETWEEN 1 AND 120),
    subject_key TEXT NOT NULL CHECK (length(btrim(subject_key)) BETWEEN 1 AND 120),
    category TEXT NOT NULL CHECK (length(btrim(category)) BETWEEN 1 AND 64),
    attributes JSONB NOT NULL CHECK (jsonb_typeof(attributes) = 'array'),
    caption TEXT NOT NULL CHECK (length(btrim(caption)) BETWEEN 1 AND 500),
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    uncertain BOOLEAN NOT NULL,
    model TEXT NOT NULL,
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE image_embeddings (
    image_id UUID PRIMARY KEY REFERENCES images(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    embedding vector(768) NOT NULL,
    model TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX image_embeddings_cosine_idx ON image_embeddings USING hnsw (embedding vector_cosine_ops);
CREATE INDEX images_tenant_status_idx ON images (tenant_id, status, created_at DESC);

CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    body TEXT NOT NULL CHECK (length(btrim(body)) BETWEEN 1 AND 20000),
    subject TEXT NOT NULL CHECK (length(btrim(subject)) BETWEEN 1 AND 120),
    subject_key TEXT NOT NULL CHECK (length(btrim(subject_key)) BETWEEN 1 AND 120),
    subject_confidence DOUBLE PRECISION NOT NULL CHECK (subject_confidence >= 0 AND subject_confidence <= 1),
    embedding vector(768) NOT NULL,
    embedding_model TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX posts_tenant_created_idx ON posts (tenant_id, created_at DESC);

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    image_id UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL CHECK (job_type = 'analyze_image'),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX jobs_claim_idx ON jobs (available_at, created_at) WHERE status = 'queued';
CREATE UNIQUE INDEX jobs_active_image_type_idx ON jobs (image_id, job_type)
    WHERE status IN ('queued', 'running');

CREATE TABLE ai_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    image_id UUID REFERENCES images(id) ON DELETE SET NULL,
    post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    operation TEXT NOT NULL,
    input_tokens BIGINT CHECK (input_tokens >= 0),
    output_tokens BIGINT CHECK (output_tokens >= 0),
    latency_ms BIGINT NOT NULL CHECK (latency_ms >= 0),
    success BOOLEAN NOT NULL,
    error_code TEXT,
    cost_usd NUMERIC(12, 8) CHECK (cost_usd IS NULL OR cost_usd >= 0),
    cost_basis TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ai_usage_tenant_created_idx ON ai_usage (tenant_id, created_at DESC);

CREATE TABLE match_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    image_id UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    rank INTEGER NOT NULL CHECK (rank > 0),
    similarity DOUBLE PRECISION NOT NULL CHECK (similarity >= -1 AND similarity <= 1),
    accepted BOOLEAN NOT NULL,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE review_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    suggestion_id UUID NOT NULL REFERENCES match_suggestions(id) ON DELETE CASCADE,
    decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    comment TEXT CHECK (comment IS NULL OR length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX review_actions_suggestion_idx ON review_actions (suggestion_id, created_at);
