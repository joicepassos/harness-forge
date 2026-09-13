package infrastructure

import (
	"context"
	"harnessforge/internal/github/application"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientPaginatesPreservesSourcesAndDoesNotMutate(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		if r.Method != http.MethodGet {
			t.Fatal("non-read request")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("token header missing")
		}
		switch r.URL.Path {
		case "/repos/acme/demo/issues":
			w.Write([]byte(`[{"number":2,"body":"Decision: use ports","html_url":"https://example.test/issues/2","user":{"login":"a"}}]`))
		case "/repos/acme/demo/pulls":
			w.Write([]byte(`[{"number":4,"body":"Consider adapters","html_url":"https://example.test/pull/4","head":{"sha":"abc"}}]`))
		case "/repos/acme/demo/pulls/4/reviews":
			w.Write([]byte(`[{"number":4,"body":"Please add a test","html_url":"https://example.test/pull/4#review","commit_id":"abc","user":{"login":"b"}}]`))
		case "/repos/acme/demo/commits":
			w.Write([]byte(`[{"sha":"def","html_url":"https://example.test/commit/def","commit":{"message":"Document limits"}}]`))
		case "/repos/acme/demo/issues/2/comments", "/repos/acme/demo/issues/4/comments", "/repos/acme/demo/pulls/4/comments":
			w.Write([]byte("[]"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	report, err := application.NewExtract(NewClientWithOptions(server.URL, "test-token", server.Client(), 2)).Execute(context.Background(), "acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Discussions) != 4 || len(report.Candidates) != 4 {
		t.Fatalf("%#v", report)
	}
	foundDecision := false
	for _, candidate := range report.Candidates {
		if candidate.Classification == "decision" && candidate.RequiresHumanReview {
			foundDecision = true
		}
	}
	if !foundDecision {
		t.Fatalf("decision candidate = %#v", report.Candidates)
	}
	if strings.Contains(strings.Join(requests, "\n"), "POST") {
		t.Fatal("mutation requested")
	}
}
func TestClientRejectsUnsafeRepositoryAndHonorsCancellation(t *testing.T) {
	client := NewClientWithOptions("http://example.test", "", http.DefaultClient, 1)
	if _, err := client.Read(context.Background(), "../unsafe"); err == nil {
		t.Fatal("unsafe repository accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Read(ctx, "acme/demo"); err == nil {
		t.Fatal("cancelled context accepted")
	}
}

func TestClientBoundsPagination(t *testing.T) {
	items := "[" + strings.TrimSuffix(strings.Repeat(`{"number":1,"html_url":"https://example.test/item"},`, 100), ",") + "]"
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "1" {
			w.Write([]byte(items))
			return
		}
		w.Write([]byte("[]"))
	}))
	defer server.Close()
	client := NewClientWithOptions(server.URL, "", server.Client(), 2)
	itemsRead, err := client.list(context.Background(), "/repos/acme/demo/issues?state=all")
	if err != nil {
		t.Fatal(err)
	}
	if len(itemsRead) != 100 || requests != 2 {
		t.Fatalf("items=%d requests=%d", len(itemsRead), requests)
	}
}
