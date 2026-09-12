package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/contextpack"
	"harnessforge/internal/llm"
)

func newAskCommand() *cobra.Command {
	var model, format, repository string
	var contextBudget int
	var stream bool
	var contextExplain bool
	cmd := &cobra.Command{Use: "ask [prompt]", Short: "Ask an AI provider", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if format != "text" && format != "json" {
			return fmt.Errorf("unsupported format %q", format)
		}
		if stream && format == "json" {
			return fmt.Errorf("--stream cannot be used with validated JSON output")
		}
		if repository != "" && format != "json" {
			return fmt.Errorf("--repository requires --format json")
		}
		if contextExplain && repository == "" {
			return fmt.Errorf("--context-explain requires --repository")
		}
		if contextExplain && stream {
			return fmt.Errorf("--context-explain cannot be used with --stream")
		}
		provider, err := selectedProvider(cmd)
		if err != nil {
			return err
		}
		ask := llm.NewAsk()
		if stream {
			err := ask.Stream(cmd.Context(), provider, model, args[0], func(text string) error { _, err := fmt.Fprint(cmd.OutOrStdout(), text); return err })
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout())
			return err
		}
		if format == "json" {
			sources := map[string]string{"prompt": args[0]}
			if repository != "" {
				contextModel := model
				if contextModel == "" {
					contextModel = llm.DefaultModel(provider)
				}
				plan, err := contextpack.Build(cmd.Context(), repository, args[0], contextModel, contextpack.Options{BudgetTokens: contextBudget})
				if err != nil {
					return err
				}
				if contextExplain {
					encoder := json.NewEncoder(cmd.OutOrStdout())
					encoder.SetIndent("", "  ")
					return encoder.Encode(plan)
				}
				sources = contextpack.Sources(args[0], plan)
			} else if contextExplain {
				return fmt.Errorf("--context-explain requires --repository")
			}
			result, err := ask.StructuredWithSources(cmd.Context(), provider, model, args[0], sources)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(result)
		}
		result, err := ask.Execute(cmd.Context(), provider, model, args[0])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), result.Content)
		return err
	}}
	cmd.Flags().StringVar(&model, "model", "", "Model override")
	cmd.Flags().String("provider", "openai", "Override saved provider")
	cmd.Flags().StringVar(&format, "format", "text", "Output: text or validated json")
	cmd.Flags().StringVar(&repository, "repository", "", "Include selected repository context as evidence (JSON only)")
	cmd.Flags().IntVar(&contextBudget, "context-budget", 0, "Maximum estimated tokens for repository context; 0 selects the model default")
	cmd.Flags().BoolVar(&contextExplain, "context-explain", false, "Print repository context selection and do not call the provider")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream text as it arrives")
	return cmd
}
