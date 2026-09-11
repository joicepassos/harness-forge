package chatcompat

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRegistryRoutesProviders(t *testing.T) {
	for _, tc := range []struct{ name, endpoint, key string }{
		{"gemini", "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", "GEMINI_API_KEY"},
		{"groq", "https://api.groq.com/openai/v1/chat/completions", "GROQ_API_KEY"},
		{"ollama", "http://localhost:11434/v1/chat/completions", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry(func(key string) string {
				if key != tc.key {
					t.Fatalf("unexpected key lookup %s", key)
				}
				return "test-secret"
			})
			p, err := registry.create(tc.name, "chosen-model")
			if err != nil {
				t.Fatal(err)
			}
			p.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != tc.endpoint {
					t.Fatalf("wrong endpoint %s", r.URL)
				}
				wantAuth := "Bearer test-secret"
				if tc.key == "" {
					wantAuth = ""
				}
				if r.Header.Get("Authorization") != wantAuth {
					t.Fatal("wrong authorization")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
			})
			if _, err := p.Generate(context.Background(), Request{Prompt: "hello"}); err != nil {
				t.Fatal(err)
			}
			if _, err := registry.Resolve(tc.name, ""); err == nil {
				t.Fatal("missing model accepted")
			}
		})
	}
}
