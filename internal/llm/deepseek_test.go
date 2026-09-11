package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekProvider(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-deepseek")
	p, err := NewDeepSeekProviderFromEnv("")
	if err != nil {
		t.Fatal(err)
	}
	if p.model != "deepseek-v4-flash" {
		t.Fatalf("unexpected default: %s", p.model)
	}
	p.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.deepseek.com/chat/completions" {
			t.Fatalf("wrong endpoint: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-deepseek" {
			t.Fatal("wrong credential")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
	})
	if _, err := p.Generate(context.Background(), Request{Prompt: "test"}); err != nil {
		t.Fatal(err)
	}
	p, err = NewDeepSeekProviderFromEnv("custom-model")
	if err != nil || p.model != "custom-model" {
		t.Fatal("model override failed")
	}
}

func TestProviderSelectionRejectsMissingKeyAndUnknownProvider(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "other-key")
	if _, err := NewProviderFromEnv("deepseek", ""); err == nil {
		t.Fatal("missing DeepSeek key accepted")
	}
	if _, err := NewProviderFromEnv("unknown", ""); err == nil {
		t.Fatal("unknown provider accepted")
	}
}
