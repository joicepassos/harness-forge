package chatcompat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRetryBudgetAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		status, want int
		after        string
	}{{429, 3, "2"}, {503, 3, ""}, {401, 1, ""}, {402, 1, ""}, {429, 1, "120"}} {
		count := 0
		p := &Client{attempts: 3, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			count++
			return &http.Response{StatusCode: tc.status, Header: http.Header{"Retry-After": []string{tc.after}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"failure"}}`))}, nil
		})}, pause: func(_ context.Context, d time.Duration) error {
			if tc.after == "2" && d != 2*time.Second {
				t.Errorf("Retry-After ignored: %s", d)
			}
			return nil
		}}
		if _, err := p.Generate(context.Background(), Request{}); err == nil {
			t.Fatal("expected error")
		}
		if count != tc.want {
			t.Fatalf("status %d attempts %d want %d", tc.status, count, tc.want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRetry(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}
func TestRetryThenSuccessAndJSONMode(t *testing.T) {
	calls := 0
	p := &Client{attempts: 3, pause: func(context.Context, time.Duration) error { return nil }, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(data), `"response_format":{"type":"json_object"}`) {
			t.Fatal("JSON mode missing")
		}
		calls++
		status := 503
		body := `{}`
		if calls == 2 {
			status = 200
			body = `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	if _, err := p.Generate(context.Background(), Request{JSON: true}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls %d", calls)
	}
}
func TestStreamingCompletionAndTruncation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", true},
		{"data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n", false},
		{"data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", false},
	} {
		calls := 0
		p := &Client{attempts: 3, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}}
		var output strings.Builder
		err := p.Stream(context.Background(), Request{}, func(s string) error { output.WriteString(s); return nil })
		if (err == nil) != tc.valid {
			t.Fatalf("valid %v err %v", tc.valid, err)
		}
		if calls != 1 {
			t.Fatal("stream was replayed")
		}
		if tc.valid && output.String() != "hello" {
			t.Fatalf("output %q", output.String())
		}
	}
}

func TestMissingFinishAndOutputFailure(t *testing.T) {
	p := &Client{client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"unconfirmed"}}]}`))}, nil
	})}}
	if _, err := p.Generate(context.Background(), Request{}); err == nil {
		t.Fatal("missing finish reason accepted")
	}
	expected := errors.New("output unavailable")
	p.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))}, nil
	})
	if err := p.Stream(context.Background(), Request{}, func(string) error { return expected }); !errors.Is(err, expected) {
		t.Fatalf("output error: %v", err)
	}
}
