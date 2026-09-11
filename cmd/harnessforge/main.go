package main

import (
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

	rootCmd.AddCommand(&cobra.Command{
		Use:   "analyze",
		Short: "Analyze the current repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			analysis, err := analyzer.Analyze(".")
			if err != nil {
				return err
			}

			analyzer.Print(cmd.OutOrStdout(), analysis)
			return nil
		},
	})

	cobra.CheckErr(rootCmd.Execute())
}
