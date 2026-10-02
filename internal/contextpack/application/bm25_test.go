package application

import "testing"

func TestRankBM25PrefersRelevantDocumentAndIsDeterministic(t *testing.T) {
	documents := []BM25Document{
		{ID: "b", Text: "deployment notes and unrelated history"},
		{ID: "a", Text: "JWT authentication validates the token"},
	}
	first := RankBM25("where is JWT authentication validated", documents, 2)
	second := RankBM25("where is JWT authentication validated", documents, 2)
	if len(first) != 1 || first[0].ID != "a" {
		t.Fatalf("ranking = %#v", first)
	}
	if len(second) != len(first) || second[0] != first[0] {
		t.Fatalf("ranking is not deterministic: %#v vs %#v", first, second)
	}
}
