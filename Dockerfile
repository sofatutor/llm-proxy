FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Install build dependencies for CGO and SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Build the application with CGO enabled for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux \
    go build -a -ldflags "-w" \
    -trimpath -o /llm-proxy ./cmd/proxy

# Use a small alpine image for the final container
FROM alpine:3.18

# Security: Add only required runtime libraries
RUN apk --no-cache add ca-certificates tzdata sqlite-libs wget

# Security: Create non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/config /app/certs && \
    chown -R appuser:appgroup /app

WORKDIR /app

RUN mkdir -p /app/bin

# Copy the binary from the builder stage
COPY --from=builder --chown=appuser:appgroup /llm-proxy /app/bin/llm-proxy

# Security: Set restrictive permissions
RUN chmod 550 /app/bin/llm-proxy && \
    chmod -R 750 /app/data /app/logs /app/config /app/certs

# Copy entrypoint script
COPY --chown=appuser:appgroup entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# Copy web templates
COPY --chown=appuser:appgroup web/templates /app/web/templates

# Copy web assets
COPY --chown=appuser:appgroup web/static /app/web/static

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
