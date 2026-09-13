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

type VersionReader interface {
	ContainsAt(context.Context, string, string, string) (bool, error)
}

type Detect struct {
	loader HarnessLoader
	reader RuleReader
}

func NewDetect(loader HarnessLoader, reader RuleReader) *Detect {
	return &Detect{loader: loader, reader: reader}
}

func (d *Detect) Execute(ctx context.Context, harnessPath string) (domain.Report, error) {
	if err := ctx.Err(); err != nil {
		return domain.Report{}, err
	}
	h, err := d.loader.Load(harnessPath)
	if err != nil {
		return domain.Report{}, err
	}
	report := domain.Report{Occurrences: []domain.Occurrence{}, Limitations: []string{
		"A difference can be a violation, an intentional architectural change, or stale evidence; it is not classified automatically.",
		"Only approved rules with literal evidence symbols are evaluated in this initial strategy.",
		"All proposals are review-only and no repository or Harness IR file is modified.",
	}}
	for _, rule := range h.Rules {
		if err := ctx.Err(); err != nil {
			return domain.Report{}, err
		}
		if rule.Status != "approved" {
			continue
		}
		if len(rule.Evidence) == 0 {
			report.Occurrences = append(report.Occurrences, domain.Occurrence{RuleID: rule.ID, Status: domain.StatusNotEvaluated, Explanations: []string{"The rule has no structural evidence."}})
			continue
		}
		for _, evidence := range rule.Evidence {
			report.Occurrences = append(report.Occurrences, d.evaluateEvidence(ctx, rule.ID, evidence))
		}
	}
	return report, ctx.Err()
}

func (d *Detect) evaluateEvidence(ctx context.Context, id string, evidence harnessdomain.Evidence) domain.Occurrence {
	occurrence := domain.Occurrence{RuleID: id, Status: domain.StatusNotEvaluated, Locations: []string{evidence.File}, Revision: evidence.Revision}
	if evidence.Symbol == "" {
		occurrence.Explanations = []string{"The evidence has no literal structural symbol."}
		return occurrence
	}
	if evidence.Revision != "" {
		reader, ok := d.reader.(VersionReader)
		if !ok {
			occurrence.Explanations = []string{"The repository reader cannot evaluate historical revisions."}
			return occurrence
		}
		present, err := reader.ContainsAt(ctx, evidence.File, evidence.Symbol, evidence.Revision)
		if err != nil {
			occurrence.BaselineStatus = "not_evaluated"
			occurrence.Explanations = []string{"Historical evidence could not be evaluated: " + err.Error()}
			return occurrence
		}
		if !present {
			occurrence.BaselineStatus = "invalid"
			occurrence.Explanations = []string{"The symbol is absent from the declared baseline revision; review stale or invalid evidence."}
			return occurrence
		}
		occurrence.BaselineStatus = "verified"
	}
	found, err := d.reader.Contains(ctx, evidence.File, evidence.Symbol)
	if err != nil {
		occurrence.Explanations = []string{"Current evidence could not be evaluated: " + err.Error()}
		return occurrence
	}
	if found {
		occurrence.Status = domain.StatusAligned
		return occurrence
	}
	change := difference(id, evidence.File, "The referenced literal symbol is absent.")
	change.Revision, change.BaselineStatus = occurrence.Revision, occurrence.BaselineStatus
	return change
}

func difference(ruleID, location, detail string) domain.Occurrence {
	return domain.Occurrence{RuleID: ruleID, Status: domain.StatusDifference, Locations: []string{location}, Explanations: []string{detail, "Possible violation of the approved rule.", "Possible intentional architectural change.", "Possible stale rule or evidence."}, CodeProposal: "Review the affected code and restore the intended structure if the rule remains valid.", HarnessProposal: "Review the approved rule and update its evidence or status if the architecture intentionally changed."}
}
