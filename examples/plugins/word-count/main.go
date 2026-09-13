package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const apiVersion = "harnessforge.plugin/v1"

type request struct {
	APIVersion string         `json:"api_version"`
	Operation  string         `json:"operation"`
	Capability string         `json:"capability"`
	Input      map[string]any `json:"input"`
}

func main() {
	var value request
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		fail(err)
		return
	}
	if value.APIVersion != apiVersion || value.Operation != "execute" || value.Capability != "analyzer" {
		fail(fmt.Errorf("unsupported request"))
		return
	}
	text, ok := value.Input["text"].(string)
	if !ok {
		fail(fmt.Errorf("input.text must be a string"))
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"api_version": apiVersion, "output": map[string]any{"words": len(strings.Fields(text))}})
}

func fail(err error) {
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"api_version": apiVersion, "output": map[string]any{}, "error": err.Error()})
}
