package eventtransformer

import "testing"

func TestCountOpenAITokens(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMin int
		wantErr bool
	}{
		{"normal", "Hello, world!", 1, false},
		{"empty", "", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := CountOpenAITokens(c.input)
			if (err != nil) != c.wantErr {
				t.Errorf("CountOpenAITokens() error = %v, wantErr %v", err, c.wantErr)
			}
			if n < c.wantMin {
				t.Errorf("CountOpenAITokens() = %d, want at least %d", n, c.wantMin)
			}
		})
	}
}

// NOTE: tiktoken.EncodingForModel returns error only if model is unknown. We cannot easily mock it without changing the code to allow injection. If needed, this branch is covered by design for all supported models.
