.PHONY: all build test test-coverage lint clean tools dev-setup db-setup run docker swag

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
BINDIR=bin
PROXY_BINARY=$(BINDIR)/llm-proxy
BENCHMARK_BINARY=$(BINDIR)/llm-benchmark

all: test build

build: | $(BINDIR)
	$(GOBUILD) -o $(PROXY_BINARY) ./cmd/proxy
	$(GOBUILD) -o $(BENCHMARK_BINARY) ./cmd/benchmark

$(BINDIR):
	@mkdir -p $(BINDIR)

test:
	$(GOTEST) -v -race ./...

test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

test-coverage-html: test-coverage
	go tool cover -html=coverage.out

lint:
	$(GOLINT) run ./...

clean:
	$(GOCLEAN)
	rm -f $(PROXY_BINARY)
	rm -f $(BENCHMARK_BINARY)

run: build
	./$(PROXY_BINARY)

docker:
	docker build -t llm-proxy .

# API documentation
swag:
	swag init -g cmd/proxy/main.go -o api/swagger

# Development setup
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
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