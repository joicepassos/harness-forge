package domain

type Discussion struct {
	Kind     string `json:"kind"`
	Number   int    `json:"number"`
	Revision string `json:"revision,omitempty"`
	URL      string `json:"url"`
	Author   string `json:"author,omitempty"`
	Body     string `json:"body"`
}

type Candidate struct {
	Statement           string   `json:"statement"`
	Classification      string   `json:"classification"`
	Sources             []string `json:"sources"`
	RequiresHumanReview bool     `json:"requires_human_review"`
}

type Report struct {
	Discussions []Discussion `json:"discussions"`
	Candidates  []Candidate  `json:"candidates"`
	Limitations []string     `json:"limitations"`
}
