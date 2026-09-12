package domain

type Dataset struct {
	Version        int    `yaml:"version" json:"version"`
	DatasetVersion string `yaml:"dataset_version" json:"dataset_version"`
	Cases          []Case `yaml:"cases" json:"cases"`
}

type Case struct {
	ID              string   `yaml:"id" json:"id"`
	Query           string   `yaml:"query" json:"query"`
	RelevantSources []string `yaml:"relevant_sources" json:"relevant_sources"`
	RequiredTerms   []string `yaml:"required_terms" json:"required_terms"`
}

type Results struct {
	IndexVersion  string       `yaml:"index_version" json:"index_version"`
	Model         string       `yaml:"model" json:"model"`
	PromptVersion string       `yaml:"prompt_version" json:"prompt_version"`
	RubricVersion string       `yaml:"rubric_version" json:"rubric_version"`
	Cases         []CaseResult `yaml:"cases" json:"cases"`
}

type CaseResult struct {
	ID               string   `yaml:"id" json:"id"`
	RetrievedSources []string `yaml:"retrieved_sources" json:"retrieved_sources"`
	CitedSources     []string `yaml:"cited_sources" json:"cited_sources"`
	Answer           string   `yaml:"answer" json:"answer"`
	Tokens           int      `yaml:"tokens" json:"tokens"`
	LatencyMS        int      `yaml:"latency_ms" json:"latency_ms"`
	Error            string   `yaml:"error" json:"error"`
}

type Report struct {
	DatasetVersion string       `json:"dataset_version"`
	IndexVersion   string       `json:"index_version"`
	Model          string       `json:"model"`
	PromptVersion  string       `json:"prompt_version"`
	RubricVersion  string       `json:"rubric_version"`
	Cases          []CaseReport `json:"cases"`
	Aggregate      Aggregate    `json:"aggregate"`
	Limitations    []string     `json:"limitations"`
}

type CaseReport struct {
	ID           string  `json:"id"`
	RecallAtK    float64 `json:"recall_at_k"`
	PrecisionAtK float64 `json:"precision_at_k"`
	Correctness  bool    `json:"correctness"`
	Faithfulness bool    `json:"faithfulness"`
	Tokens       int     `json:"tokens"`
	LatencyMS    int     `json:"latency_ms"`
	Error        string  `json:"error,omitempty"`
}

type Aggregate struct {
	RecallAtK    float64 `json:"recall_at_k"`
	PrecisionAtK float64 `json:"precision_at_k"`
	Correctness  float64 `json:"correctness"`
	Faithfulness float64 `json:"faithfulness"`
	Tokens       int     `json:"tokens"`
	LatencyMS    int     `json:"latency_ms"`
	Failures     int     `json:"failures"`
}

type Comparison struct {
	Baseline  Aggregate `json:"baseline"`
	Candidate Aggregate `json:"candidate"`
	Delta     Aggregate `json:"delta"`
}
