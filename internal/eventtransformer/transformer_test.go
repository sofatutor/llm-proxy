package eventtransformer

import "testing"

func TestDispatchTransformer(t *testing.T) {
	tr := DispatchTransformer("openai")
	if tr == nil {
		t.Error("DispatchTransformer(openai) = nil, want OpenAITransformer")
	}
	tr = DispatchTransformer("unknown")
	if tr != nil {
		t.Error("DispatchTransformer(unknown) != nil, want nil")
	}
}
