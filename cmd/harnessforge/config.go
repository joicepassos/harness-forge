package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/config"
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
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Provider preference saved")
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
	if cmd.Flags().Changed("provider") {
		return cmd.Flags().GetString("provider")
	}
	preferences, err := config.UserPreferences()
	if err != nil {
		return "", err
	}
	return preferences.Provider()
}
