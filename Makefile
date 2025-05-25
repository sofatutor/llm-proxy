.PHONY: all build test test-coverage lint clean tools dev-setup db-setup run docker swag test-benchmark coverage

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test -parallel=8
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Binary names
BINDIR=bin
PROXY_BINARY=$(BINDIR)/llm-proxy

all: test build

build: | $(BINDIR)
	$(GOBUILD) -o $(PROXY_BINARY) ./cmd/proxy

$(BINDIR):
	@mkdir -p $(BINDIR)

test:
	$(GOTEST) -v -race ./...

integration-test:
	$(GOTEST) -v -race -tags=integration -timeout=5m -run=Integration ./...

test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

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

clean:
	$(GOCLEAN)
	rm -f $(PROXY_BINARY)

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

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html 