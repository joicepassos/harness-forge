package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

func setupInteractive(input io.Reader, output io.Writer, accessible bool) bool {
	if accessible || os.Getenv("HARNESSFORGE_ACCESSIBLE") != "" {
		return false
	}
	in, okIn := input.(*os.File)
	out, okOut := output.(*os.File)
	return okIn && okOut && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func (s setupSession) uiText(text string) string {
	if _, ok := s.output.(setupLocalizedWriter); ok {
		labels := map[string]string{
			"Yes": "Sim", "No": "Nao", "Provider": "Provedor", "Model": "Modelo", "Languages": "Linguagens",
			"Agent instructions": "Instrucoes dos agentes", "Model identifier": "Identificador do modelo",
			"Additional observations or instructions": "Observacoes ou instrucoes adicionais",
			"A value is required.":                    "Informe um valor.", "Input is too long.": "Entrada muito longa.",
			"Observations exceed 8 KiB.": "Observacoes excedem 8 KiB.", "Select at least one language.": "Selecione pelo menos uma linguagem.",
			"Selected context": "Contexto selecionado", "Context browser": "Navegador de contexto", "Continue": "Continuar",
			"Context exceeds %d files": "Contexto excede %d arquivos", "Selected: %d document(s)": "Selecionados: %d documento(s)",
			"Arrows: navigate | Enter: open/continue | Space: select | Tab: review | p: paths | Esc: cancel": "Setas: navegar | Enter: abrir/continuar | Espaco: marcar | Tab: revisar | p: caminhos | Esc: cancelar",
		}
		if label, ok := labels[text]; ok {
			return label
		}
		return setupPortuguese.Replace(text)
	}
	return text
}

func (s setupSession) uiOutput() io.Writer {
	if localized, ok := s.output.(setupLocalizedWriter); ok {
		return localized.output
	}
	return s.output
}

func (s setupSession) runForm(field huh.Field) error {
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	err := huh.NewForm(huh.NewGroup(field)).WithTheme(setupFormTheme()).WithKeyMap(s.formKeyMap()).WithAccessible(false).WithInput(s.input).WithOutput(s.uiOutput()).RunWithContext(ctx)
	if errors.Is(err, huh.ErrUserAborted) {
		return io.EOF
	}
	return err
}

// Match guided forms to the colors used by the surrounding CLI presentation.
func setupFormTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		styles := huh.ThemeCharm(isDark)
		blue := lipgloss.Color("#00A8E8")
		orange := lipgloss.Color("#FF7A00")
		styles.Focused.Title = styles.Focused.Title.Foreground(blue)
		styles.Focused.NoteTitle = styles.Focused.NoteTitle.Foreground(blue)
		styles.Focused.SelectSelector = styles.Focused.SelectSelector.Foreground(orange)
		styles.Focused.MultiSelectSelector = styles.Focused.MultiSelectSelector.Foreground(orange)
		styles.Focused.FocusedButton = styles.Focused.FocusedButton.Background(orange)
		styles.Blurred.Title = styles.Blurred.Title.Foreground(blue)
		return styles
	})
}

func (s setupSession) formKeyMap() *huh.KeyMap {
	k := huh.NewDefaultKeyMap()
	if _, portuguese := s.output.(setupLocalizedWriter); !portuguese {
		return k
	}
	k.Confirm.Accept.SetKeys("s", "y")
	k.Confirm.Accept.SetHelp("s", "Sim")
	labels := map[string]string{"up": "acima", "down": "abaixo", "filter": "filtrar", "submit": "continuar", "select": "selecionar", "confirm": "confirmar", "toggle": "marcar", "back": "voltar", "next": "continuar", "new line": "nova linha", "select all": "marcar todos", "select none": "desmarcar todos", "set filter": "aplicar filtro", "clear filter": "limpar filtro"}
	bindings := []*key.Binding{&k.Select.Up, &k.Select.Down, &k.Select.Filter, &k.Select.Next, &k.Select.Prev, &k.Select.Submit, &k.Select.SetFilter, &k.Select.ClearFilter, &k.MultiSelect.Up, &k.MultiSelect.Down, &k.MultiSelect.Toggle, &k.MultiSelect.Filter, &k.MultiSelect.Next, &k.MultiSelect.Prev, &k.MultiSelect.Submit, &k.MultiSelect.SelectAll, &k.MultiSelect.SelectNone, &k.Input.Next, &k.Input.Prev, &k.Input.Submit, &k.Text.Next, &k.Text.Prev, &k.Text.Submit, &k.Text.NewLine, &k.Confirm.Next, &k.Confirm.Prev, &k.Confirm.Submit, &k.Confirm.Toggle}
	for _, binding := range bindings {
		help := binding.Help()
		if label, ok := labels[help.Desc]; ok {
			binding.SetHelp(help.Key, label)
		}
	}
	return k
}

