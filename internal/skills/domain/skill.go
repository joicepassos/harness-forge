package domain

type Evidence struct {
	File   string `json:"file"`
	Symbol string `json:"symbol,omitempty"`
}

type Proposal struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Examples    []Evidence `json:"examples"`
	Limitations []string   `json:"limitations"`
}
