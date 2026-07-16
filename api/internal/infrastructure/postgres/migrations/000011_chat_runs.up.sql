CREATE TABLE IF NOT EXISTS chat_runs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    session_id UUID,
    message_id UUID,
    subject_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL,
    request JSONB NOT NULL,
    result JSONB,
    error_message TEXT,
    cancel_requested BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS chat_run_events (
    run_id UUID NOT NULL,
    sequence BIGINT NOT NULL,
    tenant_id UUID NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_chat_runs_tenant_user_created
ON chat_runs (tenant_id, user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_runs_status
ON chat_runs (status, updated_at);

CREATE INDEX IF NOT EXISTS idx_chat_run_events_tenant_run
ON chat_run_events (tenant_id, run_id, sequence);
