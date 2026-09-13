package application

import (
	"context"
	"harnessforge/internal/drift/domain"
	harnessdomain "harnessforge/internal/harness/domain"
)

type HarnessLoader interface {
	Load(string) (harnessdomain.Harness, error)
}
type RuleReader interface {
	Contains(context.Context, string, string) (bool, error)
}

type Detect struct {
	loader HarnessLoader
	reader RuleReader
}

func NewDetect(loader HarnessLoader, reader RuleReader) *Detect {
	return &Detect{loader: loader, reader: reader}
}

func (d *Detect) Execute(ctx context.Context, harnessPath string) (domain.Report, error) {
	h, err := d.loader.Load(harnessPath)
	if err != nil {
		return domain.Report{}, err
	}
	report := domain.Report{Limitations: []string{
		"A difference can be a violation, an intentional architectural change, or stale evidence; it is not classified automatically.",
		"Only approved rules with literal evidence symbols are evaluated in this initial strategy.",
		"All proposals are review-only and no repository or Harness IR file is modified.",
	}}
	for _, rule := range h.Rules {
		if rule.Status == "approved" {
			report.Occurrences = append(report.Occurrences, d.evaluate(ctx, rule))
		}
	}
	return report, nil
}

func (d *Detect) evaluate(ctx context.Context, rule harnessdomain.Rule) domain.Occurrence {
	if len(rule.Evidence) == 0 || rule.Evidence[0].Symbol == "" {
		return domain.Occurrence{RuleID: rule.ID, Status: domain.StatusNotEvaluated, Explanations: []string{"The rule has no literal structural evidence symbol."}}
	}
	evidence := rule.Evidence[0]
	found, err := d.reader.Contains(ctx, evidence.File, evidence.Symbol)
	if err != nil {
		return difference(rule.ID, evidence.File, err.Error())
	}
	if found {
		return domain.Occurrence{RuleID: rule.ID, Status: domain.StatusAligned, Locations: []string{evidence.File}}
	}
	return difference(rule.ID, evidence.File, "The referenced literal symbol is absent.")
}

func difference(ruleID, location, detail string) domain.Occurrence {
	return domain.Occurrence{RuleID: ruleID, Status: domain.StatusDifference, Locations: []string{location}, Explanations: []string{detail, "Possible violation of the approved rule.", "Possible intentional architectural change.", "Possible stale rule or evidence."}, CodeProposal: "Review the affected code and restore the intended structure if the rule remains valid.", HarnessProposal: "Review the approved rule and update its evidence or status if the architecture intentionally changed."}
}
