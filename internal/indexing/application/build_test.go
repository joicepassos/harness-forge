package application

import (
	"context"
	"harnessforge/internal/indexing/domain"
	"testing"
)

type source struct{ documents []domain.Document }

func (s source) Documents(context.Context, string) ([]domain.Document, error) {
	return s.documents, nil
}

type chunker struct{}

func (chunker) Chunks(d domain.Document) []domain.Chunk {
	return []domain.Chunk{{ID: d.Path, Source: d.Path, SourceHash: d.Hash, Text: d.Text, Hash: d.Hash}}
}

type embedder struct{ calls int }

func (e *embedder) Embed(context.Context, string) ([]float64, int, error) {
	e.calls++
	return []float64{1, 0}, 2, nil
}

type store struct{ index domain.Index }

func (s *store) Load(string) (domain.Index, error)       { return s.index, nil }
func (s *store) Save(_ string, index domain.Index) error { s.index = index; return nil }
func TestIncrementalIndexReusesUpdatesAndRemoves(t *testing.T) {
	e := &embedder{}
	s := &store{index: domain.Index{Chunks: []domain.Chunk{{ID: "same.md", Hash: "a", Model: "m", Vector: []float64{1, 0}}, {ID: "changed.md", Hash: "old", Model: "m", Vector: []float64{1, 0}}, {ID: "deleted.md", Hash: "old", Model: "m", Vector: []float64{1, 0}}}}}
	documents := []domain.Document{{Path: "same.md", Text: "same", Hash: "a"}, {Path: "changed.md", Text: "new", Hash: "b"}, {Path: "added.md", Text: "add", Hash: "c"}}
	report, err := NewBuild(source{documents}, chunker{}, e, s, "m").Execute(context.Background(), "repository")
	if err != nil {
		t.Fatal(err)
	}
	if report.Reused != 1 || report.Updated != 1 || report.Added != 1 || report.Removed != 1 || e.calls != 2 {
		t.Fatalf("%#v calls=%d", report, e.calls)
	}
	e.calls = 0
	report, err = NewBuild(source{documents}, chunker{}, e, s, "m").Execute(context.Background(), "repository")
	if err != nil || report.Reused != 3 || report.Embedded != 0 || e.calls != 0 {
		t.Fatalf("%#v %v", report, err)
	}
}
