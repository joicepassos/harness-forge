package domain

type Symbol struct {
	Kind       string   `json:"kind"`
	Name       string   `json:"name"`
	File       string   `json:"file"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Receiver   string   `json:"receiver,omitempty"`
	Implements []string `json:"implements,omitempty"`
}

type Convention struct {
	Kind     string `json:"kind"`
	Matching int    `json:"matching"`
	Total    int    `json:"total"`
}

type Report struct {
	Language    string       `json:"language"`
	Symbols     []Symbol     `json:"symbols"`
	Conventions []Convention `json:"conventions"`
	Limitations []string     `json:"limitations"`
}
