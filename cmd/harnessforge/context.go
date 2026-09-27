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
	var bm25 bool
	var mmr bool
	var includeKnowledge bool
	var taskPaths []string
	command := &cobra.Command{Use: "context", Short: "Inspect model context"}
	explain := &cobra.Command{Use: "explain [repository] [prompt]", Short: "Explain repository context selection", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if budget < 0 {
			return fmt.Errorf("--budget must not be negative")
		}
		contextModel := model
		if contextModel == "" {
			contextModel = llm.DefaultModel("openai")
		}
		selection := ""
		if includeKnowledge {
			selection = "forge"
		}
		plan, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, contextpack.Options{BudgetTokens: budget, UseBM25: bm25, UseMMR: mmr, Layout: selection, TaskPaths: taskPaths})
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(plan); err != nil {
			return err
		}
		if plan.BudgetOverflow {
			return fmt.Errorf("context budget overflow: at least %d additional tokens required", plan.OverflowTokens)
		}
		return nil
	}}
	explain.Flags().IntVar(&budget, "budget", 0, "Maximum estimated tokens for repository context; 0 selects the model default")
	explain.Flags().StringVar(&model, "model", "", "Model name used to choose the default budget")
	explain.Flags().BoolVar(&bm25, "bm25", false, "Use the experimental BM25 lexical ranking baseline")
	explain.Flags().BoolVar(&mmr, "mmr", false, "Use experimental diversity-aware MMR-like ranking")
	explain.Flags().BoolVar(&includeKnowledge, "knowledge", false, "Include approved Forge knowledge as task context")
	explain.Flags().StringArrayVar(&taskPaths, "task-path", nil, "Repository-relative file path affected by the task (repeatable; used with --knowledge)")
	command.AddCommand(explain)
	return command
}
