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

// newTUICommand provides a guided terminal workflow. It deliberately uses
// ordinary input and output rather than terminal escape sequences, so it also
// works in SSH sessions, CI logs, and accessibility tools.
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
	fmt.Fprintln(output, "HarnessForge guided terminal interface")
	fmt.Fprintf(output, "Repository: %s\n", root)
	fmt.Fprintln(output, "Choose an action. Changes always require confirmation.")

	for {
		fmt.Fprint(output, "\n1) Create harness  2) Validate harness  3) Generate AGENTS.md  4) Generate CLAUDE.md  5) Exit\nChoice [5]: ")
		choice, err := readTUIInput(reader)
		if err == io.EOF {
			fmt.Fprintln(output, "\nNo choice received; exiting without changes.")
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
			confirmed, err := confirmTUIAction(reader, output, "Create .harness/harness.yaml")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			path, err := harness.Init(root)
			if err != nil {
				fmt.Fprintf(output, "Could not create harness: %v\n", err)
				continue
			}
			fmt.Fprintf(output, "Created %s. Review it before generating instructions.\n", path)
		case "2":
			path := filepath.Join(root, ".harness", "harness.yaml")
			if err := harnessapp.NewValidate(harnessinfra.YAMLLoader{}).Execute(path); err != nil {
				fmt.Fprintf(output, "Harness is not valid: %v\n", err)
				continue
			}
			fmt.Fprintln(output, "Harness is valid.")
		case "3", "4":
			agent := "codex"
			file := "AGENTS.md"
			if choice == "4" {
				agent, file = "claude", "CLAUDE.md"
			}
			confirmed, err := confirmTUIAction(reader, output, "Generate "+file+" from approved rules")
			if err != nil {
				return err
			}
			if !confirmed {
				continue
			}
			harnessPath := filepath.Join(root, ".harness", "harness.yaml")
			if err := application.NewGenerate(harnessinfra.YAMLLoader{}, generationinfra.Markdown{Agent: agent}, generationinfra.FileWriter{}).Execute(context.Background(), harnessPath, root); err != nil {
				fmt.Fprintf(output, "Could not generate %s: %v\n", file, err)
				continue
			}
			fmt.Fprintf(output, "Generated %s. Review the file before committing it.\n", file)
		default:
			fmt.Fprintln(output, "Unknown choice. Enter 1, 2, 3, 4, or 5.")
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
