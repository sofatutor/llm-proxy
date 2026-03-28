package plugins

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

type fakeCloudWatchLogsClient struct {
	putInputs          []*cloudwatchlogs.PutLogEventsInput
	createInputs       []*cloudwatchlogs.CreateLogStreamInput
	putErr             error
	putErrAfterCreate  error
	createdLogStream   bool
}

func (f *fakeCloudWatchLogsClient) CreateLogStream(_ context.Context, input *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	f.createInputs = append(f.createInputs, input)
	f.createdLogStream = true
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

func (f *fakeCloudWatchLogsClient) PutLogEvents(_ context.Context, input *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.putInputs = append(f.putInputs, input)
	if !f.createdLogStream && f.putErr != nil {
		return nil, f.putErr
	}
	if f.createdLogStream && f.putErrAfterCreate != nil {
		return nil, f.putErrAfterCreate
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
			"request_id":    "req-1",
			"path":          "/v1/responses",
			"model":         "gpt-4.1-mini",
			"project_id":    "project-1",
			"token_id":      "token-uuid-1",
			"status":        200,
			"duration_ms":   int64(123),
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
	if decoded["total_tokens"].(float64) != 5 {
		t.Fatalf("expected total_tokens 5, got %v", decoded["total_tokens"])
	}
}

func TestCloudWatchPlugin_SendEvents_CreatesStreamOnMissing(t *testing.T) {
	client := &fakeCloudWatchLogsClient{
		putErr: &cloudwatchtypes.ResourceNotFoundException{},
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

func stringPtrCW(value string) *string {
	return &value
}