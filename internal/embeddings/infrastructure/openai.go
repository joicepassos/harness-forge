package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"harnessforge/internal/embeddings/domain"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultModel = "text-embedding-3-small"

type OpenAI struct {
	endpoint, token string
	client          *http.Client
}

func NewOpenAI(endpoint, token string, client *http.Client) *OpenAI {
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	if copyClient.Timeout == 0 {
		copyClient.Timeout = 30 * time.Second
	}
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &OpenAI{endpoint: strings.TrimRight(endpoint, "/"), token: token, client: &copyClient}
}
func (o *OpenAI) Embed(ctx context.Context, model, input string) (domain.Vector, error) {
	if o.token == "" {
		return domain.Vector{}, fmt.Errorf("OPENAI_API_KEY is not set")
	}
	if model == "" {
		model = DefaultModel
	}
	body, err := json.Marshal(map[string]any{"input": input, "model": model, "encoding_format": "float"})
	if err != nil {
		return domain.Vector{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return domain.Vector{}, err
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := o.client.Do(req)
	if err != nil {
		return domain.Vector{}, fmt.Errorf("embedding request: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil {
		return domain.Vector{}, err
	}
	if len(data) > 4<<20 {
		return domain.Vector{}, fmt.Errorf("embedding response exceeds 4 MiB")
	}
	if response.StatusCode != http.StatusOK {
		return domain.Vector{}, fmt.Errorf("embedding request failed with status %d", response.StatusCode)
	}
	var result struct {
		Model string `json:"model"`
		Data  []struct {
			Values []float64 `json:"embedding"`
			Index  int       `json:"index"`
		} `json:"data"`
		Usage struct {
			Tokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return domain.Vector{}, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(result.Data) != 1 || result.Data[0].Index != 0 {
		return domain.Vector{}, fmt.Errorf("embedding response must contain exactly index zero")
	}
	return domain.Vector{Model: result.Model, Dimensions: len(result.Data[0].Values), Values: result.Data[0].Values, Tokens: result.Usage.Tokens}, nil
}
