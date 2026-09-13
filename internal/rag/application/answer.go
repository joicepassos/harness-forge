package application

import (
	"context"
	"fmt"
	"harnessforge/internal/rag/domain"
	"strings"
	"time"
)

const maxContextBytes = 32 * 1024

type Retriever interface {
	Retrieve(context.Context, string, string, int) ([]domain.Source, error)
}
type Generator interface {
	Generate(context.Context, string, []domain.Source) (domain.Generation, error)
}
type Answer struct {
	retriever Retriever
	generator Generator
	now       func() time.Time
}

func NewAnswer(r Retriever, g Generator) *Answer { return &Answer{r, g, time.Now} }
func (a *Answer) Execute(ctx context.Context, repository, query string, k int, direct bool) (domain.Answer, error) {
	if strings.TrimSpace(query) == "" {
		return domain.Answer{}, fmt.Errorf("query must not be empty")
	}
	start := a.now()
	mode := "retrieval"
	var sources []domain.Source
	excludedSources := 0
	var err error
	if direct {
		mode = "direct"
	} else {
		sources, err = a.retriever.Retrieve(ctx, repository, query, k)
		if err != nil {
			return domain.Answer{}, err
		}
		if len(sources) == 0 || sources[0].Score <= 0 {
			return domain.Answer{Answer: "Insufficient repository evidence.", Sources: sources, InsufficientEvidence: true, LatencyMS: a.now().Sub(start).Milliseconds(), Mode: mode, Limitations: limitations()}, nil
		}
		retrievedCount := len(sources)
		sources = boundedSources(sources, maxContextBytes)
		excludedSources = retrievedCount - len(sources)
		if len(sources) == 0 {
			return domain.Answer{Answer: "Insufficient repository evidence: retrieved context exceeds the safe context budget.", InsufficientEvidence: true, LatencyMS: a.now().Sub(start).Milliseconds(), ExcludedSources: excludedSources, Mode: mode, Limitations: limitations()}, nil
		}
	}
	generation, err := a.generator.Generate(ctx, query, sources)
	if err != nil {
		return domain.Answer{}, err
	}
	available := map[string]bool{}
	for _, source := range sources {
		available[source.ChunkID] = true
	}
	if !direct {
		if len(generation.Citations) == 0 && !generation.InsufficientEvidence {
			return domain.Answer{}, fmt.Errorf("grounded answer must cite a retrieved chunk")
		}
		for _, citation := range generation.Citations {
			if !available[citation] {
				return domain.Answer{}, fmt.Errorf("fabricated citation %q", citation)
			}
		}
	} else if len(generation.Citations) > 0 {
		return domain.Answer{}, fmt.Errorf("direct answers cannot claim repository citations")
	}
	contextBytes := 0
	for _, source := range sources {
		contextBytes += len(source.Text)
	}
	return domain.Answer{Answer: generation.Answer, Citations: generation.Citations, Sources: sources, InsufficientEvidence: generation.InsufficientEvidence, InputTokens: generation.InputTokens, OutputTokens: generation.OutputTokens, LatencyMS: a.now().Sub(start).Milliseconds(), ContextBytes: contextBytes, ExcludedSources: excludedSources, Mode: mode, Limitations: limitations()}, nil
}

func boundedSources(sources []domain.Source, budget int) []domain.Source {
	bounded := make([]domain.Source, 0, len(sources))
	used := 0
	for _, source := range sources {
		if len(source.Text) > budget-used {
			break
		}
		bounded = append(bounded, source)
		used += len(source.Text)
	}
	return bounded
}
func limitations() []string {
	return []string{"Retrieved citations prove only that text was supplied to the provider, not that the answer is correct.", "Direct mode does not inspect repository evidence and cannot return repository citations."}
}
