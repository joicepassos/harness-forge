package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"harnessforge/internal/contextpack"
	"harnessforge/internal/discovery/application"
	"harnessforge/internal/discovery/domain"
	discoveryinfra "harnessforge/internal/discovery/infrastructure"
	"harnessforge/internal/llm"
	"strings"
	"unicode"
)

func newDiscoverCommand() *cobra.Command {
	command := &cobra.Command{Use: "discover", Short: "Propose and apply evidence-backed rules"}
	var model string
	propose := &cobra.Command{Use: "propose [repository] [prompt]", Short: "Propose rules without changing the harness", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := selectedProviderFor(cmd, args[0])
		if err != nil {
			return err
		}
		model, err = selectedModelFor(args[0], model)
		if err != nil {
			return err
		}
		contextModel := model
		if contextModel == "" {
			contextModel = llm.DefaultModel(provider)
		}
		plan, err := contextpack.Build(cmd.Context(), args[0], args[1], contextModel, contextpack.Options{})
		if err != nil {
			return err
		}
		analysis, err := llm.NewAsk().StructuredWithSources(cmd.Context(), provider, model, args[1], contextpack.Sources(args[1], plan))
		if err != nil {
			return err
		}
		proposals := []domain.Proposal{}
		for _, pattern := range analysis.Patterns {
			proposal := domain.Proposal{ID: slug(pattern.Name), Description: "Adopt " + pattern.Name + " after reviewing the cited repository evidence.", Confidence: pattern.Confidence, Limitations: []string{"Confidence is an uncalibrated model estimate and does not prove this rule is correct."}}
			for _, evidence := range pattern.Evidence {
				if strings.HasPrefix(evidence.Source, "repository-file:") {
					proposal.Evidence = append(proposal.Evidence, domain.Evidence{File: strings.TrimPrefix(evidence.Source, "repository-file:"), Symbol: evidence.Quote})
				}
			}
			if len(proposal.Evidence) > 0 && proposal.ID != "" {
				proposals = append(proposals, proposal)
			}
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(proposals)
	}}
	propose.Flags().String("provider", "openai", "Override saved provider")
	propose.Flags().StringVar(&model, "model", "", "Model override")
	var repository, harness string
	var approve bool
	apply := &cobra.Command{Use: "apply [proposal-json]", Short: "Apply an explicitly approved proposal", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var proposal domain.Proposal
		if err := json.Unmarshal([]byte(args[0]), &proposal); err != nil {
			return fmt.Errorf("invalid proposal JSON: %w", err)
		}
		return application.NewApply(discoveryinfra.Store{}).Execute(repository, harness, proposal, approve)
	}}
	apply.Flags().StringVar(&repository, "repository", ".", "Repository containing the evidence")
	apply.Flags().StringVar(&harness, "file", ".harness/harness.yaml", "Harness YAML file")
	apply.Flags().BoolVar(&approve, "approve", false, "Confirm human approval of this proposal")
	command.AddCommand(propose, apply)
	return command
}
func slug(value string) string {
	var out []rune
	dash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, r)
			dash = false
		} else if len(out) > 0 && !dash {
			out = append(out, '-')
			dash = true
		}
	}
	return strings.Trim(string(out), "-")
}
