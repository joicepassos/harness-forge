package application

import "harnessforge/internal/harness/domain"

type RuleStore interface {
	Load(string) (domain.Harness, error)
	SaveStatus(path, id, previous, status string) error
}
type Review struct{ store RuleStore }

func NewReview(store RuleStore) *Review { return &Review{store: store} }
func (r *Review) Execute(path, id, status string) error {
	h, err := r.store.Load(path)
	if err != nil {
		return err
	}
	if err := h.Validate(); err != nil {
		return err
	}
	previous := ""
	for _, rule := range h.Rules {
		if rule.ID == id {
			previous = rule.Status
		}
	}
	if err := h.ReviewRule(id, status); err != nil {
		return err
	}
	if previous == status {
		return nil
	}
	return r.store.SaveStatus(path, id, previous, status)
}
