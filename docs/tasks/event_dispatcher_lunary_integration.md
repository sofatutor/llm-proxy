# Event Dispatcher Service (Pluggable CLI) — Lunary Integration

Implements docs/issues/phase-4-event-dispatcher-service.md

## 1. Preparation & Discovery
- **Review Lunary Ingest API**  
  - Endpoint: `POST https://api.lunary.ai/v1/runs/ingest`  
  - Auth: Bearer token  
- **Define config flags / env vars**  
  - `--service` (e.g. `lunary`, `helicone`, `file`, etc.)  
  - `--api-key` / `LLM_PROXY_API_KEY` (per selected service)  
  - `--endpoint` / `LLM_PROXY_ENDPOINT` (default per service)  

## 2. Expand Go Payload Types
\`\`\`go
type EventPayload struct {
  Type        string          \`json:"type"\`
  Event       string          \`json:"event"\`
  RunID       string          \`json:"runId"\`
  ParentRunID *string         \`json:"parentRunId,omitempty"\`
  Name        *string         \`json:"name,omitempty"\`
  Timestamp   time.Time       \`json:"timestamp"\`
  Input       json.RawMessage \`json:"input,omitempty"\`
  Output      json.RawMessage \`json:"output,omitempty"\`

  // Additional fields:
  UserID      *string          \`json:"userId,omitempty"\`
  TokensUsage *TokensUsage     \`json:"tokensUsage,omitempty"\`
  UserProps   map[string]any   \`json:"userProps,omitempty"\`
  Extra       map[string]any   \`json:"extra,omitempty"\`
  Metadata    map[string]any   \`json:"metadata,omitempty"\`
  Tags        []string         \`json:"tags,omitempty"\`
}

type TokensUsage struct {
  Completion int \`json:"completion"\`
  Prompt     int \`json:"prompt"\`
}
\`\`\`

## 3. Token Counting with Go-tiktoken
- **Import**  
  \`\`\`go
  import tiktoken "github.com/pkoukk/tiktoken-go"
  \`\`\`
- **Helper**  
  \`\`\`go
  func countTokens(model string, msgs []map[string]string) (int, error) {
    enc, err := tiktoken.GetEncoding(model)
    if err != nil { return 0, err }
    total := 0
    for _, m := range msgs {
      total += len(enc.Encode(m["content"], nil, nil))
    }
    return total, nil
  }
  \`\`\`
- **Populate**  
  ```go
  var inMsgs, outMsgs []map[string]string
  json.Unmarshal(evt.Input, &inMsgs)
  json.Unmarshal(evt.Output, &outMsgs)
  pt, _ := countTokens(*evt.Name, inMsgs)
  ct, _ := countTokens(*evt.Name, outMsgs)
  evt.TokensUsage = &TokensUsage{Prompt: pt, Completion: ct}
  ```

## 4. HTTP Client & Request Builder
- **Client factory**  
  ```go
  func NewLunaryClient(apiKey, endpoint string) *LunaryClient { … }
  ```
- **Request**  
  ```go
  func (c *LunaryClient) buildRequest(ctx, events) (*http.Request, error) { … }
  ```
- **Send**  
  - Marshal all fields (including maps and slices)  
  - Set headers: `Content-Type: application/json`, `Authorization: Bearer <apiKey>`  

## 5. Plugin Interface & `lunary.go`
\`\`\`go
type BackendPlugin interface {
  Init(cfg map[string]string) error
  SendEvents(ctx context.Context, events []EventPayload) error
}
\`\`\`
- **Init**: read `apiKey`, `endpoint` from `cfg`  
- **SendEvents**: count tokens, marshal, call `LunaryClient.Send`  

## 6. CLI Integration & Config
- **Command**: `llm-proxy dispatcher`  
- **Flags**:
  ```bash
  llm-proxy dispatcher     --service lunary     --api-key $LUNARY_API_KEY     --endpoint https://api.lunary.ai/v1/runs/ingest     --detach
  ```
- **Service dispatch**:
  ```go
  cmd.Flags().String("service", "", "Dispatcher service to use")
  cmd.Flags().String("api-key", "", "API key for the selected service")
  cmd.Flags().String("endpoint", "", "Endpoint URL for the selected service")
  cmd.Flags().Bool("detach", false, "Run in background")

  // ...
  switch service {
  case "lunary":
    plugin = plugins.NewLunaryPlugin()
  case "helicone":
    plugin = plugins.NewHeliconePlugin()
  default:
    return errors.Errorf("unknown service: %s", service)
  }
  plugin.Init(cfg)
  dispatcher.SetPlugin(plugin)
  dispatcher.Run(ctx, viper.GetBool("detach"))
  ```

## 7. Retry Logic, Metrics & Logging
- **Retry**: exponential backoff on 5xx + network errors  
- **Logging**: include `runId`, attempt, HTTP status, `promptTokens`, `completionTokens`  
- **Metrics**:  
  - Counter: events sent, failures  
  - Histogram: request latency  
  - Gauge: in-flight requests  

## 8. Testing
- **Unit**  
  - `countTokens` against known inputs  
  - JSON marshaling includes all fields  
- **Integration** (`httptest.Server`)  
  - Simulate 200, 500 responses; verify retries  
- **Plugin**  
  - Fail on missing config  
  - Populate `TokensUsage` correctly  

## 9. Docs & Examples
- **Usage snippet** in README:
  ```markdown
  ## Lunary Dispatcher
  \`\`\`bash
  llm-proxy dispatcher     --service lunary     --api-key $LLM_PROXY_API_KEY     --endpoint https://api.lunary.ai/v1/runs/ingest     --detach
  \`\`\`
  - **Env vars**: `LLM_PROXY_API_KEY`, `LLM_PROXY_ENDPOINT`  
  - **Events**: how to emit start/end with `userProps`, `extra`, `metadata`, `tags`  
  ```
- **Code sample** showing an LLM start event as detailed above.
