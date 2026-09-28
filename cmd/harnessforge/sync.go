package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/generation/infrastructure"
)

func newSyncCommand() *cobra.Command {
	var dryRun, check, apply bool
	var repository string
	cmd := &cobra.Command{Use: "sync", Short: "Compile and synchronize generated agent files", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		modes := 0
		mode := "dry-run"
		if dryRun {
			modes++
			mode = "dry-run"
		}
		if check {
			modes++
			mode = "check"
		}
		if apply {
			modes++
			mode = "apply"
		}
		if modes != 1 {
			return fmt.Errorf("choose exactly one of --dry-run, --check, or --apply")
		}
		result, err := infrastructure.SyncForge(cmd.Context(), repository, mode)
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return err
	}}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show generated changes without writing")
	cmd.Flags().BoolVar(&check, "check", false, "Check whether generated files are current")
	cmd.Flags().BoolVar(&apply, "apply", false, "Safely update generated files")
	cmd.Flags().StringVar(&repository, "repository", ".", "Repository to synchronize")
	return cmd
}
