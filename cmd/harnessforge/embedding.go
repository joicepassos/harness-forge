package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/embeddings/application"
	"harnessforge/internal/embeddings/domain"
	"harnessforge/internal/embeddings/infrastructure"
	"net/http"
	"os"
)

func newEmbeddingCommand() *cobra.Command {
	command := &cobra.Command{Use: "embedding", Short: "Create and compare standalone embeddings"}
	var model string
	create := &cobra.Command{Use: "create [text]", Short: "Create an embedding for a short text", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		vector, err := application.NewEmbed(infrastructure.NewOpenAI("https://api.openai.com/v1", os.Getenv("OPENAI_API_KEY"), &http.Client{})).Execute(cmd.Context(), model, args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(vector)
	}}
	create.Flags().StringVar(&model, "model", infrastructure.DefaultModel, "Embedding model")
	similarity := &cobra.Command{Use: "similarity [first-json] [second-json]", Short: "Calculate cosine similarity", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var first, second domain.Vector
		if err := json.Unmarshal([]byte(args[0]), &first); err != nil {
			return fmt.Errorf("invalid first vector: %w", err)
		}
		if err := json.Unmarshal([]byte(args[1]), &second); err != nil {
			return fmt.Errorf("invalid second vector: %w", err)
		}
		value, err := domain.Cosine(first, second)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]float64{"cosine_similarity": value})
	}}
	command.AddCommand(create, similarity)
	return command
}
