package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// KnowledgeKind classifies a durable piece of project knowledge.
type KnowledgeKind string

const (
	KnowledgeFact         KnowledgeKind = "fact"
	KnowledgeConvention   KnowledgeKind = "convention"
	KnowledgeBusinessRule KnowledgeKind = "business_rule"
	KnowledgeDecision     KnowledgeKind = "decision"
	KnowledgeConstraint   KnowledgeKind = "constraint"
)

type KnowledgeReviewState string

const (
	KnowledgeCandidate  KnowledgeReviewState = "candidate"
	KnowledgeApproved   KnowledgeReviewState = "approved"
	KnowledgeRejected   KnowledgeReviewState = "rejected"
	KnowledgeDeprecated KnowledgeReviewState = "deprecated"
)

// KnowledgeHealth is independent of review: approved knowledge can later become stale.
type KnowledgeHealth string

const (
	KnowledgeVerified KnowledgeHealth = "verified"
	KnowledgeStale    KnowledgeHealth = "stale"
	KnowledgeMissing  KnowledgeHealth = "missing"
	KnowledgeUnknown  KnowledgeHealth = "unknown"
)

// KnowledgeItem is a stable, reviewable unit of project knowledge.
type KnowledgeItem struct {
	ID                 string               `json:"id" yaml:"id"`
	Kind               KnowledgeKind        `json:"kind" yaml:"kind"`
	Scope              Scope                `json:"scope,omitempty" yaml:"scope,omitempty"`
	Content            string               `json:"content" yaml:"content"`
	Origin             string               `json:"origin" yaml:"origin"`
	Review             KnowledgeReviewState `json:"review" yaml:"review"`
	Health             KnowledgeHealth      `json:"health" yaml:"health"`
	Evidence           []KnowledgeEvidence  `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Reviewer           string               `json:"reviewer,omitempty" yaml:"reviewer,omitempty"`
	ReviewDiff         string               `json:"review_diff,omitempty" yaml:"review_diff,omitempty"`
	ContentSHA256      string               `json:"content_sha256,omitempty" yaml:"content_sha256,omitempty"`
	EvidenceSHA256     string               `json:"evidence_sha256,omitempty" yaml:"evidence_sha256,omitempty"`
	LegacyReviewStatus string               `json:"legacy_review_status,omitempty" yaml:"legacy_review_status,omitempty"`
	LegacyReview       *ReviewRecord        `json:"legacy_review,omitempty" yaml:"legacy_review,omitempty"`
}

// KnowledgeEvidence records a source location and the exact observed material.
type KnowledgeEvidence struct {
	Path      string `json:"path" yaml:"path"`
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Kind      string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Symbol    string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Quote     string `json:"quote,omitempty" yaml:"quote,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
	SHA256    string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Revision  string `json:"revision,omitempty" yaml:"revision,omitempty"`
}

// Validate checks the knowledge contract using only the supplied value.
// It does not read evidence paths or execute project commands.
func (k KnowledgeItem) Validate() error {
	if strings.TrimSpace(k.ID) == "" {
		return fmt.Errorf("id: must not be empty")
	}
	switch k.Kind {
	case KnowledgeFact, KnowledgeConvention, KnowledgeBusinessRule, KnowledgeDecision, KnowledgeConstraint:
	default:
		return fmt.Errorf("kind: unsupported value %q", k.Kind)
	}
	if strings.TrimSpace(k.Content) == "" {
		return fmt.Errorf("content: must not be empty")
	}
	if strings.TrimSpace(k.Origin) == "" {
		return fmt.Errorf("origin: must not be empty")
	}
	switch k.Review {
	case KnowledgeCandidate, KnowledgeApproved, KnowledgeRejected, KnowledgeDeprecated:
	default:
		return fmt.Errorf("review: unsupported value %q", k.Review)
	}
	switch k.Health {
	case KnowledgeVerified, KnowledgeStale, KnowledgeMissing, KnowledgeUnknown:
	default:
		return fmt.Errorf("health: unsupported value %q", k.Health)
	}
	if err := nonemptyList("scope.paths", k.Scope.Paths); err != nil {
		return err
	}
	if k.ContentSHA256 != "" && !validSHA256(k.ContentSHA256) {
		return fmt.Errorf("content_sha256: expected 64 hexadecimal characters")
	}
	if k.EvidenceSHA256 != "" && !validSHA256(k.EvidenceSHA256) {
		return fmt.Errorf("evidence_sha256: expected 64 hexadecimal characters")
	}
	if k.LegacyReviewStatus != "" && k.LegacyReviewStatus != "candidate" && k.LegacyReviewStatus != "approved" && k.LegacyReviewStatus != "rejected" {
		return fmt.Errorf("legacy_review_status: unsupported value %q", k.LegacyReviewStatus)
	}
	if k.Review != KnowledgeCandidate {
		if strings.TrimSpace(k.Reviewer) == "" {
			return fmt.Errorf("reviewer: required after candidate review")
		}
		if k.ContentSHA256 == "" {
			return fmt.Errorf("content_sha256: required after candidate review")
		}
		if k.EvidenceSHA256 == "" {
			return fmt.Errorf("evidence_sha256: required after candidate review")
		}
	}
	for i, evidence := range k.Evidence {
		field := fmt.Sprintf("evidence[%d]", i)
		if strings.TrimSpace(evidence.Path) == "" {
			return fmt.Errorf("%s.path: must not be empty", field)
		}
		if evidence.Workspace != "" && strings.TrimSpace(evidence.Workspace) == "" {
			return fmt.Errorf("%s.workspace: must not be blank", field)
		}
		if evidence.StartLine < 0 || evidence.EndLine < 0 || (evidence.EndLine > 0 && evidence.StartLine > evidence.EndLine) {
			return fmt.Errorf("%s: invalid line range", field)
		}
		if evidence.SHA256 != "" && !validSHA256(evidence.SHA256) {
			return fmt.Errorf("%s.sha256: expected 64 hexadecimal characters", field)
		}
	}
	return nil
}

// ValidateKnowledge checks item validity and uniqueness of stable IDs.
func ValidateKnowledge(items []KnowledgeItem) error {
	ids := make(map[string]bool, len(items))
	for i, item := range items {
		field := fmt.Sprintf("knowledge[%d]", i)
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if ids[item.ID] {
			return fmt.Errorf("%s.id: duplicate %q", field, item.ID)
		}
		ids[item.ID] = true
	}
	return nil
}

// HashKnowledgeContent returns the SHA-256 hash used to bind a review to content.
func HashKnowledgeContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// HashKnowledgeEvidence binds review to the ordered evidence file contents.
func HashKnowledgeEvidence(values []string) string {
	if values == nil {
		values = []string{}
	}
	encoded, _ := json.Marshal(values)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
