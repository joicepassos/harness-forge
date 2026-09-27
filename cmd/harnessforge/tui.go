package main

import (
	"bufio"
	"context"
	"fmt"
	"harnessforge/internal/generation/application"
	generationinfra "harnessforge/internal/generation/infrastructure"
	"harnessforge/internal/harness"
	harnessapp "harnessforge/internal/harness/application"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newTUICommand provides a guided terminal workflow with ordinary line input.
// Its presentation falls back to plain text when output is captured or redirected.
func newTUICommand() *cobra.Command {
	return &cobra.Command{
		Use:     "install [repository]",
		Aliases: []string{"tui"},
		Short:   "Set up a harness with guided terminal prompts",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository := "."
			if len(args) == 1 {
				repository = args[0]
			}
			return runTUI(cmd.InOrStdin(), cmd.OutOrStdout(), repository)
		},
	}
}

func runTUI(input io.Reader, output io.Writer, repository string) error {
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("repository must be a directory: %s", repository)
	}

	reader := bufio.NewReader(input)
	style := presentationFor(output)
	fmt.Fprintln(output, style.brand())
	fmt.Fprintln(output, style.heading("Guided setup"))
	fmt.Fprintf(output, "Repository: %s\n", root)
	fmt.Fprintln(output, "Choose an action. Changes always require confirmation.")

	for {
		fmt.Fprintln(output)
		fmt.Fprintln(output, style.heading("Actions"))
		for i, label := range []string{"Create harness", "Validate harness", "Generate AGENTS.md", "Generate CLAUDE.md", "Exit"} {
			fmt.Fprintf(output, "  %s  %s\n", style.accent(fmt.Sprintf("%d)", i+1)), label)
		}
		fmt.Fprint(output, "Choice [5]: ")
		choice, err := readTUIInput(reader)
		if err == io.EOF {
			fmt.Fprintln(output, "\n"+style.status("warning", "No choice received; exiting without changes."))
			return nil
		}
		if err != nil {
			return err
		}

		switch choice {
		case "", "5", "q", "quit", "exit":
			fmt.Fprintln(output, "Goodbye.")
			return nil
		case "1":
			fmt.Fprintln(output, style.heading("Create harness"))
			harnessPath := filepath.Join(root, ".harness", "harness.yaml")
			if _, err := os.Lstat(harnessPath); err == nil {
				fmt.Fprintln(output, style.status("warning", ".harness/harness.yaml already exists; left it unchanged. Choose 2 to validate it."))
				continue
			} else if !os.IsNotExist(err) {
				fmt.Fprintln(output, style.status("error", fmt.Sprintf("Could not inspect harness: %v", err)))
				continue
			}
			confirmed, err := confirmTUIAction(reader, output, "Create .harness/harness.yaml")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			path, err := harness.Init(root)
			if err != nil {
				if os.IsExist(err) {
					fmt.Fprintln(output, style.status("warning", ".harness/harness.yaml already exists; left it unchanged. Choose 2 to validate it."))
					continue
				}
				fmt.Fprintln(output, style.status("error", fmt.Sprintf("Could not create harness: %v", err)))
				continue
			}
			fmt.Fprintln(output, style.status("success", fmt.Sprintf("Created %s. Review it before generating instructions.", path)))
		case "2":
			fmt.Fprintln(output, style.heading("Validate harness"))
			path := filepath.Join(root, ".harness", "harness.yaml")
			if err := harnessapp.NewValidate(harnessinfra.YAMLLoader{}).Execute(path); err != nil {
				fmt.Fprintln(output, style.status("error", fmt.Sprintf("Harness is not valid: %v", err)))
				continue
			}
			fmt.Fprintln(output, style.status("success", "Harness is valid."))
		case "3", "4":
			agent := "codex"
			file := "AGENTS.md"
			if choice == "4" {
				agent, file = "claude", "CLAUDE.md"
			}
			fmt.Fprintln(output, style.heading("Generate "+file))
			confirmed, err := confirmTUIAction(reader, output, "Generate "+file+" from approved rules")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			if err := application.NewGenerate(harnessinfra.ProjectLoader{Root: root}, generationinfra.Markdown{Agent: agent}, generationinfra.FileWriter{}, harnessinfra.EvidenceRevalidator{Root: root}).Execute(context.Background(), "", root); err != nil {
				fmt.Fprintln(output, style.status("error", fmt.Sprintf("Could not generate %s: %v", file, err)))
				continue
			}
			fmt.Fprintln(output, style.status("success", fmt.Sprintf("Generated %s. Review the file before committing it.", file)))
		default:
			fmt.Fprintln(output, style.status("warning", "Unknown choice. Enter 1, 2, 3, 4, or 5."))
		}
	}
}

func readTUIInput(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(line)), nil
}

func confirmTUIAction(reader *bufio.Reader, output io.Writer, action string) (bool, error) {
	fmt.Fprintf(output, "%s? [y/N]: ", action)
	answer, err := readTUIInput(reader)
	if err == io.EOF {
		return false, fmt.Errorf("confirmation required; no changes made")
	}
	if err != nil {
		return false, err
	}
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(output, "Cancelled; no changes made.")
		return false, nil
	}
	return true, nil
}
