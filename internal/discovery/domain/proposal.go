package domain

type Evidence struct {
	File     string `json:"file"`
	Symbol   string `json:"symbol,omitempty"`
	Revision string `json:"revision,omitempty"`
}
type Proposal struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Evidence    []Evidence `json:"evidence"`
	Confidence  float64    `json:"confidence"`
	Limitations []string   `json:"limitations"`
}
