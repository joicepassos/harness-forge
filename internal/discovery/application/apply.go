package application

import (
	"fmt"
	"harnessforge/internal/discovery/domain"
	"regexp"
)

var safeID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Store interface {
	Apply(string, string, domain.Proposal) error
}
type Apply struct{ store Store }

func NewApply(store Store) *Apply { return &Apply{store} }
func (a *Apply) Execute(repository, harness string, proposal domain.Proposal, approved bool) error {
	if !approved {
		return fmt.Errorf("discovery application requires explicit human approval")
	}
	if !safeID.MatchString(proposal.ID) || proposal.Description == "" || len(proposal.Evidence) == 0 {
		return fmt.Errorf("proposal requires a safe id, description and evidence")
	}
	return a.store.Apply(repository, harness, proposal)
}
