package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/config"
	"harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
)

func newConfigCommand() *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Manage user preferences (credentials stay in environment variables)"}
	set := &cobra.Command{Use: "set provider <name>", Short: "Save the default AI provider", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != "provider" {
			return fmt.Errorf("only the provider preference is supported")
		}
		preferences, err := config.UserPreferences()
		if err != nil {
			return err
		}
		if err := preferences.SetProvider(args[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), presentationFor(cmd.OutOrStdout()).status("success", "Provider preference saved"))
		return err
	}}
	get := &cobra.Command{Use: "get provider", Short: "Show the default AI provider", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] != "provider" {
			return fmt.Errorf("only the provider preference is supported")
		}
		preferences, err := config.UserPreferences()
		if err != nil {
			return err
		}
		name, err := preferences.Provider()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), name)
		return err
	}}
	command.AddCommand(set, get)
	return command
}

func selectedProvider(cmd *cobra.Command) (string, error) {
	return selectedProviderFor(cmd, ".")
}

func selectedProviderFor(cmd *cobra.Command, repository string) (string, error) {
	if cmd.Flags().Changed("provider") {
		return cmd.Flags().GetString("provider")
	}
	project, err := projectAIConfig(repository)
	if err != nil {
		return "", err
	}
	if project.Provider != "" {
		return project.Provider, nil
	}
	preferences, err := config.UserPreferences()
	if err != nil {
		return "", err
	}
	return preferences.Provider()
}

func selectedModelFor(repository, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	project, err := projectAIConfig(repository)
	if err != nil {
		return "", err
	}
	return project.Model, nil
}

func repositoryOrCurrent(repository string) string {
	if repository == "" {
		return "."
	}
	return repository
}

func projectAIConfig(repository string) (struct{ Provider, Model string }, error) {
	var selected struct{ Provider, Model string }
	path := filepath.Join(repository, ".harness", "harness.yaml")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return selected, nil
	}
	if err != nil {
		return selected, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return selected, fmt.Errorf("unsafe harness configuration")
	}
	harness, err := (infrastructure.YAMLLoader{}).Load(path)
	if err != nil {
		return selected, err
	}
	selected.Provider, selected.Model = harness.AI.Provider, harness.AI.Model
	return selected, nil
}
