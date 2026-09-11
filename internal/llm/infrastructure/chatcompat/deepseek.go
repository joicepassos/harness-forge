package chatcompat

import "os"

func (provider *Client) providerName() string {
	if provider.name == "" {
		return "openai"
	}
	return provider.name
}

func NewProviderFromEnv(name, model string) (Provider, error) {
	return NewRegistry(os.Getenv).Resolve(name, model)
}

func NewDeepSeekProviderFromEnv(model string) (*Client, error) {
	return NewRegistry(os.Getenv).create("deepseek", model)
}
