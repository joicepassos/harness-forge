package infrastructure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIEmbeddingContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/embeddings" || r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Fatal("invalid request")
		}
		w.Write([]byte(`{"model":"fixture-v1","data":[{"index":0,"embedding":[0.1,0.2]}],"usage":{"total_tokens":3}}`))
	}))
	defer server.Close()
	vector, err := NewOpenAI(server.URL, "fixture-token", server.Client()).Embed(context.Background(), "fixture-v1", "text")
	if err != nil || vector.Dimensions != 2 || vector.Tokens != 3 {
		t.Fatal(vector, err)
	}
}
func TestOpenAIEmbeddingFailures(t *testing.T) {
	if _, err := NewOpenAI("http://unused", "", http.DefaultClient).Embed(context.Background(), "m", "text"); err == nil {
		t.Fatal("missing token accepted")
	}
	for _, body := range []string{`not-json`, `{"model":"m","data":[]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		_, err := NewOpenAI(server.URL, "fixture-token", server.Client()).Embed(context.Background(), "m", "text")
		server.Close()
		if err == nil {
			t.Fatal("malformed response accepted")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", (4<<20)+1))) }))
	defer server.Close()
	if _, err := NewOpenAI(server.URL, "fixture-token", server.Client()).Embed(context.Background(), "m", "text"); err == nil {
		t.Fatal("oversized response accepted")
	}
}
