FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /llm-proxy ./cmd/proxy

# Use a small alpine image for the final container
FROM alpine:3.18

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user and directories for data, logs, and config
RUN addgroup -S appgroup && \
    adduser -S -G appgroup appuser && \
    mkdir -p /data /logs /config && \
    chown -R appuser:appgroup /data /logs /config

# Set working directory and switch to non-root user
WORKDIR /app
USER appuser

# Copy the binary from the builder stage
COPY --from=builder /llm-proxy .

# Define volumes for data persistence
VOLUME ["/data", "/logs", "/config"]

# Expose the server port
EXPOSE 8080

# Run the application
CMD ["./llm-proxy", "--config-dir", "/config", "--data-dir", "/data", "--log-dir", "/logs"]