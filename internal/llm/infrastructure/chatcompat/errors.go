package chatcompat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// Only structured provider messages are shown; raw bodies can contain HTML or
// request dumps. Remove credentials and supplied context before displaying them.
func (provider *Client) responseError(response *http.Response, request Request) error {
	prefix := fmt.Sprintf("%s request failed (HTTP %d)", provider.providerName(), response.StatusCode)
	var envelope struct {
		Error struct {
			Message string          `json:"message"`
			Code    json.RawMessage `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&envelope); err != nil {
		return fmt.Errorf("%s: provider returned no readable error message", prefix)
	}
	message := envelope.Error.Message
	var code string
	if json.Unmarshal(envelope.Error.Code, &code) != nil {
		var number json.Number
		if json.Unmarshal(envelope.Error.Code, &number) == nil {
			code = number.String()
		}
	}
	if code != "" {
		message = "code " + code + ": " + message
	}
	redact := func(value string) {
		if value != "" {
			message = strings.ReplaceAll(message, value, "[REDACTED]")
		}
	}
	redact(provider.apiKey)
	for _, prompt := range []string{request.SystemPrompt, request.Prompt} {
		redact(prompt)
		for _, line := range strings.Split(prompt, "\n") {
			if len(strings.TrimSpace(line)) >= 8 {
				redact(strings.TrimSpace(line))
			}
		}
		var values any
		if json.Unmarshal([]byte(prompt), &values) == nil {
			var visit func(any)
			visit = func(value any) {
				switch value := value.(type) {
				case string:
					redact(value)
				case map[string]any:
					for _, item := range value {
						visit(item)
					}
				case []any:
					for _, item := range value {
						visit(item)
					}
				}
			}
			visit(values)
		}
	}
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, message)
	message = strings.Join(strings.Fields(message), " ")
	runes := []rune(message)
	if len(runes) > 512 {
		message = string(runes[:512]) + "..."
	}
	if message == "" {
		return fmt.Errorf("%s: provider returned no readable error message", prefix)
	}
	return fmt.Errorf("%s: %s", prefix, message)
}
