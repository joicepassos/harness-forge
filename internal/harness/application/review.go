package application

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"harnessforge/internal/harness/domain"
)

type RuleStore interface {
	Load(string) (domain.Harness, error)
	SaveStatus(path, id, previous, status string, record *domain.ReviewRecord) error
}
type Review struct {
	store    RuleStore
	evidence EvidenceChecker
}

type EvidenceChecker interface {
	Fingerprint(path, id string, evidence []domain.Evidence) (string, error)
}

func NewReview(store RuleStore, checkers ...EvidenceChecker) *Review {
	r := &Review{store: store}
	if len(checkers) > 0 {
		r.evidence = checkers[0]
	}
	return r
}
func (r *Review) Execute(path, id, status string) error {
	h, err := r.store.Load(path)
	if err != nil {
		return err
	}
	if err := h.Validate(); err != nil {
		return err
	}
	previous := ""
	var selected *domain.Rule
	for i := range h.Rules {
		rule := &h.Rules[i]
		if rule.ID == id {
			previous = rule.Status
			selected = rule
		}
	}
	if selected == nil {
		return h.ReviewRule(id, status)
	}
	if status == "approved" {
		if len(selected.Evidence) > 0 && r.evidence == nil {
			return fmt.Errorf("cannot approve a rule with evidence without revalidating it")
		}
		emptyEvidenceHash := sha256.Sum256([]byte("[]"))
		evidenceFingerprint := hex.EncodeToString(emptyEvidenceHash[:])
		if r.evidence != nil {
			fingerprint, err := r.evidence.Fingerprint(path, id, selected.Evidence)
			if err != nil {
				return fmt.Errorf("evidence changed or is invalid; review again: %w", err)
			}
			evidenceFingerprint = fingerprint
		}
		contentHash, err := domain.RuleContentHash(*selected)
		if err != nil {
			return err
		}
		if selected.Status == "approved" && selected.Review != nil {
			if selected.Review.ContentSHA256 != contentHash || selected.Review.EvidenceSHA256 != evidenceFingerprint {
				return fmt.Errorf("approved rule or evidence changed; reopen as candidate and review again")
			}
		}
		selected.Review = &domain.ReviewRecord{ContentSHA256: contentHash, EvidenceSHA256: evidenceFingerprint}
	} else if status == "candidate" {
		selected.Review = nil
	}
	if previous != status {
		if err := h.ReviewRule(id, status); err != nil {
			return err
		}
	}
	if previous == status {
		return nil
	}
	return r.store.SaveStatus(path, id, previous, status, selected.Review)
}
