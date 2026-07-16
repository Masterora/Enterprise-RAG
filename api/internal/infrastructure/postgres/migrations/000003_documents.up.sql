CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY,
    subject_id UUID NOT NULL,
    user_id UUID NOT NULL,
    filename VARCHAR(500) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    file_url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'uploaded',
    plain_text TEXT,
    metadata JSONB,
    index_version INT NOT NULL DEFAULT 1,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_documents_subject_id ON documents(subject_id);
CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
