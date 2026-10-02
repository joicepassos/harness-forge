package main

import (
	"context"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type setupInspectorPoll struct {
	run setupBackgroundRun
	err error
}

type setupInspectorModel struct {
	session       setupSession
	run           setupBackgroundRun
	section       int
	width, height int
	spinner       spinner.Model
	viewport      viewport.Model
	err           error
}

func (s setupSession) inspectorText(text string) string {
	if _, ok := s.output.(setupLocalizedWriter); ok {
		return s.uiText(strings.NewReplacer("Response headers received; reading body", "Cabecalhos recebidos; aguardando corpo da resposta", "Proposal generation failed.", "A geracao da proposta falhou.", "Process cancelled.", "Processo cancelado.", "Proposal applied after confirmation.", "Proposta aplicada apos confirmacao.").Replace(text))
	}
	return strings.NewReplacer("Inspetor de chamadas", "Call inspector", "Contexto autorizado", "Authorized context", "arquivo(s)", "file(s)", "Conteudo e credenciais ocultos.", "Contents and credentials hidden.", "Validacao concluida", "Validation complete", "Validacao", "Validation", "regra(s) com evidencia valida", "rule(s) with valid evidence", "habilidade(s) com evidencia valida", "skill(s) with valid evidence", "Descartadas:", "Discarded:", "regra(s)", "rule(s)", "habilidade(s)", "skill(s)", "Citacoes invalidas nao foram aceitas.", "Invalid citations were not accepted.", "Proposta disponivel para revisao", "Proposal ready for review", "Proposta", "Proposal", "Ainda nao disponivel para revisao.", "Not ready for review yet.", "Nenhum arquivo e gravado por este inspetor.", "This inspector does not write any project files.", "Revisar e confirmar:", "Review and confirm:", "Chamada de geracao", "Generation call", "Provedor", "Provider", "Modelo", "Model", "Estado", "Status", "Eventos reais", "Actual events", "Falha:", "Failure:", "A proposta local continua disponivel pelo init.", "A local proposal remains available through init.", "Contexto", "Context", "Chamadas", "Calls", "Tab: secao | setas: rolar | d/q: soltar terminal | c: cancelar", "Tab: section | arrows: scroll | d/q: detach | c: cancel", "Falha ao atualizar o processo; tente abrir o inspetor novamente.", "Could not refresh the process; reopen the inspector.").Replace(text)
}

func setupInspectorSafe(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func (s setupSession) inspectorRunText(text string, run setupBackgroundRun) string {
	values := []string{run.Root, run.ID, run.Provider, run.Model, run.Error}
	for _, doc := range run.Documents {
		values = append(values, doc.Path)
	}
	for i, value := range values {
		value = setupInspectorSafe(value)
		values[i] = value
		if value != "" {
			text = strings.ReplaceAll(text, value, fmt.Sprintf("__HF_META_%d__", i))
		}
	}
	text = s.inspectorText(text)
	for i, value := range values {
		if value != "" {
			text = strings.ReplaceAll(text, fmt.Sprintf("__HF_META_%d__", i), value)
		}
	}
	return text
}

func setupInspectorActive(status string) bool { return status == "queued" || status == "running" }

func (s setupSession) inspectorStatus(status string) string {
	labels := map[string]string{"queued": "Queued", "running": "Running", "ready": "Ready for review", "failed": "Failed", "cancelled": "Cancelled", "applied": "Applied"}
	if _, ok := s.output.(setupLocalizedWriter); ok {
		labels = map[string]string{"queued": "Na fila", "running": "Em execucao", "ready": "Pronta para revisao", "failed": "Falhou", "cancelled": "Cancelado", "applied": "Aplicada"}
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return setupInspectorSafe(status)
}

func (m *setupInspectorModel) poll() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		run, err := loadSetupBackgroundRun(m.run.ID)
		return setupInspectorPoll{run, err}
	})
}

func (m *setupInspectorModel) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, m.poll()) }

