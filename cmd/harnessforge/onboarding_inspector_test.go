package main

import (
	"bytes"
	"harnessforge/internal/llm/infrastructure/chatcompat"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestInspectorEventsAreChronologicalAndHeadersNotFullResponse(t *testing.T) {
	start := time.Date(2026, 10, 2, 19, 31, 32, 0, time.UTC)
	run := setupBackgroundRun{Events: []setupRunEvent{{Stage: "context_prepared", At: start}, {Stage: "ready", At: start.Add(3 * time.Second)}}, Calls: []chatcompat.CallEvent{{Kind: "response", Attempt: 1, StatusCode: 200, At: start.Add(time.Second)}, {Kind: "started", Attempt: 1, At: start.Add(500 * time.Millisecond)}}}
	view := setupInspectorDetails(run, 1)
	previous := -1
	for _, label := range []string{"Context prepared.", "started", "Response headers received; reading body", "AI proposal ready for review."} {
		index := strings.Index(view, label)
		if index <= previous {
			t.Fatalf("events not chronological: %s", view)
		}
		previous = index
	}
	if strings.Contains(view, "Provider response received.") {
		t.Fatal("headers presented as full provider response")
	}
}

func TestInspectorOnlyShowsSafeMetadataAndActualStages(t *testing.T) {
	run := setupBackgroundRun{ID: "run-test", Provider: "deepseek", Model: "deepseek-flash", Status: "running", Stage: "waiting_provider", StartedAt: time.Now(), Documents: []setupRunDocument{{Path: "README.md", Source: "repository-file:README.md", Hash: "private-hash"}}, Events: []setupRunEvent{{Stage: "waiting_provider", At: time.Now()}, {Stage: "not-a-real-stage", At: time.Now()}}, Proposal: &setupAIProposal{Summary: "never-show-document-content", Rules: []setupRule{{Description: "never-show-quote"}}}}
	var all string
	for section := 0; section < 4; section++ {
		all += setupInspectorDetails(run, section)
	}
	for _, forbidden := range []string{"private-hash", "never-show", "not-a-real-stage", "HTTP 200", "100%"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("unsafe or fabricated detail %q: %s", forbidden, all)
		}
	}
	if !strings.Contains(all, "README.md") || !strings.Contains(all, "Waiting for provider response") {
		t.Fatal("metadata and true event missing")
	}
	if !strings.Contains(all, "harnessforge init resume run-test") {
		t.Fatal("review command missing")
	}
}

func TestInspectorDetachDoesNotCancelAndSupportsNarrowView(t *testing.T) {
	for _, width := range []int{20, 40, 80} {
		for _, height := range []int{10, 20} {
			m := &setupInspectorModel{run: setupBackgroundRun{ID: "run-test", Status: "running", StartedAt: time.Now()}, width: width, height: height, spinner: spinner.New(), viewport: viewport.New()}
			if m.View().Content == "" {
				t.Fatal("empty inspector")
			}
			for _, line := range strings.Split(m.View().Content, "\n") {
				if lipgloss.Width(line) > width {
					t.Fatalf("line overflows width %d: %q", width, line)
				}
			}
			if len(strings.Split(m.View().Content, "\n")) > height {
				t.Fatalf("view exceeds height %d", height)
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if m.section != 1 {
				t.Fatal("navigation did not change section")
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'd'})
			if cmd == nil || m.run.Status != "running" {
				t.Fatal("detach must only close view")
			}
		}
	}
}

func TestInspectorFriendlyStatesPreserveFilenamesAndAppliedOutcome(t *testing.T) {
	s := setupSession{output: setupLocalizedWriter{output: &bytes.Buffer{}}}
	run := setupBackgroundRun{ID: "run-test", Status: "running", Documents: []setupRunDocument{{Path: "Contexto-Modelo.md"}}}
	text := s.inspectorRunText(setupInspectorDetails(run, 0, s), run)
	if !strings.Contains(text, "Contexto-Modelo.md") {
		t.Fatalf("filename translated: %s", text)
	}
	if s.inspectorStatus("running") != "Em execucao" {
		t.Fatal("unfriendly state")
	}
	run.Status = "applied"
	run.Proposal = &setupAIProposal{}
	view := setupInspectorDetails(run, 3, s)
	if !strings.Contains(view, "applied after confirmation") || strings.Contains(view, "resume") {
		t.Fatalf("applied proposal offered for review: %s", view)
	}
}

func TestInspectorStripsTerminalControls(t *testing.T) {
	if actual := setupInspectorSafe("README\x1b[31m\r\n.md"); strings.ContainsAny(actual, "\x1b\r\n") {
		t.Fatalf("unsafe terminal controls: %q", actual)
	}
}
