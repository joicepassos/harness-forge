package domain

type Status string

const (
	StatusAligned      Status = "aligned"
	StatusDifference   Status = "difference"
	StatusNotEvaluated Status = "not_evaluated"
)

const (
	EvidencePresent Status = "present"
	EvidenceMissing Status = "missing"
	EvidenceChanged Status = "changed"
)

type Occurrence struct {
	SubjectType     string   `json:"subject_type,omitempty"`
	RuleID          string   `json:"rule_id"`
	Status          Status   `json:"status"`
	EvidenceStatus  Status   `json:"evidence_status"`
	Conformance     Status   `json:"conformance_status"`
	Locations       []string `json:"locations,omitempty"`
	Revision        string   `json:"revision,omitempty"`
	BaselineStatus  string   `json:"baseline_status,omitempty"`
	Explanations    []string `json:"possible_explanations,omitempty"`
	CodeProposal    string   `json:"code_fix_proposal,omitempty"`
	HarnessProposal string   `json:"harness_update_proposal,omitempty"`
}

type Report struct {
	Occurrences []Occurrence `json:"occurrences"`
	Coverage    Coverage     `json:"coverage"`
	Limitations []string     `json:"limitations"`
}

// Coverage describes which approved evidence items the current strategy could
// inspect. It is deliberately separate from rule conformance.
type Coverage struct {
	Total                int `json:"total"`
	Evaluated            int `json:"evaluated"`
	NotEvaluated         int `json:"not_evaluated"`
	RulesWithoutEvidence int `json:"rules_without_evidence"`
}
