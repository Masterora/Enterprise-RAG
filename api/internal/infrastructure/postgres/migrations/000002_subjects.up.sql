CREATE TABLE IF NOT EXISTS subjects (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    visibility VARCHAR(50) NOT NULL DEFAULT 'private',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subjects_owner_id ON subjects(owner_id);
CREATE INDEX IF NOT EXISTS idx_subjects_visibility ON subjects(visibility);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subjects_owner_name_active
ON subjects (owner_id, lower(name))
WHERE deleted_at IS NULL;
