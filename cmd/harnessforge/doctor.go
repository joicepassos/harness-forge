package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/doctor/application"
	"harnessforge/internal/doctor/infrastructure"
)

func newDoctorCommand() *cobra.Command {
	var repository string
	var fix bool
	cmd := &cobra.Command{Use: "doctor [file]", Short: "Diagnose harness health without modifying files", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ".harness/harness.yaml"
		if len(args) == 1 {
			path = args[0]
		}
		report, err := application.NewDiagnose(infrastructure.LocalSource{}).Execute(cmd.Context(), path, repository)
		if err != nil {
			return err
		}
		if fix {
			fmt.Fprintln(cmd.OutOrStdout(), "Proposals only: review and apply any changes manually.")
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "Verify evidence against this repository without modifying it")
	cmd.Flags().BoolVar(&fix, "fix", false, "Print reviewable correction proposals without applying them")
	return cmd
}
