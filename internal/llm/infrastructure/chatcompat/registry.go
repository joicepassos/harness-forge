package chatcompat

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type definition struct{ endpoint, keyVariable, defaultModel string }

// Registry resolves provider strategies from explicit configuration.
// Providers sharing the chat-completions protocol reuse this transport adapter.
type Registry struct {
	lookup      func(string) string
	definitions map[string]definition
}

func NewRegistry(lookup func(string) string) *Registry {
	return &Registry{lookup: lookup, definitions: map[string]definition{
		"openai":   {"https://api.openai.com/v1/chat/completions", "OPENAI_API_KEY", "gpt-4o-mini"},
		"deepseek": {"https://api.deepseek.com/chat/completions", "DEEPSEEK_API_KEY", "deepseek-v4-flash"},
		"gemini":   {"https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", "GEMINI_API_KEY", ""},
		"groq":     {"https://api.groq.com/openai/v1/chat/completions", "GROQ_API_KEY", ""},
		"ollama":   {"http://localhost:11434/v1/chat/completions", "", ""},
	}}
}

func (registry *Registry) Resolve(name, model string) (Provider, error) {
	return registry.create(name, model)
}

func (registry *Registry) create(name, model string) (*Client, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	config, ok := registry.definitions[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider %q (use openai, deepseek, gemini, groq or ollama)", name)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = config.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("--model is required for %s", name)
	}
	key := ""
	if config.keyVariable != "" {
		key = strings.TrimSpace(registry.lookup(config.keyVariable))
		if key == "" {
			return nil, fmt.Errorf("%s is not set", config.keyVariable)
		}
	}
	return &Client{attempts: 3, name: name, endpoint: config.endpoint, apiKey: key, model: model, client: &http.Client{Timeout: 60 * time.Second}}, nil
}

// Supports checks configuration without requiring credentials or making API calls.
func (registry *Registry) Supports(name string) bool {
	_, ok := registry.definitions[name]
	return ok
}

func (registry *Registry) DefaultModel(name string) string {
	config, ok := registry.definitions[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return ""
	}
	return config.defaultModel
}
