package chatcompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestNewOpenAIProviderFromEnvRequiresAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	_, err := NewOpenAIProviderFromEnv("")
	if err == nil {
		t.Fatal("NewOpenAIProviderFromEnv() error = nil, want error")
	}
}

func TestNewOpenAIProviderFromEnvUsesDefaultModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	provider, err := NewOpenAIProviderFromEnv("")
	if err != nil {
		t.Fatalf("NewOpenAIProviderFromEnv() error = %v", err)
	}

	if provider.model != defaultOpenAIModel {
		t.Fatalf("model = %q, want %q", provider.model, defaultOpenAIModel)
	}

	if os.Getenv("OPENAI_API_KEY") != "test-key" {
		t.Fatal("OPENAI_API_KEY was not preserved")
	}
}

// roundTripFunc keeps provider tests offline while exercising the HTTP contract.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateHTTPContract(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{"success", 200, `{"choices":[{"message":{"content":"Project summary"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3}}`, ""},
		{"authentication", 401, `{"error":{"message":"Invalid key"}}`, "HTTP 401"},
		{"rate limit", 429, `{"error":{"message":"Rate limited"}}`, "HTTP 429"},
		{"proxy failure", 502, `<html>Bad gateway</html>`, "HTTP 502"},
		{"invalid json", 200, `{`, "decode openai response"},
		{"no choices", 200, `{"choices":[]}`, "did not include choices"},
		{"empty content", 200, `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`, "did not include text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := &Client{apiKey: "test-key", model: "test-model", client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.URL.String() != "https://api.openai.com/v1/chat/completions" {
					t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing authorization")
				}
				var payload openAIRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Model != "test-model" || len(payload.Messages) != 2 || payload.Messages[1].Content != "Project" {
					t.Errorf("unexpected payload: %+v", payload)
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			result, err := provider.Generate(context.Background(), Request{SystemPrompt: "Summarize", Prompt: "Project", Temperature: 0.2})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %s", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Content != "Project summary" || result.InputTokens != 12 || result.OutputTokens != 3 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
