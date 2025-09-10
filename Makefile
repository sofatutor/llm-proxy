.PHONY: all build test test-coverage test-coverage-ci test-watch test-coverage-watch test-dev lint clean tools dev-setup db-setup run docker docker-build docker-run docker-smoke docker-stop swag test-benchmark coverage

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test -parallel=8
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Test runner (prefer gotestsum if available, fallback to go test)
GOTESTSUM := $(shell command -v gotestsum 2> /dev/null || echo "$(HOME)/go/bin/gotestsum")
TEST_CMD := $(if $(shell test -x "$(GOTESTSUM)" && echo "exists"),$(GOTESTSUM) --format testname --,$(GOTEST))

# Binary names
BINDIR=bin
PROXY_BINARY=$(BINDIR)/llm-proxy
IMAGE?=llm-proxy:latest
RUN_FLAGS?=--rm
MOUNTS?=-v $(PWD)/tmp/llm-proxy-data:/app/data

all: test build

build: | $(BINDIR)
	$(GOBUILD) -o $(PROXY_BINARY) ./cmd/proxy

$(BINDIR):
	@mkdir -p $(BINDIR)

test:
	$(TEST_CMD) -v -race ./...

integration-test:
	$(TEST_CMD) -v -race -tags=integration -timeout=5m -run=Integration ./...

test-coverage:
	$(TEST_CMD) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

test-coverage-ci:
	@./scripts/run-coverage.sh ci --coverpkg ./internal/... --outfile coverage_ci.txt

test-dev:
	@./scripts/run-coverage.sh dev --coverpkg ./internal/... --outfile coverage_dev.out

test-coverage-html: test-coverage
	go tool cover -html=coverage.out

lint:
	$(GOLINT) run ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted with gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@# Guard against raw fmt/log usage in guarded runtime packages (exclude tests)
	@RAW_LOG_DIRS="internal/server internal/proxy internal/token internal/admin internal/eventtransformer internal/middleware internal/logging internal/setup internal/api internal/obfuscate internal/utils internal/database"; \
	PATTERN='\\<(fmt|log)\\.(Printf|Println|Print|Fatal|Fatalf|Panic|Panicf)\\('; \
	matches=$$(grep -R -nE "$$PATTERN" --include='*.go' --exclude='*_test.go' $$RAW_LOG_DIRS 2>/dev/null || true); \
	if [ -n "$$matches" ]; then \
		echo "Disallowed raw fmt/log calls detected in guarded directories:"; \
		echo "$$matches"; \
		echo "Replace with structured zap logging (internal/logging) and appropriate levels."; \
		exit 1; \
	fi

clean:
	$(GOCLEAN)
	rm -f $(PROXY_BINARY)

run: build
	./$(PROXY_BINARY)

docker: docker-build

docker-build:
	docker build -t llm-proxy:latest .

docker-run:
	@mkdir -p $(PWD)/tmp/llm-proxy-data
	docker run $(RUN_FLAGS) -d \
	  --name llm-proxy \
	  -p 8080:8080 \
	  $(MOUNTS) \
	  -e MANAGEMENT_TOKEN=$${MANAGEMENT_TOKEN:-dev-management-token} \
	  -e LLM_PROXY_EVENT_BUS=in-memory \
	  $(IMAGE) server

docker-stop:
	@docker rm -f llm-proxy >/dev/null 2>&1 || true
	@echo "llm-proxy container stopped"

docker-smoke:
	# Wait for container to be healthy and test /health
	@# Start container if not running
	@if [ -z "$$(docker ps -q -f name=^/llm-proxy$$ -f status=running)" ]; then \
	  echo "Starting llm-proxy container (no host mount)..."; \
	  $(MAKE) docker-run RUN_FLAGS= MOUNTS=; \
	  sleep 2; \
	fi
	@echo "Waiting for llm-proxy to become healthy..."
	@for i in `seq 1 40`; do \
	  if curl -sf http://localhost:8080/health >/dev/null; then \
	    echo "Healthcheck OK"; \
	    exit 0; \
	  fi; \
	  sleep 1; \
	done; \
	echo "Healthcheck failed"; \
	docker logs llm-proxy || true; \
	exit 1


# API documentation
swag:
	swag init -g cmd/proxy/main.go -o api/swagger

# Development setup
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/golang/mock/mockgen@v1.6.0
	go install github.com/swaggo/swag/cmd/swag@latest

dev-setup: tools
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	gofmt -s -w .

# Set up SQLite database
db-setup:
	mkdir -p ./data
	cat ./scripts/schema.sql | sqlite3 ./data/llm-proxy.db

coverage: test-coverage-html