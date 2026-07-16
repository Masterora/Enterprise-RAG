CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    password_hash TEXT,
    language VARCHAR(20) NOT NULL DEFAULT 'zh-CN',
    nickname VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

ALTER TABLE users
ADD COLUMN IF NOT EXISTS language VARCHAR(20) NOT NULL DEFAULT 'zh-CN';

ALTER TABLE users
ADD COLUMN IF NOT EXISTS nickname VARCHAR(100) NOT NULL DEFAULT '';

UPDATE users
SET nickname = username
WHERE nickname = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique_active
ON users(username)
WHERE deleted_at IS NULL;
