package main

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestInterfaceLanguageIsFirstAndPortugueseGuidesCancellation(t *testing.T) {
	var output bytes.Buffer
	err := runGuidedInit(context.Background(), strings.NewReader("2\n\nn\n"), &output, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.HasPrefix(text, "Interface language / Idioma da interface") || !strings.Contains(text, "Analisar este projeto agora?") || !strings.Contains(text, "Analise cancelada") {
		t.Fatalf("language did not guide setup: %s", text)
	}
}

func TestPortugueseConfirmation(t *testing.T) {
	s := setupSession{reader: bufio.NewReader(strings.NewReader("sim\n")), output: &bytes.Buffer{}}
	confirmed, err := s.confirm("Continuar?")
	if err != nil || !confirmed {
		t.Fatalf("confirmation = %v, %v", confirmed, err)
	}
}
