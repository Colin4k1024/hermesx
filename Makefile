BINARY=hermesx
VERSION=2.0.0
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build build-linux build-all test test-short test-race test-cover clean install run lint saas-up saas-down test-e2e test-e2e-headed test-k8s test-infra-up test-infra-down test-integration

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/hermesx/

install: build
	mkdir -p ~/.local/bin
	cp $(BINARY) ~/.local/bin/

run: build
	./$(BINARY) saas-api

test:
	go test ./... -v -count=1

test-short:
	go test ./... -short -count=1

test-race:
	go test ./... -race -count=1

test-cover: ## Run tests with coverage report
	go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -1
	@echo "HTML report: go tool cover -html=coverage.out"

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

# SaaS service release binaries
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-saas-linux-amd64 ./cmd/hermesx/
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-saas-linux-arm64 ./cmd/hermesx/

build-all: build-linux

# ─── SaaS Deployment (Docker Compose) ────────────────────────────────────────

saas-up: ## Start the SaaS API stack (API serves embedded WebUI from /static)
	docker compose -f docker-compose.saas.yml up -d --build

saas-down: ## Stop and remove the SaaS API stack
	docker compose -f docker-compose.saas.yml down

test-e2e: ## Run Playwright isolation tests (13 tests)
	@if [ ! -f tests/fixtures/tenants.json ]; then \
		echo "❌ tests/fixtures/tenants.json not found. Bootstrap a SaaS test tenant first."; \
		exit 1; \
	fi
	node_modules/.bin/playwright test --project=api-isolation

test-e2e-headed: ## Run E2E tests with browser visible
	node_modules/.bin/playwright test --project=api-isolation --headed

test-k8s: ## Run E2E tests against a remote deployment (set K8S_BASE_URL)
	BASE_URL=$${K8S_BASE_URL:-http://localhost:8080} \
		node_modules/.bin/playwright test --project=api-isolation

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