func (m *setupInspectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case setupInspectorPoll:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.run = msg.run
		}
		return m, m.poll()
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "d", "esc", "ctrl+c":
			return m, tea.Quit
		case "c":
			if setupInspectorActive(m.run.Status) {
				m.err = cancelSetupBackgroundRun(m.run.ID)
			}
			return m, nil
		case "tab", "right":
			m.section = (m.section + 1) % 4
			m.viewport.GotoTop()
			return m, nil
		case "shift+tab", "left":
			m.section = (m.section + 3) % 4
			m.viewport.GotoTop()
			return m, nil
		case "1", "2", "3", "4":
			m.section = int(msg.String()[0] - '1')
			m.viewport.GotoTop()
			return m, nil
		}
	}
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func setupInspectorDetails(run setupBackgroundRun, section int, sessions ...setupSession) string {
	s := setupSession{}
	if len(sessions) > 0 {
		s = sessions[0]
	}
	switch section {
	case 0:
		lines := []string{fmt.Sprintf("Contexto autorizado | %d arquivo(s)", len(run.Documents)), ""}
		for _, doc := range run.Documents {
			lines = append(lines, "  "+setupInspectorSafe(doc.Path))
		}
		lines = append(lines, "", "Conteudo e credenciais ocultos.")
		return strings.Join(lines, "\n")
	case 2:
		if run.Proposal == nil {
			return "Validacao\n\n" + setupInspectorSafe(setupStageLabel(setupStage(run.Stage)))
		}
		return fmt.Sprintf("Validacao concluida\n\n%d regra(s) com evidencia valida\n%d habilidade(s) com evidencia valida\n\nDescartadas: %d regra(s), %d habilidade(s).\nCitacoes invalidas nao foram aceitas.", len(run.Proposal.Rules), len(run.Proposal.Skills), run.DiscardedRules, run.DiscardedSkills)
	case 3:
		if run.Status == "applied" {
			return "Proposal applied after confirmation."
		}
		if run.Proposal == nil {
			return "Proposta\n\nAinda nao disponivel para revisao."
		}
		return fmt.Sprintf("Proposta disponivel para revisao\n\n%d regra(s) | %d habilidade(s)\n\nNenhum arquivo e gravado por este inspetor.\n\nRevisar e confirmar:\nharnessforge init resume %s", len(run.Proposal.Rules), len(run.Proposal.Skills), setupInspectorSafe(run.ID))
	default:
		lines := []string{"Chamada de geracao", "", "Provedor  " + setupInspectorSafe(run.Provider), "Modelo    " + setupInspectorSafe(run.Model), "Estado    " + s.inspectorStatus(run.Status), "", "Eventos reais"}
		type entry struct {
			at   time.Time
			text string
		}
		var timeline []entry
		for _, event := range run.Events {
			label := setupStageLabel(setupStage(event.Stage))
			if label == "" {
				label = map[string]string{"ready": "AI proposal ready for review.", "failed": "Proposal generation failed.", "cancelled": "Process cancelled.", "applied": "Proposal applied after confirmation."}[event.Stage]
			}
			if label != "" {
				timeline = append(timeline, entry{event.At, event.At.Format("15:04:05") + "  " + label})
			}
		}
		for _, call := range run.Calls {
			status := ""
			if call.StatusCode > 0 {
				status = fmt.Sprintf(" | HTTP %d", call.StatusCode)
			}
			if call.Duration > 0 {
				status += fmt.Sprintf(" | %dms", call.Duration.Milliseconds())
			}
			if call.RetryDelay > 0 {
				status += fmt.Sprintf(" | retry in %s", call.RetryDelay)
			}
			kind := setupInspectorSafe(call.Kind)
			if call.Kind == "response" {
				kind = "Response headers received; reading body"
			}
			timeline = append(timeline, entry{call.At, fmt.Sprintf("%s  #%d %s %s | %s%s", call.At.Format("15:04:05"), call.Attempt, setupInspectorSafe(call.Method), setupInspectorSafe(call.Operation), kind, status)})
		}
		sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].at.Before(timeline[j].at) })
		for _, event := range timeline {
			lines = append(lines, event.text)
		}
		if run.Error != "" {
			lines = append(lines, "", "Falha: "+setupInspectorSafe(run.Error), "A proposta local continua disponivel pelo init.", "harnessforge init resume "+setupInspectorSafe(run.ID))
		}
		return strings.Join(lines, "\n")
	}
}

