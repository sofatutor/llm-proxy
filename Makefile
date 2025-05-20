.PHONY: all build test lint clean tools dev-setup db-setup run docker

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
PROXY_BINARY=llm-proxy
BENCHMARK_BINARY=llm-benchmark

all: test build

build:
	$(GOBUILD) -o $(PROXY_BINARY) ./cmd/proxy
	$(GOBUILD) -o $(BENCHMARK_BINARY) ./cmd/benchmark

test:
	$(GOTEST) -v -race ./...

lint:
	$(GOLINT) run ./...

clean:
	$(GOCLEAN)
	rm -f $(PROXY_BINARY)
	rm -f $(BENCHMARK_BINARY)

run:
	$(GOBUILD) -o $(PROXY_BINARY) ./cmd/proxy
	./$(PROXY_BINARY)

docker:
	docker build -t llm-proxy .

# Development setup
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/golang/mock/mockgen@v1.6.0
	go install github.com/swaggo/swag/cmd/swag@latest

dev-setup: tools
	$(GOMOD) download
	$(GOMOD) tidy

# Set up SQLite database
db-setup:
	mkdir -p ./data
	cat ./scripts/schema.sql | sqlite3 ./data/llm-proxy.db 