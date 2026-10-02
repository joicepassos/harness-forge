package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

type setupInspectorPoll struct {
	run setupBackgroundRun
	err error
}

type setupInspectorColorKey struct{}

func setupInspectorColors(ctx context.Context) bool {
	mode := "auto"
	if ctx != nil {
		if value, ok := ctx.Value(setupInspectorColorKey{}).(string); ok {
			mode = value
		}
	}
	return mode == "always" || (mode != "never" && os.Getenv("NO_COLOR") == "" && os.Getenv("CLICOLOR") != "0")
}

type setupInspectorModel struct {
	session       setupSession
	run           setupBackgroundRun
	section       int
	width, height int
	spinner       spinner.Model
	viewport      viewport.Model
	err           error
	focus         int
	action        int
	review        bool
	confirmCancel bool
}

func (m *setupInspectorModel) label(en, pt string) string {
	if _, ok := m.session.output.(setupLocalizedWriter); ok {
		return pt
	}
	return en
}

func (m *setupInspectorModel) canReview() bool {
	return m.run.Status == "ready" || m.run.Status == "failed" || m.run.Status == "cancelled"
}

func (m *setupInspectorModel) focusNextStep() {
	if m.canReview() || m.run.Status == "applied" {
		m.confirmCancel = false
		m.section, m.focus, m.action = 3, 2, 0
		m.viewport.GotoTop()
	}
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
			changed := m.run.Status != msg.run.Status
			m.run = msg.run
			if changed {
				m.focusNextStep()
			}
		}
		return m, m.poll()
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case tea.KeyPressMsg:
		key := msg.String()
		if m.confirmCancel {
			switch key {
			case "enter":
				m.err = cancelSetupBackgroundRun(m.run.ID)
				m.confirmCancel = false
			case "esc":
				m.confirmCancel = false
			case "ctrl+c", "d", "q":
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "d", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.focus = 0
			return m, nil
		case "r":
			if m.canReview() {
				m.review = true
				return m, tea.Quit
			}
			return m, nil
		case "c":
			if setupInspectorActive(m.run.Status) {
				m.confirmCancel = true
			}
			return m, nil
		case "tab":
			m.focus = (m.focus + 1) % 3
			return m, nil
		case "shift+tab":
			m.focus = (m.focus + 2) % 3
			return m, nil
		case "enter":
			if m.focus == 0 {
				m.focus = 1
			} else if m.focus == 2 {
				switch m.action {
				case 0:
					if m.run.Status == "applied" {
						return m, tea.Quit
					}
					if m.canReview() {
						m.review = true
						return m, tea.Quit
					}
				case 1:
					if m.run.Status == "applied" {
						m.focus, m.section = 1, 3
						return m, nil
					}
					return m, tea.Quit
				case 2:
					m.confirmCancel = setupInspectorActive(m.run.Status)
				}
			}
			return m, nil
		case "up", "down", "left", "right":
			step := 1
			if key == "up" || key == "left" {
				step = -1
			}
			if m.focus == 0 {
				m.section = (m.section + step + 4) % 4
				m.viewport.GotoTop()
				return m, nil
			}
			if m.focus == 2 {
				count := 3
				if m.run.Status == "applied" {
					count = 2
				}
				m.action = (m.action + step + count) % count
				return m, nil
			}
		case "1", "2", "3", "4":
			m.section = int(msg.String()[0] - '1')
			m.focus = 0
			m.viewport.GotoTop()
			return m, nil
		}
	}
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	if m.focus == 1 {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
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
			lines := []string{"Proposal applied after confirmation.", ""}
			if len(run.AppliedFiles) == 0 {
				lines = append(lines, s.uiText("File list unavailable for this older run."))
			}
			for _, path := range run.AppliedFiles {
				lines = append(lines, "  "+setupInspectorSafe(path))
			}
			return strings.Join(lines, "\n")
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
	height := max(1, m.height)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#55D6BE")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#A3ABB8"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("#90F2DB")).Background(lipgloss.Color("#254039")).Bold(true)
	stateColor := "#74B9FF"
	switch m.run.Status {
	case "ready", "applied":
		stateColor = "#55D6BE"
	case "failed":
		stateColor = "#FF8494"
	case "queued", "cancelled":
		stateColor = "#EDC56F"
	}
	stateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(stateColor)).Bold(true)
	header := accent.Render("HarnessForge") + muted.Render(" / "+m.label("Call inspector", "Inspetor de chamadas")) + "\n" + muted.Render(setupInspectorSafe(m.run.ID)+"  "+setupInspectorSafe(m.run.Root))
	end := time.Now()
	if !setupInspectorActive(m.run.Status) {
		end = m.run.UpdatedAt
	}
	elapsed := max(0, int(end.Sub(m.run.StartedAt).Seconds()))
	activity := ""
	if setupInspectorActive(m.run.Status) {
		activity = m.spinner.View() + " "
	}
	status := stateStyle.Render(fmt.Sprintf("%s%s", activity, m.session.inspectorStatus(m.run.Status))) + muted.Render(fmt.Sprintf("  /  %ds  /  ", elapsed)) + setupInspectorSafe(m.run.Provider) + " / " + setupInspectorSafe(m.run.Model)
	if m.run.Status == "ready" {
		status = stateStyle.Render(m.label("PROPOSAL READY / Next: review proposal", "PROPOSTA PRONTA / Proximo passo: revisar proposta"))
	} else if m.run.Status == "applied" {
		status = stateStyle.Render(m.label("COMPLETE / Files saved / Next: finish", "CONCLUIDO / Arquivos gravados / Proximo passo: encerrar"))
	}
	labels := []string{m.label("Context", "Contexto"), m.label("Calls", "Chamadas"), m.label("Validation", "Validacao"), m.label("Proposal", "Proposta")}
	for i := range labels {
		labels[i] = fmt.Sprintf("%d  %s", i+1, labels[i])
		if i == m.section {
			labels[i] = selected.Width(19).Render("> " + labels[i])
		} else {
			labels[i] = muted.Render("  " + labels[i])
		}
	}
	bodyWidth := width
	sidebar := ""
	if width >= 72 && height >= 16 {
		bodyWidth = width - 22
		sidebar = lipgloss.NewStyle().Width(21).Render(accent.Render(m.label("SECTIONS", "SECOES")) + "\n\n" + strings.Join(labels, "\n\n"))
	}
	if sidebar == "" {
		header += "\n" + strings.Join(labels, " | ")
	}
	actions := []string{m.label("Review proposal", "Revisar proposta"), m.label("Detach", "Soltar terminal"), m.label("Cancel", "Cancelar")}
	if m.run.Status == "applied" {
		actions = []string{m.label("Finish", "Encerrar"), m.label("View saved files", "Ver arquivos gravados"), ""}
	}
	for i, action := range actions {
		style := muted
		if (i == 0 && m.canReview()) || (i == 2 && setupInspectorActive(m.run.Status)) {
			style = accent
		}
		if m.focus == 2 && i == m.action {
			style = selected
		}
		actions[i] = style.Padding(0, 1).Render(action)
	}
	footer := strings.Join(actions, "  ") + "\n" + muted.Render(m.label("Arrows: select | Enter: open | Tab: focus | Esc: sections | r: review | d: detach", "Setas: selecionar | Enter: abrir | Tab: foco | Esc: secoes | r: revisar | d: soltar"))
	if m.focus == 1 {
		footer = strings.Join(actions, "  ") + "\n" + muted.Render(m.label("DETAILS / arrows: scroll | Esc: sections | Tab: actions", "DETALHES / setas: rolar | Esc: secoes | Tab: acoes"))
	}
	if m.focus == 2 && (m.run.Status == "ready" || m.run.Status == "applied") {
		footer = strings.Join(actions, "  ") + "\n" + muted.Render(m.label("Enter: continue with the highlighted action | arrows: choose", "Enter: continuar com a acao destacada | setas: escolher"))
	}
	if m.confirmCancel {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("#EDC56F")).Bold(true).Render(m.label("Cancel the provider request? Enter: confirm / Esc: keep running", "Cancelar a chamada ao provedor? Enter: confirmar / Esc: continuar"))
	}
	if m.err != nil {
		footer = "Falha ao atualizar o processo; tente abrir o inspetor novamente."
	}
	if width < 48 || height < 16 {
		header = accent.Render("HarnessForge") + "\n" + ansi.Truncate(status, width, "") + "\n" + ansi.Truncate(labels[m.section], width, "")
		footer = m.label("Enter / Tab / Esc / r / d", "Enter / Tab / Esc / r / d")
		if m.focus == 2 && m.run.Status == "ready" {
			footer = m.label("Enter: review", "Enter: revisar")
		} else if m.focus == 2 && m.run.Status == "applied" {
			footer = m.label("Enter: finish", "Enter: encerrar")
		}
		if m.confirmCancel {
			footer = m.label("Cancel? Enter / Esc", "Cancelar? Enter / Esc")
		}
	}
	if width >= 48 && height >= 16 {
		header += "\n" + status
	}
	header = ansi.Hardwrap(header, width, true)
	footer = ansi.Hardwrap(footer, width, true)
	if lipgloss.Height(header)+lipgloss.Height(footer)+1 >= height {
		header = ansi.Truncate("HarnessForge | "+m.session.inspectorStatus(m.run.Status), width, "")
		footer = ansi.Truncate("Enter / Esc / d", width, "")
	}
	m.viewport.SetWidth(max(1, bodyWidth))
	m.viewport.SetHeight(max(1, height-lipgloss.Height(header)-lipgloss.Height(footer)-1))
	details := strings.Split(m.session.inspectorRunText(setupInspectorDetails(m.run, m.section, m.session), m.run), "\n")
	if m.section == 3 && m.run.Status == "ready" && m.run.Proposal != nil {
		details = []string{m.label("Ready for your review", "Pronta para sua revisao"), "", fmt.Sprintf(m.label("%d rules / %d skills", "%d regras / %d skills"), len(m.run.Proposal.Rules), len(m.run.Proposal.Skills)), "", m.label("No project files have been changed.", "Nenhum arquivo do projeto foi alterado."), "", m.label("Review the file preview, then apply or go back without changes.", "Confira a previa dos arquivos e escolha aplicar ou voltar sem alterar.")}
	}
	for i, line := range details {
		if i == 0 {
			details[i] = accent.Render(line)
		} else if strings.Contains(line, "HTTP ") || strings.Contains(line, "#") {
			details[i] = lipgloss.NewStyle().Foreground(lipgloss.Color("#74B9FF")).Render(line)
		}
	}
	m.viewport.SetContent(ansi.Hardwrap(strings.Join(details, "\n"), max(1, bodyWidth), true))
	body := m.viewport.View()
	if sidebar != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, body)
	}
	view := ansi.Hardwrap(header+"\n"+body+"\n"+footer, width, true)
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		view = strings.Join(lines[:height], "\n")
	}
	result := tea.NewView(view)
	result.AltScreen = true
	if setupInspectorColors(m.session.ctx) {
		result.BackgroundColor = lipgloss.Color("#111315")
		result.ForegroundColor = lipgloss.Color("#E6EAF0")
	}
	return result
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
	for {
		activity := spinner.New(spinner.WithSpinner(spinner.Dot))
		activity.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#55D6BE"))
		m := &setupInspectorModel{session: s, run: run, section: 1, width: 80, height: 24, spinner: activity, viewport: viewport.New()}
		m.focusNextStep()
		options := []tea.ProgramOption{tea.WithInput(input), tea.WithOutput(s.uiOutput()), tea.WithContext(ctx)}
		mode, _ := ctx.Value(setupInspectorColorKey{}).(string)
		if mode == "always" || (runtime.GOOS == "windows" && setupInspectorColors(ctx) && (os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb")) {
			options = append(options, tea.WithColorProfile(colorprofile.ANSI256))
		} else if !setupInspectorColors(ctx) {
			options = append(options, tea.WithColorProfile(colorprofile.NoTTY))
		}
		if _, err = tea.NewProgram(m, options...).Run(); err != nil || !m.review {
			return err
		}
		if err = reviewSetupBackgroundRun(ctx, input, output, id, false, true); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		run, err = loadSetupBackgroundRun(id)
		if err != nil {
			return err
		}
	}
}
