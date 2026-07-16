ALTER TABLE index_tasks
ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_index_tasks_running_lease
ON index_tasks (lease_expires_at)
WHERE status = 'running';
