package main

import (
	"context"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/config"
	"harnessforge/internal/harness"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "harnessforge",
		Short: "HarnessForge creates and maintains coding-agent harnesses",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the HarnessForge version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("harnessforge version " + config.Version)
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create the initial harness configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := harness.Init(".")
			if err != nil {
				return err
			}

			cmd.Printf("Created %s\n", path)
			return nil
		},
	})

	analyzeCmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repositoryPath := "."
			if len(args) > 0 {
				repositoryPath = args[0]
			}

			includeGit, err := cmd.Flags().GetBool("git")
			if err != nil {
				return err
			}

			analysis, err := analyzer.AnalyzeWithOptions(context.Background(), repositoryPath, analyzer.Options{
				IncludeGit: includeGit,
			})
			if err != nil {
				return err
			}

			analyzer.Print(cmd.OutOrStdout(), analysis)
			return nil
		},
	}
	analyzeCmd.Flags().Bool("git", false, "Include Git repository metadata")
	rootCmd.AddCommand(analyzeCmd)

	cobra.CheckErr(rootCmd.Execute())
}
