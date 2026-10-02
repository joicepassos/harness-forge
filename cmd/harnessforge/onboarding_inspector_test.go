package main

import (
	"bytes"
	"context"
	"harnessforge/internal/llm/infrastructure/chatcompat"
	"os"
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
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
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

func TestInspectorFocusAndReviewActions(t *testing.T) {
	m := &setupInspectorModel{run: setupBackgroundRun{Status: "ready"}, spinner: spinner.New(), viewport: viewport.New()}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.section != 1 || m.focus != 0 {
		t.Fatal("arrows should select sections without stealing focus")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focus != 1 {
		t.Fatal("enter should focus details")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.section != 1 {
		t.Fatal("scrolling details changed section")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.focus != 0 || cmd != nil {
		t.Fatal("escape should return to sections, not detach")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focus != 2 || !m.review || cmd == nil {
		t.Fatal("review action did not hand off to explicit review")
	}
	m = &setupInspectorModel{run: setupBackgroundRun{Status: "running"}}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'r'})
	if m.review || cmd != nil {
		t.Fatal("running proposal must not be reviewable")
	}
}

func TestInspectorCancellationRequiresConfirmation(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	run, err := loadSetupBackgroundRun(payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	m := &setupInspectorModel{run: run}
	m.Update(tea.KeyPressMsg{Code: 'c'})
	if !m.confirmCancel {
		t.Fatal("missing cancel confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.confirmCancel {
		t.Fatal("escape did not dismiss confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: 'c'})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	path, _ := setupRunPath(run.ID)
	if _, err := os.Stat(path + ".cancel"); err != nil {
		t.Fatal("confirmed cancellation was not requested")
	}
}

func TestInspectorUsesColorAndAlternateScreen(t *testing.T) {
	m := &setupInspectorModel{run: setupBackgroundRun{Status: "ready", StartedAt: time.Now(), UpdatedAt: time.Now()}, width: 80, height: 24, spinner: spinner.New(), viewport: viewport.New()}
	v := m.View()
	if !v.AltScreen || !strings.Contains(v.Content, "\x1b[") || !strings.Contains(v.Content, "Review proposal") {
		t.Fatal("missing styled full-screen inspector and visible actions")
	}
}

func TestInspectorColorPolicyAndCommandPropagation(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if setupInspectorColors(context.Background()) {
		t.Fatal("automatic color ignored NO_COLOR")
	}
	if !setupInspectorColors(context.WithValue(context.Background(), setupInspectorColorKey{}, "always")) {
		t.Fatal("explicit always did not override NO_COLOR")
	}
	if setupInspectorColors(context.WithValue(context.Background(), setupInspectorColorKey{}, "never")) {
		t.Fatal("explicit never enabled colors")
	}
	setupBackgroundTestCache(t)
	root := newRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"init", "runs", "--color", "always"})
	command, _, err := root.Find([]string{"init", "runs"})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !setupInspectorColors(command.Context()) {
		t.Fatal("root language hook lost the explicit color setting")
	}
}

func TestInspectorProposalLifecycleHasExplicitNextActions(t *testing.T) {
	m := &setupInspectorModel{run: setupBackgroundRun{Status: "running"}, width: 80, height: 24, spinner: spinner.New(), viewport: viewport.New()}
	ready := setupBackgroundRun{Status: "ready", Proposal: &setupAIProposal{}}
	m.Update(setupInspectorPoll{run: ready})
	if m.section != 3 || m.focus != 2 || m.action != 0 {
		t.Fatal("ready proposal did not focus its next action")
	}
	if !strings.Contains(m.View().Content, "PROPOSAL READY") {
		t.Fatal("missing ready step")
	}
	m.run = setupBackgroundRun{Status: "applied", AppliedFiles: []string{"AGENTS.md", ".harness/harness.yaml"}}
	m.focusNextStep()
	view := m.View().Content
	if !strings.Contains(view, "COMPLETE") || !strings.Contains(view, "AGENTS.md") || !strings.Contains(view, "Finish") || strings.Contains(view, "Review proposal") {
		t.Fatalf("ambiguous completed state: %s", view)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.review {
		t.Fatal("completion must finish, not reopen review")
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
