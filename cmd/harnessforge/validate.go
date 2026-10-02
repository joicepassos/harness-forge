package main

import (
	"github.com/spf13/cobra"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/infrastructure"
)

func newValidateCommand() *cobra.Command {
	var repository, layout string
	command := &cobra.Command{Use: "validate [file]", Short: "Validate a manually edited Harness IR YAML file", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		l, err := localizerFor(cmd)
		if err != nil {
			return err
		}
		root := repository
		if root == "" {
			root = "."
		}
		path := ""
		harnessConfig := len(args) > 0
		var projectConfig *infrastructure.ProjectConfig
		if len(args) > 0 {
			path = args[0]
		} else {
			project, err := infrastructure.LoadProject(root, layout)
			if err != nil {
				return err
			}
			projectConfig = &project
			if project.Layout.Kind == infrastructure.LayoutHarness {
				path = project.Layout.HarnessPath
				harnessConfig = true
			} else {
				path = project.Layout.ManifestPath
			}
		}
		if harnessConfig {
			if err := application.NewValidate(infrastructure.YAMLLoader{}).Execute(path); err != nil {
				return err
			}
		}
		if projectConfig != nil && projectConfig.Manifest != nil {
			if err := infrastructure.ValidateManifestReferences(projectConfig.Layout, *projectConfig.Manifest); err != nil {
				return err
			}
		}
		if repository != "" && harnessConfig {
			h, err := (infrastructure.YAMLLoader{}).Load(path)
			if err != nil {
				return err
			}
			if err := infrastructure.CheckEvidence(cmd.Context(), root, h); err != nil {
				return err
			}
		}
		return l.printf(cmd, "output.valid", path)
	}}
	command.Flags().StringVar(&repository, "repository", "", "Verify evidence files and literal symbols in this repository")
	command.Flags().StringVar(&layout, "layout", "", "Select harness or forge when both project layouts exist")
	return command
}
