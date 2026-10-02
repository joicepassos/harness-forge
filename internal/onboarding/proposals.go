package onboarding

import (
	"strings"
	"unicode"

	"harnessforge/internal/discovery/domain"
	llmdomain "harnessforge/internal/llm/domain"
)

// Candidates converts validated model findings into reviewable proposals.
// It never marks a rule approved and drops findings without repository-file
// evidence so unsupported model output cannot become a harness rule.
func Candidates(analysis llmdomain.ArchitectureAnalysis) []domain.Proposal {
	proposals := make([]domain.Proposal, 0, len(analysis.Patterns))
	for _, pattern := range analysis.Patterns {
		proposal := domain.Proposal{
			ID:          slug(pattern.Name),
			Description: "Adopt " + pattern.Name + " after reviewing the cited repository evidence.",
			Confidence:  pattern.Confidence,
			Limitations: []string{"Confidence is an uncalibrated model estimate and does not prove this rule is correct."},
		}
		for _, evidence := range pattern.Evidence {
			if !strings.HasPrefix(evidence.Source, "repository-file:") {
				continue
			}
			file := strings.TrimPrefix(evidence.Source, "repository-file:")
			if hash := strings.IndexByte(file, '#'); hash >= 0 {
				file = file[:hash]
			}
			proposal.Evidence = append(proposal.Evidence, domain.Evidence{File: file, Quote: evidence.Quote, Workspace: evidence.Workspace})
		}
		if proposal.ID != "" && len(proposal.Evidence) > 0 {
			proposals = append(proposals, proposal)
		}
	}
	return proposals
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
