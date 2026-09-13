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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cobra.CheckErr(newRootCommand().ExecuteContext(ctx))
}

func newRootCommand() *cobra.Command {
	language := languageValue("en")
	rootCmd := &cobra.Command{
		Use:   "harnessforge",
		Short: "HarnessForge creates and maintains coding-agent harnesses",
	}
	rootCmd.PersistentFlags().Var(&language, "language", "Language for CLI help and common output (en, pt-BR or es)")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the HarnessForge version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := newLocalizer(string(language))
			if err != nil {
				return err
			}
			cmd.Println("harnessforge version " + config.Version)
			return nil
		},
	})

	rootCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create the initial harness configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := newLocalizer(string(language))
			if err != nil {
				return err
			}
			path, err := harness.Init(".")
			if err != nil {
				return err
			}

			return l.printf(cmd, "output.created", path)
		},
	})

	analyzeCmd := &cobra.Command{
		Use:   "analyze [path]",
		Short: "Analyze a repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := localizerFor(cmd)
			if err != nil {
				return err
			}
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
				return fmt.Errorf(l.text("error.unsupported_format"), format)
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

	rootCmd.AddCommand(newAskCommand(), newConfigCommand(), newValidateCommand(), newReviewCommand(), newContextCommand(), newSkillCommand(), newEvalCommand(), newDoctorCommand(), newGitHubCommand(), newDriftCommand(), newEmbeddingCommand(), newSymbolsCommand(), newGenerateCommand(), newDiscoverCommand(), newIndexCommand(), newSearchCommand(), newRAGCommand(), newPluginCommand())
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		l, err := newLocalizer(string(language))
		if err != nil {
			return err
		}
		applyLanguage(rootCmd, l)
		return nil
	}
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		l, err := newLocalizer(string(language))
		if err == nil {
			applyLanguage(rootCmd, l)
		}
		cmd.Printf("%s\n\n%s", cmd.Short, cmd.UsageString())
	})
	return rootCmd
}
