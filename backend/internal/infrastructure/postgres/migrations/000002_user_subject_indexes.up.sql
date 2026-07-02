CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_unique ON users(username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subjects_owner_id ON subjects(owner_id);
CREATE INDEX IF NOT EXISTS idx_subjects_visibility ON subjects(visibility);
