package main

import (
	"github.com/spf13/cobra"
)

// install and tui remain as aliases for users of the earlier guided menu.
func newTUICommand() *cobra.Command {
	var accessible bool
	var background bool
	command := &cobra.Command{
		Use:     "install [repository]",
		Aliases: []string{"tui"},
		Short:   "Run the guided project setup (alias for init)",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repository := "."
			if len(args) > 0 {
				repository = args[0]
			}
			return runGuidedInitOptions(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), repository, requestSetupProposal, accessible, background)
		},
	}
	command.Flags().BoolVar(&accessible, "accessible", false, "Use plain prompts for screen readers and automation")
	command.Flags().BoolVar(&background, "background", false, "Generate the authorized AI proposal in a detached process")
	return command
}
