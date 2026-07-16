CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

INSERT INTO tenants (id, name, created_at, updated_at)
SELECT id, COALESCE(NULLIF(nickname, ''), username), created_at, updated_at
FROM users
ON CONFLICT (id) DO NOTHING;

ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE users SET tenant_id = id WHERE tenant_id IS NULL;
ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE subjects ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE subjects SET tenant_id = owner_id WHERE tenant_id IS NULL;
ALTER TABLE subjects ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE documents ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE documents SET tenant_id = user_id WHERE tenant_id IS NULL;
ALTER TABLE documents ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE document_chunks ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE document_chunks SET tenant_id = user_id WHERE tenant_id IS NULL;
ALTER TABLE document_chunks ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE index_tasks ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE index_tasks SET tenant_id = user_id WHERE tenant_id IS NULL;
ALTER TABLE index_tasks ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE chat_sessions SET tenant_id = user_id WHERE tenant_id IS NULL;
ALTER TABLE chat_sessions ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS tenant_id UUID;
UPDATE chat_messages SET tenant_id = user_id WHERE tenant_id IS NULL;
ALTER TABLE chat_messages ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_tenant ON users (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subjects_tenant ON subjects (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_tenant_subject ON documents (tenant_id, subject_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_chunks_tenant_subject ON document_chunks (tenant_id, subject_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_index_tasks_tenant ON index_tasks (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_tenant_user ON chat_sessions (tenant_id, user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_chat_messages_tenant_session ON chat_messages (tenant_id, session_id);
