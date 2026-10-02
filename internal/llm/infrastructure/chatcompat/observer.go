package chatcompat

import (
	"context"
	"net/http"
	"time"
)

// CallEvent exposes transport facts only, never request contents or credentials.
type CallEvent struct {
	Kind       string        `json:"kind"`
	Attempt    int           `json:"attempt"`
	Provider   string        `json:"provider"`
	Model      string        `json:"model"`
	Method     string        `json:"method"`
	Operation  string        `json:"operation"`
	StatusCode int           `json:"status_code,omitempty"`
	At         time.Time     `json:"at"`
	Duration   time.Duration `json:"duration,omitempty"`
	RetryDelay time.Duration `json:"retry_delay,omitempty"`
}

type callObserverKey struct{}

// WithCallObserver installs a synchronous observer for each real HTTP attempt.
// Observers should return promptly and must not modify the request lifecycle.
func WithCallObserver(ctx context.Context, observer func(CallEvent)) context.Context {
	return context.WithValue(ctx, callObserverKey{}, observer)
}

func (provider *Client) reportCall(ctx context.Context, kind string, attempt, status int, started time.Time, delay time.Duration) {
	observer, _ := ctx.Value(callObserverKey{}).(func(CallEvent))
	if observer == nil {
		return
	}
	now := time.Now().UTC()
	duration := time.Duration(0)
	if kind != "started" {
		duration = now.Sub(started)
	}
	observer(CallEvent{Kind: kind, Attempt: attempt, Provider: provider.providerName(), Model: provider.model,
		Method: http.MethodPost, Operation: "/chat/completions", StatusCode: status, At: now, Duration: duration, RetryDelay: delay})
}
