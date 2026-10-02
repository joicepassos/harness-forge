package main

import (
	"context"
	"fmt"
	"harnessforge/internal/analyzer"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type setupStageKey struct{}
type setupStage string

func reportSetupStage(ctx context.Context, stage string) {
	if setupStageLabel(setupStage(stage)) == "" {
		return
	}
	if report, ok := ctx.Value(setupStageKey{}).(func(setupStage)); ok {
		report(setupStage(stage))
	}
}

func setupStageLabel(stage setupStage) string {
	switch stage {
	case "context_prepared":
		return "Context prepared."
	case "waiting_provider":
		return "Waiting for provider response..."
	case "response_received":
		return "Provider response received."
	case "validating_proposal":
		return "Validating the AI proposal..."
	default:
		return ""
	}
}

type setupProposalOutcome struct {
	proposal setupAIProposal
	err      error
}
type setupElapsedMsg time.Time

type setupProgressModel struct {
	session              setupSession
	started              time.Time
	spinner              spinner.Model
	phase                string
	history              []string
	width                int
	result               <-chan setupProposalOutcome
	completed, cancelled bool
	cancel               context.CancelFunc
	outcome              setupProposalOutcome
	provider, model      string
	documents            []setupDocument
}

func setupElapsedTick() tea.Cmd {
	return tea.Tick(time.Second, func(now time.Time) tea.Msg { return setupElapsedMsg(now) })
}

func (m *setupProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, setupElapsedTick(), func() tea.Msg { return <-m.result })
}

func (m *setupProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setupStage:
		if label := setupStageLabel(msg); label != "" {
			m.phase = label
			m.history = append(m.history, m.session.uiText(label))
		}
		return m, nil
	case setupProposalOutcome:
		m.outcome, m.completed = msg, true
		return m, tea.Quit
	case setupElapsedMsg:
		return m, setupElapsedTick()
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.cancelled = true
			m.phase = "Cancelling the AI request..."
			m.cancel()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *setupProgressModel) View() tea.View {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	lines := []string{accent.Render("HarnessForge / " + m.session.uiText("Call inspector")), ""}
	lines = append(lines, fmt.Sprintf("%s %s | %.0fs", m.spinner.View(), m.session.uiText(m.phase), time.Since(m.started).Seconds()), "")
	lines = append(lines, m.session.uiText("Provider")+"  "+setupInspectorSafe(m.provider), m.session.uiText("Model")+"    "+setupInspectorSafe(m.model), fmt.Sprintf("%s  %d", m.session.uiText("Selected context"), len(m.documents)), "")
	for _, doc := range m.documents {
		lines = append(lines, "  "+setupInspectorSafe(doc.Source))
	}
	lines = append(lines, "", muted.Render(m.session.uiText("Real events")))
	lines = append(lines, m.history...)
	if m.completed && m.outcome.err == nil && !m.cancelled {
		lines = append(lines, m.session.uiText("AI proposal ready for review."))
	}
	lines = append(lines, "", muted.Render(m.session.uiText("Esc: cancel request")))
	return tea.NewView(lipgloss.NewStyle().Width(max(1, m.width)).Render(strings.Join(lines, "\n")))
}

func (s setupSession) generateProposal(ctx context.Context, config setupProvider, analysis *analyzer.Analysis, documents []setupDocument, notes string, propose setupProposer) (setupAIProposal, error) {
	if !s.interactive {
		fmt.Fprintln(s.output, "Preparing selected context...")
		ctx = context.WithValue(ctx, setupStageKey{}, func(stage setupStage) { fmt.Fprintln(s.output, setupStageLabel(stage)) })
		return propose(ctx, config, analysis, documents, notes)
	}
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan setupProposalOutcome, 1)
	finished := make(chan struct{})
	activity := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := &setupProgressModel{session: s, started: time.Now(), spinner: activity, phase: "Preparing selected context...", width: 80, result: result, cancel: cancel, provider: config.Name, model: config.Model, documents: documents}
	program := tea.NewProgram(m, tea.WithInput(s.input), tea.WithOutput(s.uiOutput()), tea.WithContext(ctx))
	requestCtx = context.WithValue(requestCtx, setupStageKey{}, func(stage setupStage) { program.Send(stage) })
	go func() {
		defer close(finished)
		proposal, err := propose(requestCtx, config, analysis, documents, notes)
		result <- setupProposalOutcome{proposal: proposal, err: err}
	}()
	_, err := program.Run()
	cancel()
	<-finished
	if m.cancelled {
		return setupAIProposal{}, io.EOF
	}
	if ctx.Err() != nil {
		return setupAIProposal{}, ctx.Err()
	}
	if err != nil {
		return setupAIProposal{}, err
	}
	return m.outcome.proposal, m.outcome.err
}
