package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"harnessforge/internal/drift/application"
	driftinfra "harnessforge/internal/drift/infrastructure"
	"harnessforge/internal/harness/infrastructure"
)

func newDriftCommand() *cobra.Command {
	var repository string
	command := &cobra.Command{Use: "drift [file]", Short: "Compare approved harness rules with a repository without changing either", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ".harness/harness.yaml"
		if len(args) == 1 {
			path = args[0]
		}
		reader, err := driftinfra.NewFileReader(repository)
		if err != nil {
			return err
		}
		report, err := application.NewDetect(infrastructure.YAMLLoader{}, reader).Execute(cmd.Context(), path)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}}
	command.Flags().StringVar(&repository, "repository", ".", "Repository to inspect without modifying it")
	return command
}
