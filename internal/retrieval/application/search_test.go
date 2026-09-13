package application

import (
	"context"
	indexdomain "harnessforge/internal/indexing/domain"
	"strings"
	"testing"
)

type reader struct{ index indexdomain.Index }

func (r reader) Load(string) (indexdomain.Index, error) { return r.index, nil }

type embedder struct{ vector []float64 }

func (e embedder) Embed(context.Context, string) ([]float64, int, error) { return e.vector, 1, nil }

type freshness struct{ stale []string }

func (f freshness) Stale(string, indexdomain.Index) ([]string, error) { return f.stale, nil }
func fixtureIndex() indexdomain.Index {
	return indexdomain.Index{Model: "m", Dimensions: 2, Chunks: []indexdomain.Chunk{{ID: "b", Source: "docs/b.md", Text: "beta", Model: "m", Vector: []float64{0, 1}}, {ID: "a", Source: "docs/a.md", Text: "alpha", Model: "m", Vector: []float64{1, 0}}}}
}
func TestSearchRanksFiltersAndMeasures(t *testing.T) {
	report, err := NewSearch(reader{fixtureIndex()}, embedder{[]float64{1, 0}}, freshness{}, "m").Execute(context.Background(), "repo", "alpha", "docs/", 1, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Results) != 1 || report.Results[0].ChunkID != "a" || report.Results[0].Rank != 1 || report.Metrics.RecallAtK != 1 || report.Metrics.PrecisionAtK != 1 {
		t.Fatal(report)
	}
}
func TestSearchSurfacesEmptyStaleAndIncompatibleIndexes(t *testing.T) {
	cases := []struct {
		s    *Search
		want string
	}{{NewSearch(reader{}, embedder{}, freshness{}, "m"), "empty"}, {NewSearch(reader{fixtureIndex()}, embedder{[]float64{1, 0}}, freshness{[]string{"docs/a.md"}}, "m"), "stale"}, {NewSearch(reader{fixtureIndex()}, embedder{[]float64{1, 0}}, freshness{}, "other"), "incompatible"}, {NewSearch(reader{fixtureIndex()}, embedder{[]float64{1}}, freshness{}, "m"), "dimensions"}}
	for _, tc := range cases {
		_, err := tc.s.Execute(context.Background(), "repo", "query", "", 5, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("wanted %s: %v", tc.want, err)
		}
	}
}
