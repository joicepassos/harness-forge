package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/infrastructure"
)

func newValidateCommand() *cobra.Command {
	return &cobra.Command{Use: "validate [file]", Short: "Validate a manually edited Harness IR YAML file", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ".harness/harness.yaml"
		if len(args) > 0 {
			path = args[0]
		}
		if err := application.NewValidate(infrastructure.YAMLLoader{}).Execute(path); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Valid Harness IR: %s\n", path)
		return err
	}}
}
