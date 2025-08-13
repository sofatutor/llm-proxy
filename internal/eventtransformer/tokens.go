package eventtransformer

import (
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// CountOpenAITokens counts tokens using a general-purpose encoding.
// Note: Prefer CountOpenAITokensForModel when the model is known.
func CountOpenAITokens(text string) (int, error) {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, err
	}
	return len(enc.Encode(text, nil, nil)), nil
}

// CountOpenAITokensForModel selects an encoding based on the provided model name.
// Fallback rules:
// - 4o/omni/o1 family → o200k_base
// - otherwise → cl100k_base
// If EncodingForModel succeeds, it is used directly.
func CountOpenAITokensForModel(text, model string) (int, error) {
	if model != "" {
		if enc, err := tiktoken.EncodingForModel(model); err == nil {
			return len(enc.Encode(text, nil, nil)), nil
		}
	}
	// Heuristic fallback by family
	var base string
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "gpt-4o"), strings.HasPrefix(lower, "o1"), strings.Contains(lower, "omni"):
		base = "o200k_base"
	default:
		base = "cl100k_base"
	}
	enc, err := tiktoken.GetEncoding(base)
	if err != nil {
		// Last resort: try cl100k_base
		enc, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return 0, err
		}
	}
	return len(enc.Encode(text, nil, nil)), nil
}
