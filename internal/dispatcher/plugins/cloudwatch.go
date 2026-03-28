package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

var errCloudWatchRetriesExhausted = errors.New("cloudwatch put log events retries exhausted")

type cloudWatchLogsClient interface {
	CreateLogStream(context.Context, *cloudwatchlogs.CreateLogStreamInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	PutLogEvents(context.Context, *cloudwatchlogs.PutLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

// CloudWatchPlugin writes sanitized observability records to CloudWatch Logs.
type CloudWatchPlugin struct {
	client    cloudWatchLogsClient
	logGroup  string
	logStream string
	region    string

	mu            sync.Mutex
	sequenceToken *string
}

// NewCloudWatchPlugin creates a CloudWatch plugin.
func NewCloudWatchPlugin() *CloudWatchPlugin {
	return &CloudWatchPlugin{}
}

// Init initializes the plugin with CloudWatch configuration.
func (p *CloudWatchPlugin) Init(cfg map[string]string) error {
	p.logGroup = firstNonEmpty(cfg["log-group"], os.Getenv("DISPATCHER_CLOUDWATCH_LOG_GROUP"))
	p.logStream = firstNonEmpty(cfg["log-stream"], os.Getenv("DISPATCHER_CLOUDWATCH_LOG_STREAM"))
	p.region = firstNonEmpty(cfg["region"], os.Getenv("DISPATCHER_CLOUDWATCH_REGION"), os.Getenv("AWS_REGION"), os.Getenv("AWS_DEFAULT_REGION"))

	if p.logGroup == "" {
		return fmt.Errorf("cloudwatch plugin requires 'log-group' configuration")
	}
	if p.logStream == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
		p.logStream = fmt.Sprintf("llm-proxy-dispatcher-%s-%d", hostname, os.Getpid())
	}

	if p.client != nil {
		return nil
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{}
	if p.region != "" {
		loadOptions = append(loadOptions, awsconfig.WithRegion(p.region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	p.client = cloudwatchlogs.NewFromConfig(awsCfg)
	return nil
}

// SendEvents writes a batch of sanitized events to CloudWatch Logs.
func (p *CloudWatchPlugin) SendEvents(ctx context.Context, events []dispatcher.EventPayload) error {
	if len(events) == 0 {
		return nil
	}
	if p.client == nil {
		return fmt.Errorf("cloudwatch plugin not initialized")
	}

	sortedEvents := append([]dispatcher.EventPayload(nil), events...)
	sort.SliceStable(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp.Before(sortedEvents[j].Timestamp)
	})

	logEvents := make([]cloudwatchtypes.InputLogEvent, 0, len(sortedEvents))
	for _, event := range sortedEvents {
		message, err := cloudWatchLogMessage(event)
		if err != nil {
			return err
		}
		logEvents = append(logEvents, cloudwatchtypes.InputLogEvent{
			Message:   &message,
			Timestamp: awsInt64(event.Timestamp.UnixMilli()),
		})
	}

	for attempt := 0; attempt < 3; attempt++ {
		output, err := p.client.PutLogEvents(ctx, p.newPutLogEventsInput(logEvents))
		if err == nil {
			p.setSequenceToken(output.NextSequenceToken)
			return nil
		}

		var notFound *cloudwatchtypes.ResourceNotFoundException
		if errorAs(err, &notFound) {
			if createErr := p.ensureLogStream(ctx); createErr != nil {
				return createErr
			}
			p.setSequenceToken(nil)
			continue
		}

		var invalidSequence *cloudwatchtypes.InvalidSequenceTokenException
		if errorAs(err, &invalidSequence) {
			p.setSequenceToken(invalidSequence.ExpectedSequenceToken)
			continue
		}

		var alreadyAccepted *cloudwatchtypes.DataAlreadyAcceptedException
		if errorAs(err, &alreadyAccepted) {
			p.setSequenceToken(alreadyAccepted.ExpectedSequenceToken)
			return nil
		}

		return err
	}

	return fmt.Errorf("put cloudwatch log events: %w", errCloudWatchRetriesExhausted)
}

// Close cleans up plugin resources.
func (p *CloudWatchPlugin) Close() error {
	return nil
}

func (p *CloudWatchPlugin) ensureLogStream(ctx context.Context) error {
	_, createErr := p.client.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &p.logGroup,
		LogStreamName: &p.logStream,
	})
	if createErr == nil {
		return nil
	}

	var alreadyExists *cloudwatchtypes.ResourceAlreadyExistsException
	if errorAs(createErr, &alreadyExists) {
		return nil
	}

	return createErr
}

func (p *CloudWatchPlugin) newPutLogEventsInput(logEvents []cloudwatchtypes.InputLogEvent) *cloudwatchlogs.PutLogEventsInput {
	input := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     logEvents,
		LogGroupName:  &p.logGroup,
		LogStreamName: &p.logStream,
	}

	if sequenceToken := p.getSequenceToken(); sequenceToken != nil {
		input.SequenceToken = sequenceToken
	}

	return input
}

func (p *CloudWatchPlugin) getSequenceToken() *string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sequenceToken == nil {
		return nil
	}

	value := *p.sequenceToken
	return &value
}

func (p *CloudWatchPlugin) setSequenceToken(sequenceToken *string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sequenceToken == nil || *sequenceToken == "" {
		p.sequenceToken = nil
		return
	}

	value := *sequenceToken
	p.sequenceToken = &value
}

func cloudWatchLogMessage(event dispatcher.EventPayload) (string, error) {
	message := map[string]any{
		"type":        "llm_proxy_usage",
		"timestamp":   event.Timestamp.UTC().Format(time.RFC3339Nano),
		"run_id":      event.RunID,
		"event":       event.Event,
		"request_id":  metadataString(event.Metadata, "request_id"),
		"path":        metadataString(event.Metadata, "path"),
		"model":       metadataString(event.Metadata, "model"),
		"project_id":  metadataString(event.Metadata, "project_id"),
		"token_id":    metadataString(event.Metadata, "token_id"),
		"status":      metadataInt(event.Metadata, "status"),
		"duration_ms": metadataInt64(event.Metadata, "duration_ms"),
	}

	if event.UserID != nil && *event.UserID != "" {
		message["user_id"] = *event.UserID
	}
	if event.TokensUsage != nil {
		message["prompt_tokens"] = event.TokensUsage.Prompt
		message["completion_tokens"] = event.TokensUsage.Completion
		message["total_tokens"] = event.TokensUsage.Prompt + event.TokensUsage.Completion
	}
	if tokenMetadata, ok := event.Metadata["token_metadata"].(map[string]string); ok && len(tokenMetadata) > 0 {
		message["token_metadata"] = tokenMetadata
	}

	encoded, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("marshal cloudwatch event: %w", err)
	}

	return string(encoded), nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return value
	}
	return ""
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func metadataInt64(metadata map[string]any, key string) int64 {
	return int64(metadataInt(metadata, key))
}

func awsInt64(value int64) *int64 {
	return &value
}

func errorAs(err error, target interface{}) bool {
	return errors.As(err, target)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
