[Home](./index.md) | [Features](./features.md) | [Screenshots](./screenshots.md) | [Quickstart](./quickstart.md) | [CLI Reference](./cli-reference.md) | [Architecture](./architecture.md) | [Contributing](./contributing.md) | [Coverage](./coverage/) | [Roadmap](../PLAN.md)

## Quickstart

1) Build CLI
```bash
go build -o llm-proxy ./cmd/proxy
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

4) Test chat via proxy
```bash
./llm-proxy openai chat --token $PROXY_TOKEN --model gpt-4.1-mini
```

See [CLI reference](./cli-reference.md) and [Architecture](./architecture.md).


