package main

import (
	"context"
	"fmt"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/config"
	"harnessforge/internal/harness"
	"os"
	"os/signal"

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

			format, err := cmd.Flags().GetString("format")
			if err != nil {
				return err
			}

			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported format %q", format)
			}

			analysis, err := analyzer.AnalyzeWithOptions(cmd.Context(), repositoryPath, analyzer.Options{
				IncludeGit: includeGit,
			})
			if err != nil {
				return err
			}

			if format == "json" {
				return analyzer.PrintJSON(cmd.OutOrStdout(), analysis)
			}

			analyzer.Print(cmd.OutOrStdout(), analysis)
			return nil
		},
	}
	analyzeCmd.Flags().Bool("git", false, "Include Git repository metadata")
	analyzeCmd.Flags().String("format", "text", "Output format: text or json")
	rootCmd.AddCommand(analyzeCmd)

	rootCmd.AddCommand(newAskCommand(), newConfigCommand(), newValidateCommand(), newReviewCommand())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cobra.CheckErr(rootCmd.ExecuteContext(ctx))
}
