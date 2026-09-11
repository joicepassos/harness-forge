package llm

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func (provider *OpenAIProvider) providerName() string {
	if provider.name == "" {
		return "openai"
	}
	return provider.name
}

// NewProviderFromEnv selects an explicitly named provider without credential fallback.
func NewProviderFromEnv(name, model string) (Provider, error) {
	constructors := map[string]func(string) (*OpenAIProvider, error){
		"openai":   NewOpenAIProviderFromEnv,
		"deepseek": NewDeepSeekProviderFromEnv,
	}
	constructor, ok := constructors[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
	return constructor(model)
}

// NewDeepSeekProviderFromEnv uses DeepSeek's OpenAI-compatible chat API.
func NewDeepSeekProviderFromEnv(model string) (*OpenAIProvider, error) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return &OpenAIProvider{
		apiKey: key, model: model, name: "deepseek",
		endpoint: "https://api.deepseek.com/chat/completions",
		client:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}
