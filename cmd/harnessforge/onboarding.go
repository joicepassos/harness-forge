package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/llm"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type setupProposer func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error)

func newInitCommand() *cobra.Command {
	var repository string
	command := &cobra.Command{
		Use:   "init",
		Short: "Analyze and configure this project with a guided setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGuidedInit(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), repository, requestSetupProposal)
		},
	}
	command.Flags().StringVar(&repository, "repository", ".", "Project directory to configure")
	return command
}

func runGuidedInit(ctx context.Context, input io.Reader, output io.Writer, repository string, propose setupProposer) (err error) {
	style := presentationFor(output)
	defer func() {
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(output, style.status("warning", "Setup cancelled; no project files changed."))
			err = nil
		}
	}()
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("project must be a non-symlink directory")
	}
	if existing, readErr := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml")); readErr == nil && !starterHarness(existing) {
		fmt.Fprintln(output, style.status("warning", "This project already has a configured harness. Nothing was changed. Use `harnessforge doctor` to inspect it or `harnessforge generate codex` after reviewing rules."))
		return nil
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintf(output, "%s\n%s\nProject: %s\n\n%s\n\n", style.brand(), style.heading("HarnessForge setup"), root, style.heading("Analyzing project..."))
	analysis, err := analyzer.AnalyzeWithOptions(ctx, root, analyzer.Options{})
	if err != nil {
		return err
	}
	if style.colorful {
		var report bytes.Buffer
		analyzer.Print(&report, analysis)
		fmt.Fprint(output, style.analysis(report.String()))
	} else {
		analyzer.Print(output, analysis)
	}
	printSetupDirectories(output, root)
	fmt.Fprintln(output, style.status("info", "No project files have been changed."))
	session := setupSession{reader: bufio.NewReader(input), output: output}
	documents := defaultSetupDocuments(ctx, root)
	if len(documents) > 0 {
		fmt.Fprintln(output, "\n"+style.heading("Context detected automatically:"))
		for _, document := range documents {
			fmt.Fprintf(output, "- %s\n", document.Source)
		}
	}
	documents, err = session.selectedDocuments(ctx, root, documents)
	if err != nil {
		return err
	}
	if len(documents) > 0 {
		fmt.Fprintln(output, style.heading("Selected context:"))
		for _, document := range documents {
			label := document.Source
			if document.Truncated {
				label += " (first 16 KiB only)"
			}
			fmt.Fprintf(output, "- %s\n", label)
		}
	}
	notes, err := session.notes()
	if err != nil {
		return err
	}
	if err := setupContextBytes(documents, notes); err != nil {
		return err
	}
	config, useAI, err := askSetupProvider(session, input)
	if err != nil {
		return err
	}
	suggestion := setupAIProposal{}
	if useAI {
		fmt.Fprintf(output, "\nThe selected project analysis, %d document(s), and your observations will be sent to %s (%s). API keys are never written to project files.\n", len(documents), config.Name, config.Model)
		allowed, err := session.confirm("Send this context to the AI provider?")
		if err != nil {
			return err
		}
		if !allowed {
			fmt.Fprintln(output, style.status("warning", "AI call cancelled; continuing with a local proposal."))
			useAI = false
			config = setupProvider{}
		}
	}
	if useAI {
		if err := ensureSetupKey(session, input, &config); err != nil {
			return err
		}
		fmt.Fprintln(output, "Preparing the proposal with the selected context...")
		suggestion, err = propose(ctx, config, analysis, documents, notes)
		if err != nil {
			fmt.Fprintln(output, style.status("error", fmt.Sprintf("AI proposal could not be validated: %v", err)))
			continueLocal, askErr := session.confirm("Continue with a local proposal?")
			if askErr != nil {
				return askErr
			}
			if !continueLocal {
				return fmt.Errorf("setup stopped before changing project files")
			}
			config, suggestion = setupProvider{}, setupAIProposal{}
		}
	}
	agents, err := askSetupAgents(session)
	if err != nil {
		return err
	}
	plan, err := buildSetupPlan(root, analysis, documents, notes, config, agents, suggestion)
	if err != nil {
		return err
	}
	showSetupPlan(output, plan)
	approved, err := session.confirm("Create or update exactly these files?")
	if err != nil {
		return err
	}
	if !approved {
		fmt.Fprintln(output, style.status("warning", "Setup cancelled; no project files changed."))
		return nil
	}
	if err := writeSetupPlan(ctx, root, plan.Files); err != nil {
		return err
	}
	fmt.Fprintln(output, style.status("success", "Setup complete. Review the generated files before committing them."))
	return nil
}

