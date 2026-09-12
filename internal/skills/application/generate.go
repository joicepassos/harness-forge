package application

import (
	"fmt"
	"harnessforge/internal/skills/domain"
)

type SkillWriter interface {
	Generate(string, domain.Proposal) error
}

type Generate struct{ writer SkillWriter }

func NewGenerate(writer SkillWriter) *Generate { return &Generate{writer: writer} }

func (g *Generate) Execute(repository string, proposal domain.Proposal, approved bool) error {
	if !approved {
		return fmt.Errorf("skill generation requires explicit human approval")
	}
	if err := ValidateID(proposal.ID); err != nil {
		return err
	}
	if proposal.Description == "" || len(proposal.Examples) == 0 {
		return fmt.Errorf("proposal requires a description and examples")
	}
	if err := ValidateEvidence(proposal.Examples); err != nil {
		return err
	}
	return g.writer.Generate(repository, proposal)
}
