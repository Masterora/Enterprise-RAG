CREATE TABLE IF NOT EXISTS document_parse_logs (
    id UUID PRIMARY KEY,
    doc_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
