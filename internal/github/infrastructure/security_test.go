package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirectDoesNotForwardCredential(t *testing.T) {
	contacted := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { contacted = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer server.Close()
	_, err := NewClientWithOptions(server.URL, "test-token", server.Client(), 1).Read(context.Background(), "acme/demo")
	if err == nil || contacted {
		t.Fatalf("redirect followed: %v %v", contacted, err)
	}
}

func TestUntrustedResponsesFailExplicitly(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"credential", `[{"html_url":"https://example.test/1","body":"test-token"}]`, "credential", 200},
		{"unsafe URL", `[{"html_url":"javascript:alert(1)"}]`, "HTTPS", 200},
		{"malformed", `not json`, "decode", 200},
		{"oversized", strings.Repeat("x", maxBodyBytes+1), "exceeds", 200},
		{"rate limit", ``, "rate limit", 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer server.Close()
			_, err := NewClientWithOptions(server.URL, "test-token", server.Client(), 1).Read(context.Background(), "acme/demo")
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "test-token") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPaginationExhaustionCannotReturnIncompleteSuccess(t *testing.T) {
	body := "[" + strings.TrimSuffix(strings.Repeat(`{"html_url":"https://example.test/item"},`, 100), ",") + "]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer server.Close()
	_, err := NewClientWithOptions(server.URL, "", server.Client(), 1).list(context.Background(), "/repos/acme/demo/issues")
	if err == nil || !strings.Contains(err.Error(), "pagination limit") {
		t.Fatal(err)
	}
}

func TestInlineCommentRetainsOriginalRevisionAndContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/demo/pulls":
			w.Write([]byte(`[{"number":1,"html_url":"https://example.test/pr/1","head":{"sha":"new"}}]`))
		case "/repos/acme/demo/pulls/1/comments":
			w.Write([]byte(`[{"html_url":"https://example.test/pr/1#comment","commit_id":"old","body":"Please check this boundary","path":"src/file.go","line":8}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	items, err := NewClientWithOptions(server.URL, "", server.Client(), 1).Read(context.Background(), "acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Kind == "review_comment" {
			if item.Revision != "old" || item.ParentURL != "https://example.test/pr/1" || item.Line != 8 || item.Path != "src/file.go" {
				t.Fatal(item)
			}
			return
		}
	}
	t.Fatal("inline comment not retrieved")
}
