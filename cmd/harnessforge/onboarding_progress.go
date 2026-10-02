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
	lines := append([]string(nil), m.history...)
	if !m.completed {
		lines = append(lines, fmt.Sprintf("%s %s (%.0fs)", m.spinner.View(), m.session.uiText(m.phase), time.Since(m.started).Seconds()))
	} else if m.outcome.err == nil && !m.cancelled {
		lines = append(lines, m.session.uiText("AI proposal ready for review."))
	}
	return tea.NewView(lipgloss.NewStyle().Width(m.width).Render(strings.Join(lines, "\n")))
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
	activity := spinner.New(spinner.WithSpinner(spinner.Line))
	m := &setupProgressModel{session: s, started: time.Now(), spinner: activity, phase: "Preparing selected context...", width: 80, result: result, cancel: cancel}
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
