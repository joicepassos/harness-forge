package chatcompat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCallObserverReportsRealAttemptsWithoutSensitiveData(t *testing.T) {
	var events []CallEvent
	calls := 0
	p := &Client{name: "deepseek", model: "deepseek-flash", apiKey: "secret-key", endpoint: "https://example.invalid/private?token=secret-query", attempts: 3,
		pause: func(context.Context, time.Duration) error { return nil },
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			status, body := 503, `{}`
			if calls == 2 {
				status, body = 200, `{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
	ctx := WithCallObserver(context.Background(), func(event CallEvent) { events = append(events, event) })
	if _, err := p.Generate(ctx, Request{Prompt: "secret-document", SystemPrompt: "secret-system"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"started", "response", "retry", "started", "response"}
	if len(events) != len(want) {
		t.Fatalf("events: %+v", events)
	}
	for i, event := range events {
		if event.Kind != want[i] || event.Operation != "/chat/completions" || event.Method != http.MethodPost || event.At.IsZero() {
			t.Fatalf("event %d: %+v", i, event)
		}
		if event.Duration < 0 {
			t.Fatal("negative duration")
		}
	}
	if events[1].StatusCode != 503 || events[4].StatusCode != 200 || events[3].Attempt != 2 || events[2].RetryDelay != 200*time.Millisecond {
		t.Fatalf("incorrect transport facts: %+v", events)
	}
	data, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-key", "secret-query", "secret-document", "secret-system", "example.invalid", "private"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("sensitive metadata: %s", forbidden)
		}
	}
}

func TestCallObserverTransportFailureHasNoInventedHTTPStatus(t *testing.T) {
	var events []CallEvent
	p := &Client{client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("secret transport error")
	})}}
	ctx := WithCallObserver(context.Background(), func(event CallEvent) { events = append(events, event) })
	if _, err := p.Generate(ctx, Request{}); err == nil {
		t.Fatal("expected failure")
	}
	if len(events) != 2 || events[0].Kind != "started" || events[1].Kind != "failed" || events[1].StatusCode != 0 {
		t.Fatalf("events: %+v", events)
	}
}
