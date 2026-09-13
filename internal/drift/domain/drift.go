package domain

type Status string

const (
	StatusAligned      Status = "aligned"
	StatusDifference   Status = "difference"
	StatusNotEvaluated Status = "not_evaluated"
)

type Occurrence struct {
	RuleID          string   `json:"rule_id"`
	Status          Status   `json:"status"`
	Locations       []string `json:"locations,omitempty"`
	Explanations    []string `json:"possible_explanations,omitempty"`
	CodeProposal    string   `json:"code_fix_proposal,omitempty"`
	HarnessProposal string   `json:"harness_update_proposal,omitempty"`
}

type Report struct {
	Occurrences []Occurrence `json:"occurrences"`
	Limitations []string     `json:"limitations"`
}
