package main

import (
	"github.com/spf13/cobra"
	"harnessforge/internal/mcp"
)

func newMCPCommand() *cobra.Command {
	var repository, model string
	var budget int
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Serve read-only Forge context resources over MCP stdio",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return (mcp.Server{Repository: repository, Model: model, Budget: budget}).Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	serve.Flags().StringVar(&repository, "repository", ".", "Repository to expose as read-only context")
	serve.Flags().StringVar(&model, "model", "", "Model used for context budget estimation")
	serve.Flags().IntVar(&budget, "budget", mcp.DefaultBudgetTokens, "Maximum context budget in tokens")
	command := &cobra.Command{Use: "mcp", Short: "Expose read-only context over Model Context Protocol"}
	command.AddCommand(serve)
	return command
}
