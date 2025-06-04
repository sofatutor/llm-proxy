package eventtransformer

// Package eventtransformer provides event transformation logic for different LLM API providers.
// Each provider (e.g., OpenAI, Anthropic) should have its own transformer implementation.

// Transformer is the interface for provider-specific event transformers.
type Transformer interface {
	TransformEvent(event map[string]interface{}) (map[string]interface{}, error)
}

// DispatchTransformer returns the appropriate transformer for a given provider.
func DispatchTransformer(provider string) Transformer {
	switch provider {
	case "openai":
		return &OpenAITransformer{}
	// case "anthropic":
	// 	return &AnthropicTransformer{}
	default:
		return nil
	}
}

// OpenAITransformer implements Transformer for OpenAI events.
type OpenAITransformer struct{}
