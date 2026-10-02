package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestSetupEnvironmentKeyIsAcknowledgedWithoutValue(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "secret-never-display")
	var output bytes.Buffer
	input := strings.NewReader("")
	s := setupSession{reader: bufio.NewReader(input), output: &output}
	config := setupProvider{Name: "deepseek"}
	if err := ensureSetupKey(s, input, &config); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "key found in the environment") || strings.Contains(output.String(), "secret-never-display") || config.Key != "" {
		t.Fatalf("unsafe or missing acknowledgement: %q", output.String())
	}
}
