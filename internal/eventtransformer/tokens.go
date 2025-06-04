package eventtransformer

import (
	"github.com/pkoukk/tiktoken-go"
)

// CountOpenAITokens uses tiktoken-go to count tokens in a response string
func CountOpenAITokens(resp string) (int, error) {
	tk, err := tiktoken.EncodingForModel("gpt-3.5-turbo")
	if err != nil {
		return 0, err
	}
	return len(tk.Encode(resp, nil, nil)), nil
}
