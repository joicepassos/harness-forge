package chatcompat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Stream retries only before the successful response begins; partial output is never replayed.
func (provider *Client) Stream(ctx context.Context, request Request, emit func(string) error) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	response, err := provider.send(ctx, request, true)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var data []string
	hasText, finished := false, false
	dispatch := func() (bool, error) {
		payload := strings.Join(data, "\n")
		data = nil
		if payload == "" {
			return false, nil
		}
		if payload == "[DONE]" {
			if !hasText || !finished {
				return false, fmt.Errorf("stream ended without complete text")
			}
			return true, nil
		}
		var event openAIResponse
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return false, err
		}
		if event.Error.Message != "" {
			return false, fmt.Errorf("%s stream request failed", provider.providerName())
		}
		for _, choice := range event.Choices {
			if err := validFinish(choice.FinishReason); err != nil {
				return false, err
			}
			if choice.FinishReason == "stop" {
				finished = true
			}
			if choice.Delta.Refusal != "" {
				return false, fmt.Errorf("model refused request")
			}
			if choice.Delta.Content != "" {
				if strings.TrimSpace(choice.Delta.Content) != "" {
					hasText = true
				}
				if err := emit(choice.Delta.Content); err != nil {
					return false, err
				}
			}
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := dispatch()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	done, err := dispatch()
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	return fmt.Errorf("stream ended before [DONE]")
}
