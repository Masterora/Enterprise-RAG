CREATE TABLE users (
    id UUID PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    password_hash TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE TABLE subjects (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    visibility VARCHAR(50) NOT NULL DEFAULT 'private',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE TABLE documents (
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
CREATE INDEX idx_documents_subject_id ON documents(subject_id);
CREATE INDEX idx_documents_user_id ON documents(user_id);
CREATE INDEX idx_documents_status ON documents(status);

CREATE TABLE document_chunks (
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
CREATE INDEX idx_document_chunks_doc_id ON document_chunks(doc_id);
CREATE INDEX idx_document_chunks_subject_id ON document_chunks(subject_id);
CREATE INDEX idx_document_chunks_user_id ON document_chunks(user_id);
CREATE INDEX idx_document_chunks_index_version ON document_chunks(index_version);

CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    subject_id UUID,
    title VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL,
    user_id UUID NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    citations JSONB,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE index_tasks (
    id UUID PRIMARY KEY,
    doc_id UUID,
    subject_id UUID,
    user_id UUID,
    task_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);
CREATE INDEX idx_index_tasks_doc_id ON index_tasks(doc_id);
CREATE INDEX idx_index_tasks_status ON index_tasks(status);
CREATE INDEX idx_index_tasks_task_type ON index_tasks(task_type);

CREATE TABLE document_parse_logs (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
