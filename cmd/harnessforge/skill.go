package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/skills/application"
	"harnessforge/internal/skills/domain"
	"harnessforge/internal/skills/infrastructure"
)

func newSkillCommand() *cobra.Command {
	command := &cobra.Command{Use: "skill", Short: "Discover reviewed repository procedures"}
	discover := &cobra.Command{Use: "discover [repository]", Short: "Propose recurring development procedures", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		proposals, err := application.NewDiscover(infrastructure.RepositoryFiles{}).Execute(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(proposals)
	}}
	var approve bool
	generate := &cobra.Command{Use: "generate [repository] [proposal-json]", Short: "Generate an approved skill", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var proposal domain.Proposal
		if err := json.Unmarshal([]byte(args[1]), &proposal); err != nil {
			return fmt.Errorf("invalid proposal JSON: %w", err)
		}
		if err := application.NewGenerate(infrastructure.Store{}).Execute(args[0], proposal, approve); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Generated skill %s\n", proposal.ID)
		return err
	}}
	generate.Flags().BoolVar(&approve, "approve", false, "Confirm human review of this proposal")
	command.AddCommand(discover, generate)
	return command
}
