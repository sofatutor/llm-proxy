[Home]({{ site.baseurl }}/) | [Features]({{ site.baseurl }}/features) | [Screenshots]({{ site.baseurl }}/screenshots) | [Quickstart]({{ site.baseurl }}/quickstart) | [CLI Reference]({{ site.baseurl }}/cli-reference) | [Architecture]({{ site.baseurl }}/architecture) | [Contributing]({{ site.baseurl }}/contributing) | [Coverage]({{ site.baseurl }}/coverage/) | [Roadmap](https://github.com/sofatutor/llm-proxy/blob/main/PLAN.md)

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


