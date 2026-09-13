package domain

type Source struct {
	ChunkID   string  `json:"chunk_id"`
	Path      string  `json:"path"`
	Text      string  `json:"text"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Score     float64 `json:"score"`
}
type Generation struct {
	Answer                    string   `json:"answer"`
	Citations                 []string `json:"citations"`
	InsufficientEvidence      bool     `json:"insufficient_evidence"`
	InputTokens, OutputTokens int
}
type Answer struct {
	Answer               string   `json:"answer"`
	Citations            []string `json:"citations"`
	Sources              []Source `json:"sources"`
	InsufficientEvidence bool     `json:"insufficient_evidence"`
	InputTokens          int      `json:"input_tokens"`
	OutputTokens         int      `json:"output_tokens"`
	LatencyMS            int64    `json:"latency_ms"`
	ContextBytes         int      `json:"context_bytes"`
	ExcludedSources      int      `json:"excluded_sources"`
	Mode                 string   `json:"mode"`
	Limitations          []string `json:"limitations"`
}
