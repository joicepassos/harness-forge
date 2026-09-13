package main

import (
	"github.com/spf13/cobra"
	"harnessforge/internal/generation/application"
	"harnessforge/internal/generation/infrastructure"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

func newGenerateCommand() *cobra.Command {
	var file, repository string
	command := &cobra.Command{Use: "generate [codex|claude]", Short: "Generate reviewed agent instructions", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return application.NewGenerate(harnessinfra.YAMLLoader{}, infrastructure.Markdown{Agent: args[0]}, infrastructure.FileWriter{}).Execute(cmd.Context(), file, repository)
	}}
	command.Flags().StringVar(&file, "file", ".harness/harness.yaml", "Harness YAML file")
	command.Flags().StringVar(&repository, "repository", ".", "Repository output directory")
	return command
}
