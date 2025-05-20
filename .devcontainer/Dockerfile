FROM mcr.microsoft.com/vscode/devcontainers/go:1-1.21-bullseye

# Install additional tools
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install sqlite3 \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/*

# Install Go tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2 \
    && go install golang.org/x/tools/cmd/godoc@latest \
    && go install github.com/golang/mock/mockgen@v1.6.0 \
    && go install github.com/swaggo/swag/cmd/swag@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest 