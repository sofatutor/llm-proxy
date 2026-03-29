package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

type fakeCloudWatchLogsClient struct {
	putInputs        []*cloudwatchlogs.PutLogEventsInput
	createInputs     []*cloudwatchlogs.CreateLogStreamInput
	putResponses     []fakePutLogEventsResponse
	createErr        error
	createdLogStream bool
}

type fakePutLogEventsResponse struct {
	output *cloudwatchlogs.PutLogEventsOutput
	err    error
}

func (f *fakeCloudWatchLogsClient) CreateLogStream(_ context.Context, input *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	f.createInputs = append(f.createInputs, input)
	f.createdLogStream = true
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

func (f *fakeCloudWatchLogsClient) PutLogEvents(_ context.Context, input *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.putInputs = append(f.putInputs, input)
	if len(f.putResponses) > 0 {
		response := f.putResponses[0]
		f.putResponses = f.putResponses[1:]
		if response.err != nil {
			return nil, response.err
		}
		if response.output != nil {
			return response.output, nil
		}
	}
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func TestCloudWatchLogMessage_SanitizesPayload(t *testing.T) {
	message, err := cloudWatchLogMessage(dispatcher.EventPayload{
		RunID:     "run-1",
		Event:     "start",
		Timestamp: time.Unix(1700000000, 0),
		UserID:    stringPtrCW("42"),
		TokensUsage: &dispatcher.TokensUsage{
			Prompt:     3,
			Completion: 2,
		},
		Input:  json.RawMessage(`{"sensitive":"input"}`),
		Output: json.RawMessage(`{"sensitive":"output"}`),
		Metadata: map[string]any{
			"request_id":     "req-1",
			"path":           "/v1/responses",
			"model":          "gpt-4.1-mini",
			"project_id":     "project-1",
			"token_id":       "token-uuid-1",
			"status":         200,
			"duration_ms":    int64(123),
			"token_metadata": map[string]string{"feature": "sofabuddy", "user_id": "42"},
		},
	})
	if err != nil {
		t.Fatalf("cloudWatchLogMessage() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(message), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, exists := decoded["input"]; exists {
		t.Fatalf("expected sanitized payload to omit input")
	}
	if _, exists := decoded["output"]; exists {
		t.Fatalf("expected sanitized payload to omit output")
	}
	if decoded["user_id"] != "42" {
		t.Fatalf("expected user_id 42, got %v", decoded["user_id"])
	}
	if decoded["duration_ms"].(float64) != 123 {
		t.Fatalf("expected duration_ms 123, got %v", decoded["duration_ms"])
	}
	if decoded["total_tokens"].(float64) != 5 {
		t.Fatalf("expected total_tokens 5, got %v", decoded["total_tokens"])
	}
}

func TestCloudWatchPlugin_SendEvents_CreatesStreamOnMissing(t *testing.T) {
	client := &fakeCloudWatchLogsClient{
		putResponses: []fakePutLogEventsResponse{
			{err: &cloudwatchtypes.ResourceNotFoundException{}},
			{output: &cloudwatchlogs.PutLogEventsOutput{}},
		},
	}
	plugin := NewCloudWatchPlugin()
	plugin.client = client
	if err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "log-stream": "dispatcher-1", "region": "eu-central-1"}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{{
		RunID:     "run-1",
		Event:     "start",
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"request_id": "req-1",
			"path":       "/v1/responses",
			"status":     200,
		},
	}})
	if err != nil {
		t.Fatalf("SendEvents() error = %v", err)
	}
	if len(client.createInputs) != 1 {
		t.Fatalf("expected CreateLogStream to be called once, got %d", len(client.createInputs))
	}
	if len(client.putInputs) != 2 {
		t.Fatalf("expected PutLogEvents to be called twice, got %d", len(client.putInputs))
	}
}

func TestCloudWatchPlugin_SendEvents_SortsByTimestamp(t *testing.T) {
	client := &fakeCloudWatchLogsClient{}
	plugin := NewCloudWatchPlugin()
	plugin.client = client
	if err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "log-stream": "dispatcher-1", "region": "eu-central-1"}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	later := time.Unix(1700000010, 0)
	earlier := later.Add(-time.Minute)
	err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{
		{RunID: "run-late", Event: "finish", Timestamp: later, Metadata: map[string]any{"status": 200}},
		{RunID: "run-early", Event: "start", Timestamp: earlier, Metadata: map[string]any{"status": 200}},
	})
	if err != nil {
		t.Fatalf("SendEvents() error = %v", err)
	}
	if len(client.putInputs) != 1 {
		t.Fatalf("expected one PutLogEvents call, got %d", len(client.putInputs))
	}
	if len(client.putInputs[0].LogEvents) != 2 {
		t.Fatalf("expected two log events, got %d", len(client.putInputs[0].LogEvents))
	}
	if *client.putInputs[0].LogEvents[0].Timestamp > *client.putInputs[0].LogEvents[1].Timestamp {
		t.Fatalf("expected log events to be sorted chronologically")
	}
}

func TestCloudWatchPlugin_SendEvents_RetriesWithExpectedSequenceToken(t *testing.T) {
	expectedToken := "token-2"
	nextToken := "token-3"
	client := &fakeCloudWatchLogsClient{
		putResponses: []fakePutLogEventsResponse{
			{err: &cloudwatchtypes.InvalidSequenceTokenException{ExpectedSequenceToken: &expectedToken}},
			{output: &cloudwatchlogs.PutLogEventsOutput{NextSequenceToken: &nextToken}},
		},
	}
	plugin := NewCloudWatchPlugin()
	plugin.client = client
	plugin.setSequenceToken(stringPtrCW("token-1"))
	if err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "log-stream": "dispatcher-1", "region": "eu-central-1"}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{{
		RunID:     "run-1",
		Event:     "start",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"status": 200},
	}})
	if err != nil {
		t.Fatalf("SendEvents() error = %v", err)
	}
	if len(client.putInputs) != 2 {
		t.Fatalf("expected PutLogEvents to be retried once, got %d calls", len(client.putInputs))
	}
	if client.putInputs[0].SequenceToken == nil || *client.putInputs[0].SequenceToken != "token-1" {
		t.Fatalf("expected first request to use existing sequence token, got %v", client.putInputs[0].SequenceToken)
	}
	if client.putInputs[1].SequenceToken == nil || *client.putInputs[1].SequenceToken != expectedToken {
		t.Fatalf("expected retry to use updated sequence token, got %v", client.putInputs[1].SequenceToken)
	}
	if token := plugin.getSequenceToken(); token == nil || *token != nextToken {
		t.Fatalf("expected plugin to persist next sequence token %q, got %v", nextToken, token)
	}
}

func TestCloudWatchPlugin_SendEvents_DataAlreadyAcceptedIsTreatedAsSuccess(t *testing.T) {
	expectedToken := "token-4"
	client := &fakeCloudWatchLogsClient{
		putResponses: []fakePutLogEventsResponse{{
			err: &cloudwatchtypes.DataAlreadyAcceptedException{ExpectedSequenceToken: &expectedToken},
		}},
	}
	plugin := NewCloudWatchPlugin()
	plugin.client = client
	if err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "log-stream": "dispatcher-1", "region": "eu-central-1"}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{{
		RunID:     "run-1",
		Event:     "start",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"status": 200},
	}})
	if err != nil {
		t.Fatalf("SendEvents() error = %v", err)
	}
	if token := plugin.getSequenceToken(); token == nil || *token != expectedToken {
		t.Fatalf("expected plugin to persist expected sequence token %q, got %v", expectedToken, token)
	}
}

func TestCloudWatchPlugin_SendEvents_ExhaustedSequenceRetries(t *testing.T) {
	expectedToken := "still-wrong"
	client := &fakeCloudWatchLogsClient{
		putResponses: []fakePutLogEventsResponse{
			{err: &cloudwatchtypes.InvalidSequenceTokenException{ExpectedSequenceToken: &expectedToken}},
			{err: &cloudwatchtypes.InvalidSequenceTokenException{ExpectedSequenceToken: &expectedToken}},
			{err: &cloudwatchtypes.InvalidSequenceTokenException{ExpectedSequenceToken: &expectedToken}},
		},
	}
	plugin := NewCloudWatchPlugin()
	plugin.client = client
	if err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "log-stream": "dispatcher-1", "region": "eu-central-1"}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{{
		RunID:     "run-1",
		Event:     "start",
		Timestamp: time.Now(),
		Metadata:  map[string]any{"status": 200},
	}})
	if err == nil || !errors.Is(err, errCloudWatchRetriesExhausted) {
		t.Fatalf("SendEvents() error = %v, want errCloudWatchRetriesExhausted", err)
	}
}

