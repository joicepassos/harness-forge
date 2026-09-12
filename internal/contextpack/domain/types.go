package domain

const DefaultBudgetTokens = 1800

type Options struct {
	BudgetTokens    int
	MaxFiles        int
	MaxBytesPerFile int
}

type Plan struct {
	BudgetTokens         int        `json:"budget_tokens"`
	EstimatedTokens      int        `json:"estimated_tokens"`
	Estimator            string     `json:"estimator"`
	PayloadReserveTokens int        `json:"payload_reserve_tokens"`
	Included             []Excerpt  `json:"included"`
	Excluded             []Excerpt  `json:"excluded"`
	Comparison           Comparison `json:"comparison"`
}

type Excerpt struct {
	ID              string   `json:"id"`
	Source          string   `json:"source"`
	Path            string   `json:"path,omitempty"`
	Text            string   `json:"text"`
	Relevance       int      `json:"relevance"`
	EstimatedTokens int      `json:"estimated_tokens"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason"`
	Origins         []string `json:"origins"`
	CompressedFrom  string   `json:"compressed_from,omitempty"`
	Rank            int      `json:"rank,omitempty"`
}

type Comparison struct {
	PreviousAnalyzerJSONEstimatedTokens int    `json:"previous_analyzer_json_estimated_tokens"`
	UnfilteredCandidateEstimatedTokens  int    `json:"unfiltered_candidate_estimated_tokens"`
	SelectedEstimatedTokens             int    `json:"selected_estimated_tokens"`
	AnalyzerJSONReductionPercent        int    `json:"analyzer_json_reduction_percent"`
	UnfilteredCandidateReductionPercent int    `json:"unfiltered_candidate_reduction_percent"`
	PreviousRelevantRecallPercent       int    `json:"previous_relevant_recall_percent"`
	SelectedRelevantRecallPercent       int    `json:"selected_relevant_recall_percent"`
	Quality                             string `json:"quality"`
}

type Metrics struct {
	PreviousAnalyzerJSONEstimatedTokens int
	UnfilteredCandidateEstimatedTokens  int
	PreviousRelevantRecallPercent       int
}
