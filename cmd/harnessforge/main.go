package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"harnessforge/internal/analyzer"
	"harnessforge/internal/config"
	"harnessforge/internal/harness"
	"harnessforge/internal/onboarding"

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
		Short: "Print HarnessForge build metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := newLocalizer(string(language))
			if err != nil {
				return err
			}
			cmd.Printf("harnessforge version %s\ncommit %s\nbuild date %s\n", config.Version, config.Commit, config.BuildDate)
			return nil
		},
	})

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create the initial harness configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			l, err := newLocalizer(string(language))
			if err != nil {
				return err
			}
			path, err := harness.Init(".")
			if err != nil {
				if os.IsExist(err) {
					return l.printf(cmd, "output.exists", ".harness/harness.yaml")
				}
				return err
			}

			if err := l.printf(cmd, "output.created", path); err != nil {
				return err
			}
			return l.printf(cmd, "output.next_steps")
		},
	}
	var setupBudget int
	var setupModel string
	var setupNotes string
	var setupBM25 bool
	var setupMMR bool
	setupContext := &cobra.Command{Use: "context [repository] [goal]", Short: "Preview the shared onboarding context", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		plan, err := onboarding.BuildSetupContext(cmd.Context(), args[0], onboarding.SetupContextRequest{Goal: args[1], Notes: setupNotes, Model: setupModel, BudgetTokens: setupBudget, UseBM25: setupBM25, UseMMR: setupMMR})
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}}
	setupContext.Flags().IntVar(&setupBudget, "budget", 0, "Maximum context budget")
	setupContext.Flags().StringVar(&setupModel, "model", "", "Model used for default budget selection")
	setupContext.Flags().StringVar(&setupNotes, "notes", "", "Additional setup notes")
	setupContext.Flags().BoolVar(&setupBM25, "bm25", false, "Use the experimental BM25 ranking baseline")
	setupContext.Flags().BoolVar(&setupMMR, "mmr", false, "Use deterministic diversity-aware excerpt selection")
	initCmd.AddCommand(setupContext)
	rootCmd.AddCommand(initCmd)

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

			style := presentationFor(cmd.OutOrStdout())
			if !style.colorful {
				analyzer.Print(cmd.OutOrStdout(), analysis)
				return nil
			}
			var output bytes.Buffer
			analyzer.Print(&output, analysis)
			_, err = fmt.Fprint(cmd.OutOrStdout(), style.analysis(output.String()))
			return err
		},
	}
	analyzeCmd.Flags().Bool("git", false, "Include Git repository metadata")
	analyzeCmd.Flags().String("format", "text", "Output format: text or json")
	rootCmd.AddCommand(analyzeCmd)

	rootCmd.AddCommand(newAskCommand(), newConfigCommand(), newValidateCommand(), newMigrateCommand(), newReviewCommand(), newContextCommand(), newSkillCommand(), newEvalCommand(), newDoctorCommand(), newGitHubCommand(), newDriftCommand(), newEmbeddingCommand(), newSymbolsCommand(), newGenerateCommand(), newSyncCommand(), newDiscoverCommand(), newIndexCommand(), newSearchCommand(), newRAGCommand(), newPluginCommand(), newTUICommand())
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
		cmd.Print(presentationFor(cmd.OutOrStdout()).help(cmd.Short, cmd.UsageString()))
	})
	return rootCmd
}
