package main

import (
	"encoding/json"
	"github.com/spf13/cobra"
	"harnessforge/internal/evals/application"
	"harnessforge/internal/evals/domain"
	"harnessforge/internal/evals/infrastructure"
	"os"
)

func newEvalCommand() *cobra.Command {
	command := &cobra.Command{Use: "eval", Short: "Evaluate retrieval and grounded answers"}
	run := &cobra.Command{Use: "run [dataset] [results]", Short: "Run deterministic evaluation", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		dataset, err := infrastructure.Dataset(args[0])
		if err != nil {
			return err
		}
		results, err := infrastructure.Results(args[1])
		if err != nil {
			return err
		}
		report, err := application.Evaluate(dataset, results)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}}
	compare := &cobra.Command{Use: "compare [baseline] [candidate]", Short: "Compare evaluation reports", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		baseline, err := report(args[0])
		if err != nil {
			return err
		}
		candidate, err := report(args[1])
		if err != nil {
			return err
		}
		comparison, err := application.Compare(baseline, candidate)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(comparison)
	}}
	command.AddCommand(run, compare)
	return command
}

func report(path string) (value domain.Report, err error) {
	file, err := os.Open(path)
	if err != nil {
		return value, err
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&value)
	return value, err
}
