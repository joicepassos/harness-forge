package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	indexinfra "harnessforge/internal/indexing/infrastructure"
	"harnessforge/internal/retrieval/application"
	retrievalinfra "harnessforge/internal/retrieval/infrastructure"
	"strings"
)

func newSearchCommand() *cobra.Command {
	var k int
	var path, relevant string
	command := &cobra.Command{Use: "search [repository] [query]", Short: "Search the document index without text generation", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		var expected []string
		if relevant != "" {
			expected = strings.Split(relevant, ",")
		}
		report, err := application.NewSearch(indexinfra.JSONStore{}, indexinfra.Lexical{}, retrievalinfra.Files{}, "lexical-hash-v1").Execute(cmd.Context(), args[0], args[1], path, k, expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}}
	command.Flags().IntVar(&k, "k", 5, "Maximum results from 1 to 100")
	command.Flags().StringVar(&path, "path", "", "Filter sources by relative path prefix")
	command.Flags().StringVar(&relevant, "relevant", "", "Comma-separated expected chunk IDs for retrieval metrics")
	return command
}
