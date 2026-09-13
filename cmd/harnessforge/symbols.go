package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"harnessforge/internal/symbols/application"
	"harnessforge/internal/symbols/infrastructure"
	"os"
)

func newSymbolsCommand() *cobra.Command {
	var language string
	command := &cobra.Command{Use: "symbols [file]", Short: "Extract structural code symbols", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		report, err := application.NewExtract(infrastructure.Registry{}).Execute(cmd.Context(), language, args[0], data)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}}
	command.Flags().StringVar(&language, "source-language", "go", "Source language: go or java")
	return command
}
