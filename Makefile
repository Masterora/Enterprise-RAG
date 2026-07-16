.PHONY: verify verify-api verify-agent verify-web verify-compose verify-race test-e2e evaluate

UV ?= uv

verify: verify-api verify-agent verify-web

verify-api:
	cd api && go test ./...
	cd api && go vet ./...
	cd api && go build ./...

verify-agent:
	cd agent && $(UV) sync --locked --extra dev
	cd agent && $(UV) run ruff check src tests
	cd agent && $(UV) run mypy src
	cd agent && $(UV) run pytest

verify-web:
	cd web && pnpm lint
	cd web && pnpm build

verify-compose:
	AGENT_SERVICE_TOKEN=0123456789abcdef OPENROUTER_API_KEY=verification-key RAG_AUTH_SECRET=verification-auth-secret-0123456789 REDIS_PASSWORD=verification-redis-password POSTGRES_DSN='postgresql://rag:rag@postgres:5432/rag?sslmode=disable' docker compose config --quiet

verify-race:
	cd api && go test -race ./internal/infrastructure/agent ./internal/service/retrieval ./internal/worker

test-e2e:
	./tests/e2e/run.sh

evaluate:
	@test -n "$(CASES)" || (echo "CASES is required" && exit 1)
	@test -n "$$RAG_API_TOKEN" || (echo "RAG_API_TOKEN is required" && exit 1)
	cd api && go run ./cmd/evaluate -file "$(abspath $(CASES))" $(ARGS)
