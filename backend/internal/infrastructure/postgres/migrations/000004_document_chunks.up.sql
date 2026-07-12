CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL,
    subject_id UUID NOT NULL,
    user_id UUID NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    page INT,
    section TEXT,
    metadata JSONB,
    token_count INT,
    index_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_document_chunks_doc_id ON document_chunks(doc_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_subject_id ON document_chunks(subject_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_user_id ON document_chunks(user_id);
CREATE INDEX IF NOT EXISTS idx_document_chunks_index_version ON document_chunks(index_version);
