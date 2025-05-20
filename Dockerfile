FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# Security: Build with hardening flags
RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -ldflags "-w -extldflags '-static'" \
    -trimpath -o /llm-proxy ./cmd/proxy

# Use a small alpine image for the final container
FROM alpine:3.18

# Security: Update packages and add CA certificates
RUN apk update && \
    apk upgrade && \
    apk --no-cache add ca-certificates tzdata && \
    rm -rf /var/cache/apk/*

# Security: Create non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/data /app/logs /app/config /app/certs && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder --chown=appuser:appgroup /llm-proxy /app/llm-proxy

# Security: Set restrictive permissions
RUN chmod 550 /app/llm-proxy && \
    chmod -R 750 /app/data /app/logs /app/config /app/certs

# Create default config
COPY --chown=appuser:appgroup .env.example /app/config/.env.example

# Define volumes for data persistence
VOLUME ["/data", "/logs", "/config"]

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

# Create volumes for persistent data
VOLUME ["/app/data", "/app/logs", "/app/config", "/app/certs"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set environment variables
ENV LISTEN_ADDR=:8080 \
    DATABASE_PATH=/app/data/llm-proxy.db \
    LOG_FILE=/app/logs/llm-proxy.log

# Run the application
CMD ["/app/llm-proxy"]