package main

import (
	"github.com/spf13/cobra"
)

// install and tui remain as aliases for users of the earlier guided menu.
func newTUICommand() *cobra.Command {
	return &cobra.Command{
		Use:     "install [repository]",
		Aliases: []string{"tui"},
		Short:   "Run the guided project setup (alias for init)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository := "."
			if len(args) > 0 {
				repository = args[0]
			}
			return runGuidedInit(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), repository, requestSetupProposal)
		},
	}
}
