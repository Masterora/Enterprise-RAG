CREATE TABLE IF NOT EXISTS index_tasks (
    id UUID PRIMARY KEY,
    doc_id UUID,
    subject_id UUID,
    user_id UUID,
    task_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB,
    cleared_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

ALTER TABLE index_tasks
ADD COLUMN IF NOT EXISTS cleared_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_index_tasks_doc_id ON index_tasks(doc_id);
CREATE INDEX IF NOT EXISTS idx_index_tasks_status ON index_tasks(status);
CREATE INDEX IF NOT EXISTS idx_index_tasks_task_type ON index_tasks(task_type);
CREATE INDEX IF NOT EXISTS idx_index_tasks_user_cleared
ON index_tasks (user_id, cleared_at);
