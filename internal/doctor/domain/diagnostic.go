package domain

import "sort"

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Diagnostic struct {
	Code       string   `json:"code"`
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Evidence   []string `json:"evidence,omitempty"`
	Suggestion string   `json:"suggestion,omitempty"`
}

type Report struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Limitations []string     `json:"limitations"`
}

func (r *Report) Add(d Diagnostic) { r.Diagnostics = append(r.Diagnostics, d) }

func (r *Report) Sort() {
	sort.SliceStable(r.Diagnostics, func(i, j int) bool {
		if r.Diagnostics[i].Severity != r.Diagnostics[j].Severity {
			return r.Diagnostics[i].Severity < r.Diagnostics[j].Severity
		}
		return r.Diagnostics[i].Code < r.Diagnostics[j].Code
	})
}
