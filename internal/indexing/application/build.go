package application

import (
	"context"
	"fmt"
	"harnessforge/internal/indexing/domain"
)

type Source interface {
	Documents(context.Context, string) ([]domain.Document, error)
}
type Chunker interface {
	Chunks(domain.Document) []domain.Chunk
}
type Embedder interface {
	Embed(context.Context, string) ([]float64, int, error)
}
type Store interface {
	Load(string) (domain.Index, error)
	Save(string, domain.Index) error
}
type Build struct {
	source   Source
	chunker  Chunker
	embedder Embedder
	store    Store
	model    string
}

func NewBuild(s Source, c Chunker, e Embedder, store Store, model string) *Build {
	return &Build{s, c, e, store, model}
}
func (b *Build) Execute(ctx context.Context, repository string) (domain.Report, error) {
	previous, err := b.store.Load(repository)
	if err != nil {
		return domain.Report{}, err
	}
	documents, err := b.source.Documents(ctx, repository)
	if err != nil {
		return domain.Report{}, err
	}
	old := map[string]domain.Chunk{}
	for _, chunk := range previous.Chunks {
		old[chunk.ID] = chunk
	}
	next := domain.Index{Version: "index-v1", Model: b.model}
	report := domain.Report{Limitations: []string{"The local JSON store is suitable for one process and small repositories; PostgreSQL with pgvector remains the recommended evaluated option for concurrent or large indexes.", "Only Markdown and README documents are indexed; code and AST indexing are deferred."}}
	seen := map[string]bool{}
	for _, document := range documents {
		for _, chunk := range b.chunker.Chunks(document) {
			if err := ctx.Err(); err != nil {
				return domain.Report{}, err
			}
			seen[chunk.ID] = true
			if prior, ok := old[chunk.ID]; ok && prior.Hash == chunk.Hash && prior.Model == b.model {
				chunk.Vector = prior.Vector
				report.Reused++
			} else {
				chunk.Vector, report.Tokens, err = b.embed(ctx, chunk.Text, report.Tokens)
				if err != nil {
					return domain.Report{}, fmt.Errorf("embed %s: %w", chunk.ID, err)
				}
				report.Embedded++
				if ok {
					report.Updated++
				} else {
					report.Added++
				}
			}
			chunk.Model = b.model
			if next.Dimensions == 0 {
				next.Dimensions = len(chunk.Vector)
			} else if len(chunk.Vector) != next.Dimensions {
				return domain.Report{}, fmt.Errorf("embedding dimensions changed within index")
			}
			next.Chunks = append(next.Chunks, chunk)
		}
	}
	for id := range old {
		if !seen[id] {
			report.Removed++
		}
	}
	if err := b.store.Save(repository, next); err != nil {
		return domain.Report{}, err
	}
	return report, nil
}
func (b *Build) embed(ctx context.Context, text string, tokens int) ([]float64, int, error) {
	vector, used, err := b.embedder.Embed(ctx, text)
	return vector, tokens + used, err
}
