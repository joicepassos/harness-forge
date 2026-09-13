package main

import (
	"encoding/json"
	"fmt"
	"harnessforge/internal/plugins/application"
	"harnessforge/internal/plugins/infrastructure"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newPluginCommand() *cobra.Command {
	service := application.NewPlugins(infrastructure.Local{}, infrastructure.Local{})
	command := &cobra.Command{Use: "plugin", Short: "Discover and explicitly run versioned extensions"}
	command.AddCommand(&cobra.Command{Use: "discover [directory]", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		plugins, err := service.Discover(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(plugins)
	}})
	var authorized bool
	var capability string
	var timeout time.Duration
	run := &cobra.Command{Use: "run [directory] [name] [input-json]", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		if len(args[2]) > 1024*1024 {
			return fmt.Errorf("plugin input exceeds 1 MiB")
		}
		input := map[string]any{}
		decoder := json.NewDecoder(strings.NewReader(args[2]))
		if err := decoder.Decode(&input); err != nil {
			return fmt.Errorf("invalid plugin input: %w", err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return fmt.Errorf("invalid plugin input: trailing content")
		}
		response, err := service.Execute(cmd.Context(), args[0], args[1], capability, input, authorized, timeout)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(response)
	}}
	run.Flags().BoolVar(&authorized, "authorize", false, "Explicitly authorize this plugin execution")
	run.Flags().StringVar(&capability, "capability", "analyzer", "Declared capability to invoke")
	run.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Execution timeout, at most 5m")
	command.AddCommand(run)
	return command
}
