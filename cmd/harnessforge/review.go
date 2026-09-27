package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
)

func newReviewCommand() *cobra.Command {
	var path, repository string
	cmd := &cobra.Command{Use: "review <rule-id> <candidate|approved|rejected>", Short: "Review a rule while preserving manual YAML formatting", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root, err := filepath.Abs(filepath.Join(cwd, repository))
		if err != nil {
			return err
		}
		if err := application.NewReview(infrastructure.YAMLRuleStore{}, infrastructure.EvidenceRevalidator{Root: root}).Execute(path, args[0], args[1]); err != nil {
			return err
		}
		message := fmt.Sprintf("Rule %s: %s", args[0], args[1])
		_, err = fmt.Fprintln(cmd.OutOrStdout(), presentationFor(cmd.OutOrStdout()).status("success", message))
		return err
	}}
	cmd.Flags().StringVar(&path, "file", ".harness/harness.yaml", "Harness YAML file")
	cmd.Flags().StringVar(&repository, "repository", ".", "Repository root containing evidence files")
	return cmd
}
