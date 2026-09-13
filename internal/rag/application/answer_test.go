package application

import (
	"context"
	"harnessforge/internal/rag/domain"
	"strings"
	"testing"
	"time"
)

type retriever struct{ sources []domain.Source }

func (r retriever) Retrieve(context.Context, string, string, int) ([]domain.Source, error) {
	return r.sources, nil
}

type generator struct {
	generation domain.Generation
	calls      int
}

func (g *generator) Generate(context.Context, string, []domain.Source) (domain.Generation, error) {
	g.calls++
	return g.generation, nil
}
func TestGroundedAnswerRequiresRetrievedCitationsAndRecordsUsage(t *testing.T) {
	g := &generator{generation: domain.Generation{Answer: "Use ports.", Citations: []string{"chunk-1"}, InputTokens: 10, OutputTokens: 4}}
	answer := NewAnswer(retriever{[]domain.Source{{ChunkID: "chunk-1", Path: "doc.md", Score: 1}}}, g)
	times := []time.Time{time.UnixMilli(0), time.UnixMilli(12)}
	answer.now = func() time.Time { value := times[0]; times = times[1:]; return value }
	result, err := answer.Execute(context.Background(), "repository", "question", 5, false)
	if err != nil || result.LatencyMS != 12 || result.InputTokens != 10 || result.Mode != "retrieval" {
		t.Fatal(result, err)
	}
}
func TestFabricatedCitationIsRejected(t *testing.T) {
	g := &generator{generation: domain.Generation{Answer: "Claim", Citations: []string{"invented"}}}
	_, err := NewAnswer(retriever{[]domain.Source{{ChunkID: "real", Score: 1}}}, g).Execute(context.Background(), "repository", "question", 5, false)
	if err == nil || !strings.Contains(err.Error(), "fabricated") {
		t.Fatal(err)
	}
}
func TestInsufficientEvidenceSkipsGeneration(t *testing.T) {
	g := &generator{}
	result, err := NewAnswer(retriever{[]domain.Source{{ChunkID: "zero", Score: 0}}}, g).Execute(context.Background(), "repository", "question", 5, false)
	if err != nil || !result.InsufficientEvidence || g.calls != 0 {
		t.Fatal(result, err)
	}
}
func TestDirectModeCannotClaimRepositoryCitation(t *testing.T) {
	g := &generator{generation: domain.Generation{Answer: "Direct", Citations: []string{"claim"}}}
	_, err := NewAnswer(retriever{}, g).Execute(context.Background(), "", "question", 5, true)
	if err == nil {
		t.Fatal("direct citation accepted")
	}
}

func TestContextBudgetExcludesOversizedSourcesWithoutCallingProvider(t *testing.T) {
	g := &generator{}
	sources := []domain.Source{{ChunkID: "large", Text: strings.Repeat("x", maxContextBytes+1), Score: 1}}
	result, err := NewAnswer(retriever{sources}, g).Execute(context.Background(), "repository", "question", 5, false)
	if err != nil || !result.InsufficientEvidence || result.ExcludedSources != 1 || g.calls != 0 {
		t.Fatal(result, err)
	}
}
