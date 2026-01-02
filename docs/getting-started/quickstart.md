---
title: Quickstart
parent: Getting Started
nav_order: 2
---

# Quickstart

1) Build CLI
```bash
go build -o llm-proxy ./cmd/proxy
```

Or build with database support:
```bash
# With PostgreSQL support
go build -tags postgres -o llm-proxy ./cmd/proxy

# With MySQL support
go build -tags mysql -o llm-proxy ./cmd/proxy

# With both PostgreSQL and MySQL support
go build -tags "postgres,mysql" -o llm-proxy ./cmd/proxy
```

2) Setup
```bash
./llm-proxy setup --interactive
```

3) Start services
```bash
./llm-proxy server
./llm-proxy admin --management-token $MANAGEMENT_TOKEN
```

3b) Start with Redis caching (optional)
```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:alpine

# Start proxy with caching
HTTP_CACHE_ENABLED=true HTTP_CACHE_BACKEND=redis REDIS_ADDR=localhost:6379 ./llm-proxy server
```

3c) Start with PostgreSQL (optional)
```bash
# Start PostgreSQL with Docker Compose
docker compose --profile postgres up -d postgres

# Start proxy with PostgreSQL (requires -tags postgres build)
DB_DRIVER=postgres DATABASE_URL=postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=disable ./llm-proxy server
```

3d) Start with MySQL (optional)
```bash
# Start MySQL with Docker Compose
docker compose --profile mysql up -d mysql

# Start proxy with MySQL (requires -tags mysql build)
DB_DRIVER=mysql DATABASE_URL=llmproxy:secret@tcp(localhost:3306)/llmproxy?parseTime=true ./llm-proxy server
```

4) Generate a token
   - In the Admin UI: Tokens → Generate Token (recommended)
   - Or via CLI (replace IDs):
```bash
llm-proxy manage token generate --project-id <project-id> --management-token $MANAGEMENT_TOKEN --duration 1440
```

5) Test chat via proxy (interactive)
```bash
./llm-proxy openai chat --token $PROXY_TOKEN --model gpt-4.1-mini
```

6) Test API via curl
```bash
curl -sS -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $PROXY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1-mini",
    "messages": [
      {"role":"user","content":"Hello!"}
    ]
  }'
```

See [CLI reference](./cli-reference.md) and [Architecture](./architecture.md).


