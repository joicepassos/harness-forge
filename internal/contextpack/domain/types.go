package domain

import "context"

const DefaultBudgetTokens = 1800

type Options struct {
	BudgetTokens    int
	MaxFiles        int
	MaxBytesPerFile int
	// TaskPaths identifies the repository-relative files the task concerns.
	// Approved knowledge with path scopes is eligible only when one of these
	// paths matches its scope.
	TaskPaths []string
	UseBM25   bool
	UseMMR    bool
	Model     string
	Layout    string
	Counter   TokenCounter
}

// TokenCounter lets a provider supply model-specific input token accounting
// without making the context domain depend on a provider implementation.
type TokenCounter interface {
	Name() string
	Count(context.Context, string, []byte) (int, error)
}

type Plan struct {
	BudgetTokens         int        `json:"budget_tokens"`
	EstimatedTokens      int        `json:"estimated_tokens"`
	Estimator            string     `json:"estimator"`
	PayloadReserveTokens int        `json:"payload_reserve_tokens"`
	Included             []Excerpt  `json:"included"`
	Excluded             []Excerpt  `json:"excluded"`
	Comparison           Comparison `json:"comparison"`
	BudgetOverflow       bool       `json:"budget_overflow"`
	OverflowExcerptIDs   []string   `json:"overflow_excerpt_ids,omitempty"`
	OverflowTokens       int        `json:"overflow_tokens,omitempty"`
}

type Excerpt struct {
	ID              string   `json:"id"`
	Source          string   `json:"source"`
	Path            string   `json:"path,omitempty"`
	Workspace       string   `json:"workspace,omitempty"`
	Text            string   `json:"text"`
	Relevance       int      `json:"relevance"`
	EstimatedTokens int      `json:"estimated_tokens"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason"`
	Origins         []string `json:"origins"`
	CompressedFrom  string   `json:"compressed_from,omitempty"`
	Rank            int      `json:"rank,omitempty"`
	KnowledgeID     string   `json:"knowledge_id,omitempty"`
	KnowledgeScope  []string `json:"knowledge_scope,omitempty"`
}

// Source preserves every selected excerpt, including multiple excerpts from
// the same file. The legacy map-based payload cannot represent that safely.
type Source struct {
	ID          string   `json:"id"`
	Path        string   `json:"path,omitempty"`
	Workspace   string   `json:"workspace,omitempty"`
	StartLine   int      `json:"start_line,omitempty"`
	EndLine     int      `json:"end_line,omitempty"`
	Content     string   `json:"content"`
	KnowledgeID string   `json:"knowledge_id,omitempty"`
	Origins     []string `json:"origins,omitempty"`
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
