package domain

import (
	"fmt"
	"strings"
)

type ArchitectureAnalysis struct {
	Architecture []string  `json:"architecture"`
	Patterns     []Pattern `json:"patterns"`
}
type Pattern struct {
	Name       string     `json:"name"`
	Confidence float64    `json:"confidence"`
	Evidence   []Evidence `json:"evidence"`
}
type Evidence struct {
	Source string `json:"source"`
	Quote  string `json:"quote"`
}

// ValidateEvidence verifies citations against supplied context, not the truth of an inference.
func (a ArchitectureAnalysis) ValidateEvidence(sources map[string]string) error {
	names := map[string]bool{}
	for i, p := range a.Patterns {
		if names[p.Name] {
			return fmt.Errorf("patterns[%d].name: duplicate", i)
		}
		names[p.Name] = true
		for j, e := range p.Evidence {
			source, ok := sources[e.Source]
			if !ok || strings.TrimSpace(e.Quote) == "" || !strings.Contains(source, e.Quote) {
				return fmt.Errorf("patterns[%d].evidence[%d]: quote not found in source %q", i, j, e.Source)
			}
		}
	}
	for i, style := range a.Architecture {
		if !names[style] {
			return fmt.Errorf("architecture[%d]: requires a pattern with the same name and evidence", i)
		}
	}
	return nil
}
