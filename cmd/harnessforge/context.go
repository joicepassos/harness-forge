package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/contextpack"
	"harnessforge/internal/llm"
	"sort"
)

type contextComparisonReport struct {
	MetricNature          string            `json:"metric_nature"`
	Baseline              *contextpack.Plan `json:"baseline_without_knowledge"`
	WithApprovedKnowledge *contextpack.Plan `json:"with_approved_knowledge"`
	EstimatedTokenDelta   int               `json:"estimated_token_delta"`
	SelectedKnowledgeIDs  []string          `json:"selected_knowledge_ids"`
	SameEstimator         bool              `json:"same_estimator"`
	Interpretation        string            `json:"interpretation"`
}

func newContextCommand() *cobra.Command {
	var budget int
	var model string
	var bm25 bool
	var mmr bool
	var includeKnowledge bool
	var compareKnowledge bool
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
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		options := contextpack.Options{BudgetTokens: budget, UseBM25: bm25, UseMMR: mmr, TaskPaths: taskPaths}
		if compareKnowledge {
			if includeKnowledge {
				return fmt.Errorf("--knowledge cannot be combined with --compare-knowledge")
			}
			baselineOptions := options
			baselineOptions.ExcludeKnowledge = true
			baseline, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, baselineOptions)
			if err != nil {
				return err
			}
			knowledgeOptions := options
			knowledgeOptions.Layout = "forge"
			withKnowledge, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, knowledgeOptions)
			if err != nil {
				return err
			}
			ids := make([]string, 0)
			seen := make(map[string]bool)
			for _, excerpt := range withKnowledge.Included {
				if excerpt.KnowledgeID != "" && !seen[excerpt.KnowledgeID] {
					ids = append(ids, excerpt.KnowledgeID)
					seen[excerpt.KnowledgeID] = true
				}
			}
			sort.Strings(ids)
			report := contextComparisonReport{
				MetricNature:          "paired_context_selection_proxy_not_task_quality",
				Baseline:              baseline,
				WithApprovedKnowledge: withKnowledge,
				EstimatedTokenDelta:   withKnowledge.EstimatedTokens - baseline.EstimatedTokens,
				SelectedKnowledgeIDs:  ids,
				SameEstimator:         baseline.Estimator == withKnowledge.Estimator,
				Interpretation:        "Both plans use the same prompt, model, estimator, budget, ranking options, and task paths. Token and selection differences are retrieval proxies; they do not measure task correctness or productivity.",
			}
			if err := encoder.Encode(report); err != nil {
				return err
			}
			if baseline.BudgetOverflow || withKnowledge.BudgetOverflow {
				return fmt.Errorf("context budget overflow: baseline requires %d additional tokens; with approved knowledge requires %d", baseline.OverflowTokens, withKnowledge.OverflowTokens)
			}
			return nil
		}
		if includeKnowledge {
			options.Layout = "forge"
		}
		plan, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, options)
		if err != nil {
			return err
		}
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
	explain.Flags().BoolVar(&compareKnowledge, "compare-knowledge", false, "Compare context selection with and without approved Forge knowledge using the same budget")
	explain.Flags().StringArrayVar(&taskPaths, "task-path", nil, "Repository-relative file path affected by the task (repeatable; used with --knowledge)")
	command.AddCommand(explain)
	return command
}
