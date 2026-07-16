CREATE TABLE IF NOT EXISTS chat_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    subject_id UUID,
    title VARCHAR(255),
    llm_provider VARCHAR(100),
    llm_model VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS llm_provider VARCHAR(100);
ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS llm_model VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_updated
    ON chat_sessions(user_id, updated_at DESC);
