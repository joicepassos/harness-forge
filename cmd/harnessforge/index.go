package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"harnessforge/internal/indexing/application"
	"harnessforge/internal/indexing/infrastructure"
)

func newIndexCommand() *cobra.Command {
	return &cobra.Command{Use: "index [repository]", Short: "Build an incremental document index", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		report, err := application.NewBuild(infrastructure.Documents{}, infrastructure.Markdown{}, infrastructure.Lexical{}, infrastructure.JSONStore{}, "lexical-hash-v1").Execute(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}}
}
