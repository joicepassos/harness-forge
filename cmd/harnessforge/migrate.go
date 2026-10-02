package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/harness/infrastructure"
	"path/filepath"
)

func newMigrateCommand() *cobra.Command {
	var output, toLayout, repository, layout, ruleKind, planSHA256 string
	var targets []string
	var dryRun, apply, rollback bool
	command := &cobra.Command{Use: "migrate [file]", Short: "Preview or apply a reversible project-layout migration", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if rollback {
			return application.RollbackForgeMigration(repository)
		}
		if toLayout == "forge" {
			if dryRun == apply {
				return fmt.Errorf("choose exactly one of --dry-run or --apply")
			}
			root := repository
			if root == "" {
				root = "."
			}
			source := ""
			if len(args) == 1 {
				source = args[0]
				if !filepath.IsAbs(source) {
					source = filepath.Join(root, source)
				}
			} else {
				resolved, err := infrastructure.ResolveLayout(root, layout)
				if err != nil {
					return err
				}
				if resolved.Kind != infrastructure.LayoutHarness {
					return fmt.Errorf("migration source must use the harness layout")
				}
				source = resolved.HarnessPath
			}
			kind := domain.KnowledgeKind(ruleKind)
			plan, err := application.PreviewToForge(infrastructure.YAMLLoader{}, root, source, targets, kind)
			if err != nil {
				return err
			}
			if dryRun {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(plan)
			}
			if planSHA256 == "" {
				return fmt.Errorf("--plan-sha256 is required; run --dry-run and pass its plan_sha256 value")
			}
			if planSHA256 != plan.PlanSHA256 {
				return fmt.Errorf("migration plan digest mismatch; create a new preview")
			}
			if err := application.ApplyForgeMigration(root, plan, planSHA256); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Migrated to .forge; original %s was preserved. Use --rollback to reverse this migration.\n", plan.SourcePath)
			return err
		}
		if dryRun || apply || len(args) != 1 {
			return fmt.Errorf("schema migration requires `migrate <file> --output <file>`; use --to-layout forge for layout migration")
		}
		if output == "" {
			return fmt.Errorf("--output is required; migration never overwrites the source")
		}
		return application.MigrateV1ToV2(infrastructure.YAMLLoader{}, args[0], output)
	}}
	command.Flags().StringVar(&output, "output", "", "Destination file for the migrated v2 harness")
	command.Flags().StringVar(&toLayout, "to-layout", "", "Target layout (forge)")
	command.Flags().StringVar(&repository, "repository", ".", "Project root")
	command.Flags().StringVar(&layout, "layout", "", "Select harness when both layouts exist")
	command.Flags().StringSliceVar(&targets, "target", nil, "Target agent for the migrated layout (repeatable)")
	command.Flags().StringVar(&ruleKind, "rule-kind", "", "Knowledge type to assign to legacy rules when their type is unknown")
	command.Flags().StringVar(&planSHA256, "plan-sha256", "", "Required digest from the reviewed --dry-run plan")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show a deterministic migration preview without writing")
	command.Flags().BoolVar(&apply, "apply", false, "Apply the reviewed migration plan")
	command.Flags().BoolVar(&rollback, "rollback", false, "Remove a migration-created layout only when all owned files remain intact")
	return command
}