func TestCloudWatchPlugin_Init_RequiresLogGroup(t *testing.T) {
	plugin := NewCloudWatchPlugin()
	plugin.client = &fakeCloudWatchLogsClient{}

	err := plugin.Init(map[string]string{"log-stream": "dispatcher-1"})
	if err == nil || !strings.Contains(err.Error(), "log-group") {
		t.Fatalf("Init() error = %v, want missing log-group error", err)
	}
}

func TestCloudWatchPlugin_Init_DefaultsLogStreamWhenMissing(t *testing.T) {
	plugin := NewCloudWatchPlugin()
	plugin.client = &fakeCloudWatchLogsClient{}

	err := plugin.Init(map[string]string{"log-group": "/llm-proxy", "region": "eu-central-1"})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if plugin.logStream == "" || !strings.Contains(plugin.logStream, "llm-proxy-dispatcher-") {
		t.Fatalf("expected generated log stream, got %q", plugin.logStream)
	}
}

func TestCloudWatchPlugin_EnsureLogStream_AlreadyExists(t *testing.T) {
	plugin := NewCloudWatchPlugin()
	plugin.client = &fakeCloudWatchLogsClient{createErr: &cloudwatchtypes.ResourceAlreadyExistsException{}}
	plugin.logGroup = "/llm-proxy"
	plugin.logStream = "dispatcher-1"

	if err := plugin.ensureLogStream(context.Background()); err != nil {
		t.Fatalf("ensureLogStream() error = %v", err)
	}
}

func TestCloudWatchPlugin_EnsureLogStream_PropagatesCreateError(t *testing.T) {
	plugin := NewCloudWatchPlugin()
	plugin.client = &fakeCloudWatchLogsClient{createErr: errors.New("boom")}
	plugin.logGroup = "/llm-proxy"
	plugin.logStream = "dispatcher-1"

	err := plugin.ensureLogStream(context.Background())
	if err == nil || err.Error() != "boom" {
		t.Fatalf("ensureLogStream() error = %v, want boom", err)
	}
}

func TestCloudWatchPlugin_Close_NoOp(t *testing.T) {
	plugin := NewCloudWatchPlugin()
	if err := plugin.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCloudWatchMetadataHelpers(t *testing.T) {
	metadata := map[string]any{
		"as-string": "value",
		"as-int":    12,
		"as-float":  float64(34),
		"as-text":   "56",
		"invalid":   true,
	}

	if got := metadataString(metadata, "as-string"); got != "value" {
		t.Fatalf("metadataString() = %q, want value", got)
	}
	if got := metadataString(metadata, "missing"); got != "" {
		t.Fatalf("metadataString() missing = %q, want empty", got)
	}
	if got := metadataInt(metadata, "as-int"); got != 12 {
		t.Fatalf("metadataInt() int = %d, want 12", got)
	}
	if got := metadataInt(metadata, "as-float"); got != 34 {
		t.Fatalf("metadataInt() float = %d, want 34", got)
	}
	if got := metadataInt(metadata, "as-text"); got != 56 {
		t.Fatalf("metadataInt() string = %d, want 56", got)
	}
	if got := metadataInt(metadata, "invalid"); got != 0 {
		t.Fatalf("metadataInt() invalid = %d, want 0", got)
	}
	if got := metadataInt(nil, "as-int"); got != 0 {
		t.Fatalf("metadataInt() nil metadata = %d, want 0", got)
	}
	if got := firstNonEmpty("", "fallback", "last"); got != "fallback" {
		t.Fatalf("firstNonEmpty() = %q, want fallback", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty() empty = %q, want empty", got)
	}
}

func stringPtrCW(value string) *string {
	return &value
}
