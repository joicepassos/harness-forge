package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/doctor/application"
	"harnessforge/internal/doctor/infrastructure"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"path/filepath"
)

func newDoctorCommand() *cobra.Command {
	var repository string
	var layout string
	var fix bool
	cmd := &cobra.Command{Use: "doctor [file]", Short: "Diagnose harness health without modifying files", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) == 1 {
			path = args[0]
		} else {
			root := repository
			if root == "" {
				root = "."
			}
			projectLayout, err := harnessinfra.ResolveLayout(root, layout)
			if err != nil {
				return err
			}
			if projectLayout.Kind == harnessinfra.LayoutForge {
				path = projectLayout.ManifestPath
			} else {
				path = projectLayout.HarnessPath
			}
			if repository == "" {
				repository, _ = filepath.Abs(root)
			}
		}
		report, err := application.NewDiagnose(infrastructure.LocalSource{}).Execute(cmd.Context(), path, repository)
		if err != nil {
			return err
		}
		if fix {
			out := cmd.ErrOrStderr()
			if _, err := fmt.Fprintln(out, presentationFor(out).status("warning", "Proposals only: review and apply any changes manually.")); err != nil {
				return err
			}
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "Verify evidence against this repository without modifying it")
	cmd.Flags().StringVar(&layout, "layout", "", "Select harness or forge when both project layouts exist")
	cmd.Flags().BoolVar(&fix, "fix", false, "Print reviewable correction proposals without applying them")
	return cmd
}
