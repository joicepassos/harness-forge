package main

import (
	"bytes"
	"context"
	"errors"
	"harnessforge/internal/analyzer"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func TestProposalPlainProgressReportsOnlyRealSafeStages(t *testing.T) {
	var output bytes.Buffer
	s := setupSession{output: setupLocalizedWriter{output: &output}}
	propose := func(ctx context.Context, _ setupProvider, _ *analyzer.Analysis, _ []setupDocument, _ string) (setupAIProposal, error) {
		reportSetupStage(ctx, "context_prepared")
		reportSetupStage(ctx, "waiting_provider")
		reportSetupStage(ctx, "secret-or-document-content")
		return setupAIProposal{}, errors.New("provider stopped")
	}
	_, err := s.generateProposal(context.Background(), setupProvider{Key: "never-print-key"}, nil, nil, "never-print-notes", propose)
	if err == nil {
		t.Fatal("failure lost")
	}
	text := output.String()
	if !strings.Contains(text, "Contexto preparado.") || !strings.Contains(text, "Aguardando") {
		t.Fatalf("missing actual stages: %s", text)
	}
	for _, unexpected := range []string{"Resposta do provedor recebida", "Validando", "secret", "never-print", "\x1b", "%"} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("unsafe/fabricated output: %s", text)
		}
	}
}

func TestProposalProgressShowsElapsedAndCancelsWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &setupProgressModel{session: setupSession{output: &bytes.Buffer{}}, spinner: spinner.New(), started: time.Now().Add(-3 * time.Second), width: 80, cancel: cancel, phase: "Waiting for provider response..."}
	m.Update(setupStage("context_prepared"))
	m.Update(setupStage("waiting_provider"))
	if len(m.history) != 2 {
		t.Fatal("actual transitions missing")
	}
	view := m.View().Content
	if !strings.Contains(view, "3s") || strings.Contains(view, "%") {
		t.Fatalf("bad activity indicator: %s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.cancelled || ctx.Err() == nil || m.completed {
		t.Fatal("cancellation did not stop worker before finishing")
	}
	m.Update(setupProposalOutcome{err: context.Canceled})
	if strings.Contains(m.View().Content, "ready for review") {
		t.Fatal("failed proposal shown as ready")
	}
}

func TestInteractiveProposalCancellationWaitsForWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	started, stopped := make(chan struct{}), make(chan struct{})
	go func() { <-started; cancel() }()
	propose := func(ctx context.Context, _ setupProvider, _ *analyzer.Analysis, _ []setupDocument, _ string) (setupAIProposal, error) {
		close(started)
		reportSetupStage(ctx, "waiting_provider")
		<-ctx.Done()
		time.Sleep(10 * time.Millisecond)
		close(stopped)
		return setupAIProposal{}, ctx.Err()
	}
	var output bytes.Buffer
	s := setupSession{interactive: true, input: strings.NewReader(""), output: &output}
	_, err := s.generateProposal(ctx, setupProvider{}, nil, nil, "", propose)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("returned while provider worker was still running")
	}
}
