package application

import (
	"context"
	"fmt"
	embeddingdomain "harnessforge/internal/embeddings/domain"
	indexdomain "harnessforge/internal/indexing/domain"
	"harnessforge/internal/retrieval/domain"
	"sort"
	"strings"
)

type IndexReader interface {
	Load(string) (indexdomain.Index, error)
}
type QueryEmbedder interface {
	Embed(context.Context, string) ([]float64, int, error)
}
type Freshness interface {
	Stale(string, indexdomain.Index) ([]string, error)
}
type Search struct {
	reader    IndexReader
	embedder  QueryEmbedder
	freshness Freshness
	model     string
}

func NewSearch(r IndexReader, e QueryEmbedder, f Freshness, model string) *Search {
	return &Search{r, e, f, model}
}
func (s *Search) Execute(ctx context.Context, repository, query, path string, k int, relevant []string) (domain.Report, error) {
	if strings.TrimSpace(query) == "" {
		return domain.Report{}, fmt.Errorf("search query must not be empty")
	}
	if k < 1 || k > 100 {
		return domain.Report{}, fmt.Errorf("k must be between 1 and 100")
	}
	index, err := s.reader.Load(repository)
	if err != nil {
		return domain.Report{}, err
	}
	if len(index.Chunks) == 0 {
		return domain.Report{}, fmt.Errorf("index is empty; run index first")
	}
	if index.Model != s.model {
		return domain.Report{}, fmt.Errorf("index model %q is incompatible with query model %q", index.Model, s.model)
	}
	stale, err := s.freshness.Stale(repository, index)
	if err != nil {
		return domain.Report{}, err
	}
	if len(stale) > 0 {
		return domain.Report{}, fmt.Errorf("index is stale for %s; run index again", strings.Join(stale, ", "))
	}
	vector, _, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return domain.Report{}, err
	}
	if len(vector) != index.Dimensions {
		return domain.Report{}, fmt.Errorf("query dimensions are incompatible with the index")
	}
	var results []domain.Result
	for _, chunk := range index.Chunks {
		if err := ctx.Err(); err != nil {
			return domain.Report{}, err
		}
		if path != "" && !strings.HasPrefix(chunk.Source, path) {
			continue
		}
		score, err := embeddingdomain.Cosine(embeddingdomain.Vector{Model: s.model, Dimensions: len(vector), Values: vector}, embeddingdomain.Vector{Model: chunk.Model, Dimensions: len(chunk.Vector), Values: chunk.Vector})
		if err != nil {
			return domain.Report{}, err
		}
		results = append(results, domain.Result{Score: score, ChunkID: chunk.ID, Source: chunk.Source, StartLine: chunk.StartLine, EndLine: chunk.EndLine, Excerpt: chunk.Text})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ChunkID < results[j].ChunkID
	})
	if len(results) > k {
		results = results[:k]
	}
	for i := range results {
		results[i].Rank = i + 1
	}
	report := domain.Report{Query: query, Model: s.model, Results: results, Limitations: []string{"Lexical hash scores measure token overlap, not semantic truth or rule correctness.", "Search reads the index and never calls a text-generation model."}}
	if len(relevant) > 0 {
		wanted := map[string]bool{}
		for _, id := range relevant {
			wanted[id] = true
		}
		hits := 0
		for _, result := range results {
			if wanted[result.ChunkID] {
				hits++
			}
		}
		report.Metrics = &domain.Metrics{RecallAtK: float64(hits) / float64(len(wanted))}
		if len(results) > 0 {
			report.Metrics.PrecisionAtK = float64(hits) / float64(len(results))
		}
	}
	return report, nil
}
