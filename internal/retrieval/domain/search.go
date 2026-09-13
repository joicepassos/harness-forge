package domain

type Result struct {
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	ChunkID   string  `json:"chunk_id"`
	Source    string  `json:"source"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Excerpt   string  `json:"excerpt"`
}
type Metrics struct {
	RecallAtK    float64 `json:"recall_at_k"`
	PrecisionAtK float64 `json:"precision_at_k"`
}
type Report struct {
	Query       string   `json:"query"`
	Model       string   `json:"model"`
	Results     []Result `json:"results"`
	Metrics     *Metrics `json:"metrics,omitempty"`
	Limitations []string `json:"limitations"`
}
