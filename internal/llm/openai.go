package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultOpenAIModel = "gpt-4o-mini"

type OpenAIProvider struct {
	endpoint string
	name     string
	apiKey   string
	model    string
	client   *http.Client
}

func NewOpenAIProviderFromEnv(model string) (*OpenAIProvider, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}

	if model == "" {
		model = defaultOpenAIModel
	}

	return &OpenAIProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (provider *OpenAIProvider) Generate(ctx context.Context, request Request) (*Response, error) {
	body := openAIRequest{
		Model:       provider.model,
		Temperature: request.Temperature,
		Messages: []openAIMessage{
			{Role: "system", Content: request.SystemPrompt},
			{Role: "user", Content: request.Prompt},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	endpoint := provider.endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := provider.client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	var response openAIResponse
	decodeErr := json.NewDecoder(httpResponse.Body).Decode(&response)

	if httpResponse.StatusCode >= 400 {
		return nil, fmt.Errorf("%s request failed (HTTP %d): %s", provider.providerName(), httpResponse.StatusCode, response.Error.Message)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("decode %s response: %w", provider.providerName(), decodeErr)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("%s response did not include choices", provider.providerName())
	}
	if strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("%s response did not include text", provider.providerName())
	}

	return &Response{
		Content:      response.Choices[0].Message.Content,
		InputTokens:  response.Usage.PromptTokens,
		OutputTokens: response.Usage.CompletionTokens,
	}, nil
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
