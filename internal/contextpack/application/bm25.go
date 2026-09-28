package application

import (
	"math"
	"sort"
	"strings"
)

// BM25Document is a lexical retrieval candidate. IDs are returned unchanged
// so callers can attach scores without coupling retrieval to context format.
type BM25Document struct {
	ID   string
	Text string
}

type BM25Result struct {
	ID    string
	Score float64
}

// RankBM25 provides a deterministic lexical baseline for retrieval evaluation.
// It deliberately has no repository or provider dependencies.
func RankBM25(query string, documents []BM25Document, limit int) []BM25Result {
	if limit <= 0 || len(documents) == 0 {
		return nil
	}
	terms := Terms(query)
	if len(terms) == 0 {
		return nil
	}
	docTerms := make([]map[string]int, len(documents))
	df := map[string]int{}
	totalLength := 0
	for i, document := range documents {
		docTerms[i] = termCounts(document.Text)
		totalLength += sumCounts(docTerms[i])
		for term := range terms {
			if docTerms[i][term] > 0 {
				df[term]++
			}
		}
	}
	avgLength := float64(totalLength) / float64(len(documents))
	if avgLength == 0 {
		return nil
	}
	results := make([]BM25Result, 0, len(documents))
	for i, document := range documents {
		length := float64(sumCounts(docTerms[i]))
		score := 0.0
		for term := range terms {
			frequency := float64(docTerms[i][term])
			if frequency == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(documents)-df[term])+0.5)/(float64(df[term])+0.5))
			score += idf * frequency * 2.2 / (frequency + 1.2*(1-0.75+0.75*length/avgLength))
		}
		if score > 0 {
			results = append(results, BM25Result{ID: document.ID, Score: score})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func termCounts(text string) map[string]int {
	counts := map[string]int{}
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if len(term) >= 3 && !stopwords[term] {
			counts[term]++
		}
	}
	return counts
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
