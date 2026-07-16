ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS document_version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_provider VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_dimension INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chunk_strategy_version VARCHAR(50) NOT NULL DEFAULT 'v1';

ALTER TABLE document_chunks
    ADD COLUMN IF NOT EXISTS document_version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_provider VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_dimension INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chunk_strategy_version VARCHAR(50) NOT NULL DEFAULT 'v1';

ALTER TABLE index_tasks
    ADD COLUMN IF NOT EXISTS document_version INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_provider VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS embedding_dimension INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS chunk_strategy_version VARCHAR(50) NOT NULL DEFAULT 'v1';

CREATE TABLE IF NOT EXISTS task_outbox (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL UNIQUE REFERENCES index_tasks(id),
    subject VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE task_outbox
ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_task_outbox_pending
ON task_outbox (next_attempt_at, created_at)
WHERE published_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_chunks_version_hash
ON document_chunks (doc_id, document_version, content_hash, chunk_index)
WHERE deleted_at IS NULL;
