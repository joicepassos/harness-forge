package chatcompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultOpenAIModel = "gpt-4o-mini"

type Client struct {
	endpoint, name, apiKey, model string
	client                        *http.Client
	attempts                      int
	pause                         func(context.Context, time.Duration) error
}

func NewOpenAIProviderFromEnv(model string) (*Client, error) {
	return NewRegistry(os.Getenv).create("openai", model)
}

func (provider *Client) Generate(ctx context.Context, request Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	response, err := provider.send(ctx, request, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result openAIResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", provider.providerName(), err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("%s response did not include choices", provider.providerName())
	}
	choice := result.Choices[0]
	if choice.FinishReason == "" {
		return nil, fmt.Errorf("model response is missing finish_reason")
	}
	if err := validFinish(choice.FinishReason); err != nil {
		return nil, err
	}
	if choice.Message.Refusal != "" {
		return nil, fmt.Errorf("%s refused the request", provider.providerName())
	}
	if strings.TrimSpace(choice.Message.Content) == "" {
		return nil, fmt.Errorf("%s response did not include text", provider.providerName())
	}
	return &Response{Content: choice.Message.Content, InputTokens: result.Usage.PromptTokens, OutputTokens: result.Usage.CompletionTokens}, nil
}
func validFinish(reason string) error {
	if reason != "" && reason != "stop" {
		return fmt.Errorf("incomplete model output (finish_reason=%s)", reason)
	}
	return nil
}
func (provider *Client) send(ctx context.Context, request Request, stream bool) (*http.Response, error) {
	body := openAIRequest{Model: provider.model, Temperature: request.Temperature, Stream: stream, Messages: []openAIMessage{{Role: "system", Content: request.SystemPrompt}, {Role: "user", Content: request.Prompt}}}
	if request.JSON {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint := provider.endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	attempts := provider.attempts
	if attempts < 1 {
		attempts = 1
	}
	pause := provider.pause
	if pause == nil {
		pause = waitForRetry
	}
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		if provider.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+provider.apiKey)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := provider.client.Do(req)
		if err != nil {
			return nil, err
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		}
		var detail openAIResponse
		_ = json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&detail)
		response.Body.Close()
		failure := fmt.Errorf("%s request failed (HTTP %d): %s", provider.providerName(), response.StatusCode, detail.Error.Message)
		delay := time.Duration(1<<attempt) * 200 * time.Millisecond
		if after, ok := retryAfter(response.Header.Get("Retry-After")); ok {
			delay = after
		}
		if !retryable(response.StatusCode) || attempt+1 == attempts || delay > 10*time.Second {
			return nil, failure
		}
		if err := pause(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("retry budget exhausted")
}
func retryable(status int) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	}
	return false
}
func retryAfter(value string) (time.Duration, bool) {
	if n, err := strconv.Atoi(value); err == nil && n >= 0 {
		if n > 10 {
			return 11 * time.Second, true
		}
		return time.Duration(n) * time.Second, true
	}
	if date, err := http.ParseTime(value); err == nil {
		delay := time.Until(date)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}
func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type responseFormat struct {
	Type string `json:"type"`
}
type openAIRequest struct {
	Model          string          `json:"model"`
	Messages       []openAIMessage `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
}
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Refusal string `json:"refusal,omitempty"`
}
type openAIResponse struct {
	Choices []struct {
		Message      openAIMessage `json:"message"`
		Delta        openAIMessage `json:"delta"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
