package main

import (
	"github.com/spf13/cobra"
	"harnessforge/internal/generation/application"
	"harnessforge/internal/generation/infrastructure"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

func newGenerateCommand() *cobra.Command {
	var file, repository, layout string
	command := &cobra.Command{Use: "generate [codex|claude]", Short: "Generate reviewed agent instructions", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return application.NewGenerate(harnessinfra.ProjectLoader{Root: repository, Selection: layout}, infrastructure.Markdown{Agent: args[0]}, infrastructure.FileWriter{}, harnessinfra.EvidenceRevalidator{Root: repository}).Execute(cmd.Context(), file, repository)
	}}
	command.Flags().StringVar(&file, "file", "", "Explicit Harness YAML file; otherwise discover the project layout")
	command.Flags().StringVar(&repository, "repository", ".", "Repository output directory")
	command.Flags().StringVar(&layout, "layout", "", "Select harness or forge when both project layouts exist")
	return command
}
