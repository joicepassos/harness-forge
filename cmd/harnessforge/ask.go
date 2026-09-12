package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/llm"
)

func newAskCommand() *cobra.Command {
	var model, format, repository string
	var stream bool
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
			evidence := ""
			if repository != "" {
				report, err := analyzer.AnalyzeWithOptions(cmd.Context(), repository, analyzer.Options{})
				if err != nil {
					return err
				}
				data, err := json.Marshal(report)
				if err != nil {
					return err
				}
				evidence = string(data)
			}
			result, err := ask.Structured(cmd.Context(), provider, model, args[0], evidence)
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
	cmd.Flags().StringVar(&repository, "repository", "", "Include deterministic repository report as evidence (JSON only)")
	cmd.Flags().BoolVar(&stream, "stream", false, "Stream text as it arrives")
	return cmd
}
