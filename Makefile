BINARY=hermes
VERSION=0.7.0
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build test clean install run lint quickstart test-e2e test-e2e-headed teardown bootstrap test-k8s webui webui-teardown test-infra-up test-infra-down test-integration

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/hermes/

install: build
	mkdir -p ~/.local/bin
	cp $(BINARY) ~/.local/bin/

run: build
	./$(BINARY)

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

test-race:
	go test ./... -race -count=1

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
	go clean

deps:
	go mod tidy
	go mod download

fmt:
	go fmt ./...

# Cross-compilation
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/hermes/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/hermes/

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/hermes/
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/hermes/

build-all: build-linux build-darwin

# ─── Quickstart (Docker) ─────────────────────────────────────────────────────

quickstart: ## One-click: build + start infra + bootstrap test tenants
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ""; \
		echo "⚠️  .env created from .env.example"; \
		echo "   Edit .env with your LLM credentials, then run 'make quickstart' again."; \
		echo ""; \
		exit 1; \
	fi
	@mkdir -p tests/fixtures
	docker compose -f docker-compose.quickstart.yml up -d --build
	@echo "⏳ Waiting for bootstrap to complete..."
	docker compose -f docker-compose.quickstart.yml wait bootstrap
	@echo ""
	@echo "✅ Quickstart ready! Run: make test-e2e"
	@echo ""

bootstrap: ## Re-run bootstrap only (tenants/souls/skills). Requires quickstart infra running.
	docker compose -f docker-compose.quickstart.yml run --rm bootstrap \
		sh -c "apk add --no-cache bash curl wget python3 ca-certificates 2>/dev/null && /scripts/bootstrap.sh"

test-e2e: ## Run Playwright isolation tests (13 tests)
	@if [ ! -f tests/fixtures/tenants.json ]; then \
		echo "❌ tests/fixtures/tenants.json not found. Run 'make quickstart' first."; \
		exit 1; \
	fi
	node_modules/.bin/playwright test --project=api-isolation

test-e2e-headed: ## Run E2E tests with browser visible
	node_modules/.bin/playwright test --project=api-isolation --headed

test-k8s: ## Run E2E tests against a remote deployment (set K8S_BASE_URL)
	BASE_URL=$${K8S_BASE_URL:-http://localhost:8080} \
		node_modules/.bin/playwright test --project=api-isolation

teardown: ## Stop and remove all quickstart containers and volumes
	docker compose -f docker-compose.quickstart.yml down -v
	@echo "✅ All quickstart resources removed."

# ─── WebUI (Docker, with UI) ─────────────────────────────────────────────────

webui: ## One-click: start infra + bootstrap + WebUI at http://localhost:3000
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo ""; \
		echo "⚠️  .env created from .env.example"; \
		echo "   Edit .env with your LLM credentials, then run 'make webui' again."; \
		echo ""; \
		exit 1; \
	fi
	@mkdir -p tests/fixtures
	docker compose -f docker-compose.webui.yml up -d --build
	@echo "⏳ Waiting for bootstrap to complete..."
	docker compose -f docker-compose.webui.yml wait bootstrap
	@echo ""
	@echo "✅ WebUI ready at http://localhost:3000"
	@echo "   API endpoint: http://localhost:8080"
	@echo "   Admin token: $${HERMES_ACP_TOKEN:-dev-bootstrap-token}"
	@echo ""

webui-teardown: ## Stop and remove webui containers and volumes
	docker compose -f docker-compose.webui.yml down -v
	@echo "✅ All webui resources removed."

# ─── Integration Tests (real PG/Redis/MinIO) ────────────────────────────────

test-infra-up: ## Start isolated test infrastructure (PG 5433, Redis 6380, MinIO 9002)
	docker compose -f docker-compose.test.yml up -d --wait
	@echo "✅ Test infrastructure ready."

test-infra-down: ## Stop and remove test infrastructure
	docker compose -f docker-compose.test.yml down -v
	@echo "✅ Test infrastructure removed."

test-integration: test-infra-up ## Run Go integration tests against real infrastructure
	go test -tags=integration -v -count=1 -timeout=300s ./tests/integration/...
	@$(MAKE) test-infra-down
