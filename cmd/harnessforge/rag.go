package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"harnessforge/internal/rag/application"
	"harnessforge/internal/rag/infrastructure"
)

func newRAGCommand() *cobra.Command {
	var k int
	var model string
	var direct bool
	command := &cobra.Command{Use: "rag [repository] [query]", Short: "Answer with validated repository citations", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := selectedProviderFor(cmd, args[0])
		if err != nil {
			return err
		}
		model, err = selectedModelFor(args[0], model)
		if err != nil {
			return err
		}
		answer, err := application.NewAnswer(infrastructure.Retriever{}, infrastructure.Generator{Provider: provider, Model: model}).Execute(cmd.Context(), args[0], args[1], k, direct)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(answer)
	}}
	command.Flags().String("provider", "openai", "Override saved provider")
	command.Flags().StringVar(&model, "model", "", "Model override")
	command.Flags().IntVar(&k, "k", 5, "Maximum retrieved chunks")
	command.Flags().BoolVar(&direct, "direct", false, "Ask without repository retrieval or citations")
	return command
}
