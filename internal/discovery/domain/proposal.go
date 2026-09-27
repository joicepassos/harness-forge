package domain

type Evidence struct {
	File      string `json:"file"`
	Kind      string `json:"kind,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Quote     string `json:"quote,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Revision  string `json:"revision,omitempty"`
}
type Proposal struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Evidence    []Evidence `json:"evidence"`
	Confidence  float64    `json:"confidence"`
	Limitations []string   `json:"limitations"`
}
