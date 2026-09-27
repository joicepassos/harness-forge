package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
	"strings"
)

func newReviewCommand() *cobra.Command {
	var path, repository, layout, reviewer string
	cmd := &cobra.Command{Use: "review <rule-id> <candidate|approved|rejected>", Short: "Review a rule while preserving manual YAML formatting", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		repositoryPath := repository
		if !filepath.IsAbs(repositoryPath) {
			repositoryPath = filepath.Join(cwd, repositoryPath)
		}
		root, err := filepath.Abs(repositoryPath)
		if err != nil {
			return err
		}
		resolved, resolveErr := infrastructure.ResolveLayout(root, layout)
		if resolveErr == nil && resolved.Kind == infrastructure.LayoutForge {
			if err := infrastructure.ReviewKnowledge(root, layout, args[0], args[1], reviewer); err != nil {
				return err
			}
			message := fmt.Sprintf("Knowledge %s: %s", args[0], args[1])
			_, err = fmt.Fprintln(cmd.OutOrStdout(), presentationFor(cmd.OutOrStdout()).status("success", message))
			return err
		}
		if resolveErr != nil && !strings.Contains(resolveErr.Error(), "missing") {
			return resolveErr
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
	cmd.Flags().StringVar(&layout, "layout", "", "Select harness or forge when both project layouts exist")
	cmd.Flags().StringVar(&reviewer, "reviewer", "", "Reviewer identity required for Forge knowledge decisions")
	return cmd
}
