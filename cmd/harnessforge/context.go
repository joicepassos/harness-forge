package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/contextpack"
	"harnessforge/internal/llm"
)

func newContextCommand() *cobra.Command {
	var budget int
	var model string
	command := &cobra.Command{Use: "context", Short: "Inspect model context"}
	explain := &cobra.Command{Use: "explain [repository] [prompt]", Short: "Explain repository context selection", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if budget < 0 {
			return fmt.Errorf("--budget must not be negative")
		}
		contextModel := model
		if contextModel == "" {
			contextModel = llm.DefaultModel("openai")
		}
		plan, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, contextpack.Options{BudgetTokens: budget})
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}}
	explain.Flags().IntVar(&budget, "budget", 0, "Maximum estimated tokens for repository context; 0 selects the model default")
	explain.Flags().StringVar(&model, "model", "", "Model name used to choose the default budget")
	command.AddCommand(explain)
	return command
}
