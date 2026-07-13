package llmgateway

import "testing"

func TestNormalizeOpenAIBaseURL(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":                        "https://api.openai.com",
		"https://api.openai.com/": "https://api.openai.com",
		"https://api.openai.com/v1": "https://api.openai.com",
		"https://api.openai.com/v1/": "https://api.openai.com",
	}
	for in, want := range cases {
		if got := normalizeOpenAIBaseURL(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}

func TestIsReasoningModel(t *testing.T) {
	t.Parallel()
	if !isReasoningModel("o3-mini") || isReasoningModel("gpt-4o-mini") {
		t.Fatal("reasoning model detection")
	}
}
