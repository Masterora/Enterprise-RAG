ALTER TABLE users
ADD COLUMN IF NOT EXISTS language VARCHAR(20) NOT NULL DEFAULT 'zh-CN';

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique_active
ON users(username)
WHERE deleted_at IS NULL;
