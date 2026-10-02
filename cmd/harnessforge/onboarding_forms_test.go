package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetupRedirectedInputKeepsPlainPrompts(t *testing.T) {
	if setupInteractive(strings.NewReader(""), &bytes.Buffer{}, false) {
		t.Fatal("redirected input entered interactive mode")
	}
	t.Setenv("HARNESSFORGE_ACCESSIBLE", "1")
	if setupInteractive(strings.NewReader(""), &bytes.Buffer{}, false) {
		t.Fatal("accessible mode entered interactive mode")
	}
	for _, command := range []string{"init", "install", "tui"} {
		cmd, _, err := newRootCommand().Find([]string{command})
		if err != nil || cmd.Flags().Lookup("accessible") == nil {
			t.Fatalf("%s has no accessible flag: %v", command, err)
		}
	}
}

func TestSetupFormLabelsUsePortugueseWithoutChangingPayload(t *testing.T) {
	var output bytes.Buffer
	s := setupSession{output: setupLocalizedWriter{output: &output}}
	if s.uiText("Yes") != "Sim" || s.uiText("Continue") != "Continuar" {
		t.Fatal("missing Portuguese labels")
	}
	if s.uiOutput() != &output {
		t.Fatal("TUI escape sequences would pass through localization")
	}
	if s.uiText("deepseek-v4-flash") != "deepseek-v4-flash" {
		t.Fatal("model identifier translated")
	}
}
