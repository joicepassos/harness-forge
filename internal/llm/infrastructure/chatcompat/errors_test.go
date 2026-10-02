package chatcompat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestProviderErrorsAreUsefulAndRedacted(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{"error":{"message":"Invalid model: unknown-model"}}`, "Invalid model: unknown-model"},
		{`{"error":{"code":"invalid_request_error","message":"Invalid model"}}`, "code invalid_request_error: Invalid model"},
		{`{"error":{"code":4001,"message":"Invalid field"}}`, "code 4001: Invalid field"},
		{`{"error":{"message":"Invalid key sk-test-secret and context private document contents"}}`, "Invalid key [REDACTED] and context [REDACTED]"},
		{`<html>sk-test-secret private document contents</html>`, "provider returned no readable error message"},
		{`{"error":{"message":"bad\u001b[31m\nparameter"}}`, "bad [31m parameter"},
	} {
		calls := 0
		p := &Client{name: "deepseek", model: "unknown-model", apiKey: "sk-test-secret", attempts: 3,
			client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				data, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(data), `"model":"unknown-model"`) {
					t.Fatal("unexpected request model")
				}
				return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
		_, err := p.Generate(context.Background(), Request{Prompt: `{"repository-file:README.md":"private document contents"}`})
		if err == nil || !strings.Contains(err.Error(), "deepseek request failed (HTTP 400)") || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("error=%v; want %q", err, tc.want)
		}
		if strings.Contains(err.Error(), p.apiKey) || strings.Contains(err.Error(), "private document contents") {
			t.Fatal("sensitive values leaked")
		}
		if calls != 1 {
			t.Fatalf("400 retried %d times", calls)
		}
	}
}

func TestProviderErrorMessageBound(t *testing.T) {
	p := &Client{}
	response := &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"` + strings.Repeat("x", 2000) + `"}}`))}
	err := p.responseError(response, Request{})
	if len(err.Error()) > 600 {
		t.Fatalf("unbounded error: %d", len(err.Error()))
	}
}
