package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/github/application"
	githubinfra "harnessforge/internal/github/infrastructure"
	"os"
)

func newGitHubCommand() *cobra.Command {
	var tokenEnv string
	command := &cobra.Command{Use: "github", Short: "Read GitHub discussions without changing them"}
	learn := &cobra.Command{Use: "learn owner/repository", Short: "Extract reviewable knowledge from GitHub", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		token := os.Getenv(tokenEnv)
		if token == "" {
			return fmt.Errorf("%s is not set; provide a token through the process environment", tokenEnv)
		}
		report, err := application.NewExtract(githubinfra.NewClientWithOptions("https://api.github.com", token, githubinfra.NewHTTPClient(), 10)).Execute(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}}
	learn.Flags().StringVar(&tokenEnv, "token-env", "GITHUB_TOKEN", "Environment variable containing a GitHub token")
	command.AddCommand(learn)
	return command
}