func (m *setupInspectorModel) View() tea.View {
	width := max(1, m.width)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	header := accent.Render("HarnessForge / Inspetor de chamadas") + "\n" + muted.Render(setupInspectorSafe(m.run.ID)+"  "+setupInspectorSafe(m.run.Root))
	end := time.Now()
	if !setupInspectorActive(m.run.Status) {
		end = m.run.UpdatedAt
	}
	elapsed := max(0, int(end.Sub(m.run.StartedAt).Seconds()))
	activity := ""
	if setupInspectorActive(m.run.Status) {
		activity = m.spinner.View() + " "
	}
	status := fmt.Sprintf("%s%s | %ds", activity, m.session.inspectorStatus(m.run.Status), elapsed)
	labels := []string{"1 Contexto", "2 Chamadas", "3 Validacao", "4 Proposta"}
	for i := range labels {
		if i == m.section {
			labels[i] = accent.Render("> " + labels[i])
		}
	}
	bodyWidth := width
	sidebar := ""
	if width >= 72 && m.height >= 16 {
		bodyWidth = width - 22
		sidebar = lipgloss.NewStyle().Width(20).Render(strings.Join(labels, "\n\n"))
	}
	if sidebar == "" {
		header += "\n" + strings.Join(labels, " | ")
	}
	footer := "Tab: secao | setas: rolar | d/q: soltar terminal | c: cancelar"
	if m.err != nil {
		footer = "Falha ao atualizar o processo; tente abrir o inspetor novamente."
	}
	height := max(1, m.height)
	if width < 48 || height < 16 {
		header = accent.Render("HarnessForge") + "\n" + m.session.inspectorText(labels[m.section])
		footer = "Tab / d / c"
	}
	header = ansi.Hardwrap(m.session.inspectorRunText(header, m.run)+"\n"+status, width, true)
	footer = ansi.Hardwrap(m.session.inspectorText(footer), width, true)
	if lipgloss.Height(header)+lipgloss.Height(footer)+1 >= height {
		header = ansi.Truncate("HarnessForge | "+m.session.inspectorStatus(m.run.Status), width, "")
		footer = ansi.Truncate("Tab / d / c", width, "")
	}
	m.viewport.SetWidth(max(1, bodyWidth))
	m.viewport.SetHeight(max(1, height-lipgloss.Height(header)-lipgloss.Height(footer)))
	m.viewport.SetContent(ansi.Hardwrap(m.session.inspectorRunText(setupInspectorDetails(m.run, m.section, m.session), m.run), max(1, bodyWidth), true))
	body := m.viewport.View()
	if sidebar != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	}
	view := ansi.Hardwrap(header+"\n"+body+"\n"+muted.Render(footer), width, true)
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		view = strings.Join(lines[:height], "\n")
	}
	return tea.NewView(view)
}

func inspectSetupBackgroundRun(ctx context.Context, input io.Reader, output io.Writer, id string, accessible bool) error {
	run, err := loadSetupBackgroundRun(id)
	if err != nil {
		return err
	}
	s := setupSession{input: input, output: output, ctx: ctx}
	if !setupInteractive(input, s.uiOutput(), accessible) {
		fmt.Fprintf(s.uiOutput(), s.inspectorText("HarnessForge / Inspetor de chamadas\n%s | %s\n"), setupInspectorSafe(run.ID), s.inspectorStatus(run.Status))
		for section := 0; section < 4; section++ {
			fmt.Fprintln(s.uiOutput(), s.inspectorRunText(setupInspectorDetails(run, section, s), run))
		}
		return nil
	}
	m := &setupInspectorModel{session: s, run: run, section: 1, width: 80, height: 24, spinner: spinner.New(spinner.WithSpinner(spinner.Dot)), viewport: viewport.New()}
	_, err = tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(s.uiOutput()), tea.WithContext(ctx)).Run()
	return err
}
