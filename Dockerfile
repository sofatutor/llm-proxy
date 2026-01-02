# syntax=docker/dockerfile:1.7

# Use target platform for CGO (ensures native GCC under QEMU for non-amd64)
FROM --platform=$TARGETPLATFORM golang:1.23-alpine AS builder

# Build argument for PostgreSQL support (default: enabled for backward compatibility)
# Build without PostgreSQL: docker build --build-arg POSTGRES_SUPPORT=false .
# Build with PostgreSQL:    docker build --build-arg POSTGRES_SUPPORT=true .
ARG POSTGRES_SUPPORT=true

# Build argument for MySQL support (default: disabled)
# Build without MySQL: docker build --build-arg MYSQL_SUPPORT=false .
# Build with MySQL:    docker build --build-arg MYSQL_SUPPORT=true .
ARG MYSQL_SUPPORT=false

WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    GOMODCACHE=/go/pkg/mod go mod download

# Copy the rest of the source code
COPY . .

# Install build dependencies for CGO and SQLite
RUN --mount=type=cache,target=/var/cache/apk apk add gcc musl-dev sqlite-dev

# Build the application with CGO enabled for go-sqlite3
# Include postgres and/or mysql build tags based on build arguments
ENV CGO_ENABLED=1
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export GOMODCACHE=/go/pkg/mod && \
    BUILD_TAGS="" && \
    if [ "$POSTGRES_SUPPORT" = "true" ]; then \
        echo "Enabling PostgreSQL support..." && \
        BUILD_TAGS="postgres"; \
    fi && \
    if [ "$MYSQL_SUPPORT" = "true" ]; then \
        echo "Enabling MySQL support..." && \
        if [ -n "$BUILD_TAGS" ]; then \
            BUILD_TAGS="$BUILD_TAGS,mysql"; \
        else \
            BUILD_TAGS="mysql"; \
        fi; \
    fi && \
    if [ -n "$BUILD_TAGS" ]; then \
        echo "Building with tags: $BUILD_TAGS" && \
        go build -tags="$BUILD_TAGS" -ldflags "-w" -trimpath -o /llm-proxy ./cmd/proxy; \
    else \
        echo "Building without database driver tags..." && \
        go build -ldflags "-w" -trimpath -o /llm-proxy ./cmd/proxy; \
    fi

# Use a small alpine image for the final container
FROM alpine:3.18

# Security: Add only required runtime libraries
RUN --mount=type=cache,target=/var/cache/apk apk add ca-certificates tzdata sqlite-libs wget

# Security: Create non-root user and group with stable IDs (matches Helm chart defaults)
RUN addgroup -S -g 101 appgroup && adduser -S -u 100 -G appgroup appuser

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/config /app/certs && \
    chown -R appuser:appgroup /app

WORKDIR /app

RUN mkdir -p /app/bin

# Copy the binary from the builder stage
COPY --from=builder --chown=appuser:appgroup /llm-proxy /app/bin/llm-proxy

# Security: Ensure the binary is executable even when Kubernetes assigns a different non-root UID
RUN chmod 555 /app/bin/llm-proxy && \
    chmod -R 750 /app/data /app/logs /app/config /app/certs

# Copy entrypoint script
COPY --chown=appuser:appgroup entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Copy web templates
COPY --chown=appuser:appgroup web/templates /app/web/templates

# Copy web assets
COPY --chown=appuser:appgroup web/static /app/web/static

# Copy database migrations (required for migration runner)
# Use absolute path from builder stage (WORKDIR is /app)
COPY --from=builder --chown=appuser:appgroup /app/internal/database/migrations /app/internal/database/migrations

# Copy SQLite schema (required for runtime SQLite initialization, used by docker-smoke)
RUN mkdir -p /app/scripts
COPY --from=builder --chown=appuser:appgroup /app/scripts/schema.sql /app/scripts/schema.sql

# Define volumes for data persistence
VOLUME ["/app/data", "/app/logs", "/app/config", "/app/certs"]

# Expose the server port
EXPOSE 8080

# Security: Use non-root user
USER appuser:appgroup

# Set Docker labels for documentation
LABEL org.opencontainers.image.title="LLM Proxy" \
      org.opencontainers.image.description="Transparent LLM Proxy for OpenAI" \
      org.opencontainers.image.vendor="sofatutor" \
      org.opencontainers.image.source="https://github.com/sofatutor/llm-proxy" \
      org.opencontainers.image.documentation="https://github.com/sofatutor/llm-proxy/tree/main/docs" \
      org.opencontainers.image.licenses="MIT" \
      com.docker.security.policy="AppArmor=restricted seccomp=restricted NoNewPrivileges=true"

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV LISTEN_ADDR=:8080 \
    DATABASE_PATH=/app/data/llm-proxy.db \
    PATH=/app/bin:$PATH \
    ADMIN_UI_API_BASE_URL=http://localhost:8080

# Run the application
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["server"]