func (s setupSession) formSelect(title string, labels, values []string, selected string) (string, error) {
	options := make([]huh.Option[string], len(labels))
	for i, label := range labels {
		options[i] = huh.NewOption(s.uiText(label), values[i])
	}
	err := s.runForm(huh.NewSelect[string]().Title(s.uiText(title)).Options(options...).Value(&selected))
	return selected, err
}

func (s setupSession) formConfirm(title string, selected bool) (bool, error) {
	err := s.runForm(huh.NewConfirm().Title(s.uiText(title)).Affirmative(s.uiText("Yes")).Negative(s.uiText("No")).Value(&selected))
	return selected, err
}

func (s setupSession) formQuestion(question string) (string, error) {
	switch {
	case strings.HasPrefix(question, "Use AI"):
		answer, err := s.formConfirm("Use AI for a tailored setup proposal?", false)
		if answer {
			return "y", err
		}
		return "n", err
	case strings.HasPrefix(question, "Provider ["):
		return s.formSelect("Provider", []string{"OpenAI", "DeepSeek", "Gemini", "Groq", "Ollama"}, []string{"openai", "deepseek", "gemini", "groq", "ollama"}, "openai")
	case strings.HasPrefix(question, "Agent instructions"):
		return s.formSelect("Agent instructions", []string{"Codex / OpenCode (AGENTS.md)", "Claude (CLAUDE.md)", "Codex / OpenCode + Claude"}, []string{"1", "2", "3"}, "1")
	default:
		return s.formInput(strings.TrimSuffix(strings.TrimSpace(question), ":"), false)
	}
}

func (s setupSession) formInput(title string, hidden bool) (string, error) {
	value := ""
	field := huh.NewInput().Title(s.uiText(title)).Value(&value).Validate(func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s", s.uiText("A value is required."))
		}
		if len(value) > 16<<10 {
			return fmt.Errorf("%s", s.uiText("Input is too long."))
		}
		return nil
	})
	if hidden {
		field.EchoMode(huh.EchoModeNone)
	}
	err := s.runForm(field)
	return strings.TrimSpace(value), err
}

func (s setupSession) formKey(variable string, config *setupProvider) error {
	key, err := s.formInput(variable+" (hidden input, used only for this run)", true)
	if err == nil {
		config.Key = key
	}
	return err
}

func (s setupSession) formNotes() (string, error) {
	value := ""
	err := s.runForm(huh.NewText().Title(s.uiText("Additional observations or instructions")).ExternalEditor(false).Value(&value).Validate(func(value string) error {
		if len(value) > setupMaxNotesBytes {
			return fmt.Errorf("%s", s.uiText("Observations exceed 8 KiB."))
		}
		return nil
	}))
	return strings.TrimSpace(value), err
}

func (s setupSession) selectLanguages(choices, selected []string) ([]string, error) {
	choices = append([]string(nil), choices...)
	for _, value := range selected {
		found := false
		for _, option := range choices {
			found = found || option == value
		}
		if !found {
			choices = append(choices, value)
		}
	}
	options := make([]huh.Option[string], len(choices))
	for i, value := range choices {
		options[i] = huh.NewOption(value, value)
	}
	selected = append([]string(nil), selected...)
	err := s.runForm(huh.NewMultiSelect[string]().Title(s.uiText("Languages")).Options(options...).Value(&selected).Validate(func(values []string) error {
		if len(values) == 0 {
			return fmt.Errorf("%s", s.uiText("Select at least one language."))
		}
		return nil
	}))
	return selected, err
}
