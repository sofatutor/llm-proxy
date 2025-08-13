package obfuscate

import "testing"

func TestObfuscateTokenGeneric(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "len_le_4_len1", in: "a", want: "*"},
		{name: "len_le_4_len4", in: "abcd", want: "****"},
		{name: "len_5_to_12_len5", in: "abcde", want: "ab***"},
		{name: "len_5_to_12_len12", in: "abcdefghijkl", want: "ab**********"},
		{name: ">12", in: "abcdefghijklmnop", want: "abcdefgh...mnop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateTokenGeneric(tt.in)
			if got != tt.want {
				t.Fatalf("ObfuscateTokenGeneric(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestObfuscateTokenSimple(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "****"},
		{name: "len_le_8_len4", in: "abcd", want: "****"},
		{name: "len_le_8_len8", in: "abcdefgh", want: "****"},
		{name: ">8", in: "abcdefghijkl", want: "abcd****ijkl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateTokenSimple(tt.in)
			if got != tt.want {
				t.Fatalf("ObfuscateTokenSimple(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestObfuscateTokenByPrefix(t *testing.T) {
	const prefix = "sk-"
	tests := []struct {
		name   string
		in     string
		want   string
		prefix string
	}{
		{name: "empty", in: "", want: "", prefix: prefix},
		{name: "no_prefix_returns_input", in: "abc", want: "abc", prefix: prefix},
		{name: "with_prefix_too_short", in: "sk-12345678", want: "sk-12345678", prefix: prefix},
		{name: "with_prefix_long", in: "sk-1234567890", want: "sk-1234**7890", prefix: prefix},
		{name: "custom_prefix", in: "tk-ABCDEFGHIJKL", want: "tk-ABCD****IJKL", prefix: "tk-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ObfuscateTokenByPrefix(tt.in, tt.prefix)
			if got != tt.want {
				t.Fatalf("ObfuscateTokenByPrefix(%q,%q) = %q, want %q", tt.in, tt.prefix, got, tt.want)
			}
		})
	}
}