func printSetupDirectories(output io.Writer, root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && len(directories) < 20 {
			directories = append(directories, entry.Name()+"/")
		}
	}
	if len(directories) > 0 {
		fmt.Fprintf(output, "Top-level directories: %s\n\n", strings.Join(directories, ", "))
	}
}

func askSetupProvider(session setupSession, _ io.Reader) (setupProvider, bool, error) {
	answer, err := session.ask("Use AI for a tailored setup proposal? [Y/n]: ")
	if err != nil {
		return setupProvider{}, false, err
	}
	if strings.EqualFold(answer, "n") || strings.EqualFold(answer, "no") {
		return setupProvider{}, false, nil
	}
	name, err := session.ask("Provider [openai/deepseek/gemini/groq/ollama] (default openai): ")
	if err != nil {
		return setupProvider{}, false, err
	}
	if name == "" {
		name = "openai"
	}
	name = strings.ToLower(name)
	variable := setupKeyVariable(name)
	if variable == "?" {
		return setupProvider{}, false, fmt.Errorf("unsupported provider %q", name)
	}
	defaultModel := llm.DefaultModel(name)
	model, err := session.ask(fmt.Sprintf("Model (default %s): ", defaultModel))
	if err != nil {
		return setupProvider{}, false, err
	}
	if model == "" {
		model = defaultModel
	}
	if model == "" {
		return setupProvider{}, false, fmt.Errorf("a model name is required for %s", name)
	}
	config := setupProvider{Name: name, Model: model}
	return config, true, nil
}

func ensureSetupKey(session setupSession, input io.Reader, config *setupProvider) error {
	variable := setupKeyVariable(config.Name)
	if variable != "" && os.Getenv(variable) == "" {
		terminal, ok := input.(*os.File)
		if !ok || terminal != os.Stdin || !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("%s is missing; set it in the environment before a scripted AI setup", variable)
		}
		fmt.Fprintf(session.output, "%s (hidden input, used only for this run): ", variable)
		secret, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(session.output)
		if err != nil {
			return err
		}
		config.Key = strings.TrimSpace(string(secret))
		for index := range secret {
			secret[index] = 0
		}
		if config.Key == "" {
			return fmt.Errorf("API key was empty")
		}
	}
	return nil
}

func setupKeyVariable(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "ollama":
		return ""
	default:
		return "?"
	}
}

func askSetupAgents(session setupSession) ([]string, error) {
	answer, err := session.ask("Agent instructions [1 Codex, 2 Claude, 3 both] (default 1): ")
	if err != nil {
		return nil, err
	}
	switch answer {
	case "", "1":
		return []string{"codex"}, nil
	case "2":
		return []string{"claude"}, nil
	case "3":
		return []string{"codex", "claude"}, nil
	default:
		return nil, fmt.Errorf("choose 1, 2, or 3")
	}
}

func showSetupPlan(output io.Writer, plan setupPlan) {
	fmt.Fprintln(output, "\n"+presentationFor(output).heading("Proposed setup (nothing has been written):"))
	if plan.Summary != "" {
		fmt.Fprintf(output, "AI summary: %s\n", plan.Summary)
	}
	fmt.Fprintf(output, "Languages: %s\n", strings.Join(plan.Harness.Project.Languages, ", "))
	fmt.Fprintf(output, "Architecture: %s\n", strings.Join(plan.Harness.Architecture.Styles, ", "))
	fmt.Fprintf(output, "Rules: %d; skills: %d; context documents: %d\n", len(plan.Harness.Rules), len(plan.Harness.Skills), len(plan.Documents))
	for _, file := range plan.Files {
		action := "create"
		if file.Append {
			action = "append to existing instructions"
		} else if file.Replace {
			action = "replace starter/generated file"
		}
		fmt.Fprintf(output, "\n--- %s (%s) ---\n%s\n", file.Path, action, file.Content)
	}
}
