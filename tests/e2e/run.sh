#!/bin/sh
set -eu

project="${E2E_PROJECT:-enterprise-rag-e2e}"
compose="docker compose -p $project -f docker-compose.yml -f tests/e2e/docker-compose.e2e.yml"

export AGENT_SERVICE_TOKEN="${AGENT_SERVICE_TOKEN:-0123456789abcdef}"
export OPENROUTER_API_KEY="${OPENROUTER_API_KEY:-e2e-key}"
export RAG_AUTH_SECRET="${RAG_AUTH_SECRET:-e2e-auth-secret-0123456789abcdef}"
export REDIS_PASSWORD="${REDIS_PASSWORD:-e2e-redis-password}"
export POSTGRES_DB="${E2E_POSTGRES_DB:-rag}"
export POSTGRES_USER="${E2E_POSTGRES_USER:-rag}"
export POSTGRES_PASSWORD="${E2E_POSTGRES_PASSWORD:-e2e-postgres-password}"
export POSTGRES_DSN="${E2E_POSTGRES_DSN:-postgresql://$POSTGRES_USER:$POSTGRES_PASSWORD@postgres:5432/$POSTGRES_DB?sslmode=disable}"
export API_HTTP_PORT="${API_HTTP_PORT:-19999}"
export POSTGRES_PORT="${POSTGRES_PORT:-55433}"
export REDIS_PORT="${REDIS_PORT:-16379}"
export NATS_PORT="${NATS_PORT:-14222}"
export NATS_MONITOR_PORT="${NATS_MONITOR_PORT:-18223}"
export MINIO_PORT="${MINIO_PORT:-19000}"
export MINIO_CONSOLE_PORT="${MINIO_CONSOLE_PORT:-19001}"
export MILVUS_PORT="${MILVUS_PORT:-19531}"
export MILVUS_HEALTH_PORT="${MILVUS_HEALTH_PORT:-19091}"
export E2E_API_URL="${E2E_API_URL:-http://localhost:$API_HTTP_PORT}"

cleanup() {
    status=$?
    trap - EXIT INT TERM
    if [ "$status" -ne 0 ]; then
        $compose logs --no-color || true
    fi
    $compose down -v --remove-orphans || true
    exit "$status"
}
trap cleanup EXIT INT TERM

$compose up -d --build --wait api agent
python3 tests/e2e/run.py
