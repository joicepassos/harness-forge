package application

import (
	"encoding/json"
	"fmt"
	"harnessforge/internal/contextpack/domain"
	"sort"
	"strings"
	"unicode"
)

const (
	framingTokens    = 140
	estimatorName    = "payload-byte-upper-bound-v1"
	MaxExcerptTokens = 480
)

func Select(candidates []domain.Excerpt, prompt string, budget int, metrics domain.Metrics) *domain.Plan {
	if budget <= 0 {
		budget = domain.DefaultBudgetTokens
	}
	for i := range candidates {
		candidates[i].Relevance += Relevance(prompt, candidates[i].Path, candidates[i].Text)
		if candidates[i].EstimatedTokens > MaxExcerptTokens {
			original := candidates[i]
			candidates[i].Text = Compress(candidates[i].Text)
			candidates[i].EstimatedTokens = EstimateTokens(candidates[i].Text)
			candidates[i].CompressedFrom = original.ID
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Relevance != b.Relevance {
			return a.Relevance > b.Relevance
		}
		if a.EstimatedTokens != b.EstimatedTokens {
			return a.EstimatedTokens < b.EstimatedTokens
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		return a.ID < b.ID
	})

	seen := map[string]int{}
	seenAny := map[string]string{}
	relevantTotal := 0
	relevantIncluded := 0
	var included, excluded []domain.Excerpt
	for _, candidate := range candidates {
		if candidate.Status != "excluded" && candidate.Relevance > 0 {
			relevantTotal++
		}
		if candidate.Status == "excluded" {
			excluded = append(excluded, candidate)
			continue
		}
		key := dedupKey(candidate.Text)
		if key == "" {
			candidate.Status = "excluded"
			candidate.Reason = "empty after normalization"
			excluded = append(excluded, candidate)
			continue
		}
		if firstID, ok := seenAny[key]; ok {
			if index, includedDuplicate := seen[key]; includedDuplicate {
				included[index].Origins = appendUnique(included[index].Origins, candidate.Origins...)
			}
			candidate.Status = "excluded"
			candidate.Reason = "duplicate of " + firstID
			excluded = append(excluded, candidate)
			continue
		}
		seenAny[key] = candidate.ID
		if candidate.Relevance <= 0 {
			candidate.Status = "excluded"
			candidate.Reason = "not relevant to the prompt"
			excluded = append(excluded, candidate)
			continue
		}
		nextUsed := serializedEstimate(prompt, append(append([]domain.Excerpt{}, included...), candidate))
		if nextUsed > budget {
			candidate.Status = "excluded"
			candidate.Reason = "would exceed context budget"
			excluded = append(excluded, candidate)
			continue
		}
		priorReason := strings.TrimSpace(candidate.Reason)
		candidate.Status = "included"
		candidate.Reason = "selected within budget"
		if candidate.CompressedFrom != "" {
			candidate.Reason = "compressed and selected within budget"
		}
		if priorReason != "" {
			candidate.Reason = priorReason + "; " + candidate.Reason
		}
		candidate.Rank = len(included) + 1
		seen[key] = len(included)
		relevantIncluded++
		included = append(included, candidate)
	}
	used := serializedEstimate(prompt, included)

	analyzerReduction := 0
	if metrics.PreviousAnalyzerJSONEstimatedTokens > 0 {
		analyzerReduction = (metrics.PreviousAnalyzerJSONEstimatedTokens - used) * 100 / metrics.PreviousAnalyzerJSONEstimatedTokens
	}
	unfilteredReduction := 0
	if metrics.UnfilteredCandidateEstimatedTokens > 0 {
		unfilteredReduction = (metrics.UnfilteredCandidateEstimatedTokens - used) * 100 / metrics.UnfilteredCandidateEstimatedTokens
	}
	recall := 100
	if relevantTotal > 0 {
		recall = relevantIncluded * 100 / relevantTotal
	}
	return &domain.Plan{
		BudgetTokens:         budget,
		EstimatedTokens:      used,
		Estimator:            estimatorName,
		PayloadReserveTokens: framingTokens,
		Included:             included,
		Excluded:             excluded,
		Comparison: domain.Comparison{
			PreviousAnalyzerJSONEstimatedTokens: metrics.PreviousAnalyzerJSONEstimatedTokens,
			UnfilteredCandidateEstimatedTokens:  metrics.UnfilteredCandidateEstimatedTokens,
			SelectedEstimatedTokens:             used,
			AnalyzerJSONReductionPercent:        analyzerReduction,
			UnfilteredCandidateReductionPercent: unfilteredReduction,
			PreviousRelevantRecallPercent:       metrics.PreviousRelevantRecallPercent,
			SelectedRelevantRecallPercent:       recall,
			Quality:                             "proxy only: recall counts deterministic prompt-matched candidates retained after deduplication, compression and budget selection; answer quality still requires provider evaluation",
		},
	}
}

func Sources(prompt string, plan *domain.Plan) map[string]string {
	sources := map[string]string{"prompt": prompt}
	for _, excerpt := range plan.Included {
		sources[excerpt.Source] = excerpt.Text
	}
	return sources
}

func RequiredTokens(prompt string) int {
	return serializedEstimate(prompt, nil)
}

func EncodePrompt(prompt string, plan *domain.Plan) (string, map[string]string, error) {
	sources := Sources(prompt, plan)
	data, err := json.Marshal(sources)
	if err != nil {
		return "", nil, err
	}
	return string(data), sources, nil
}

func serializedEstimate(prompt string, included []domain.Excerpt) int {
	sources := map[string]string{"prompt": prompt}
	for _, excerpt := range included {
		sources[excerpt.Source] = excerpt.Text
	}
	data, err := json.Marshal(sources)
	if err != nil {
		return framingTokens + EstimateTokens(prompt)
	}
	return framingTokens + EstimateTokens(string(data))
}

func Relevance(prompt, path, text string) int {
	terms := Terms(prompt)
	if len(terms) == 0 {
		return 1
	}
	score := 0
	body := strings.ToLower(path + " " + text)
	for term := range terms {
		for _, variant := range variants(term) {
			if strings.Contains(body, variant) {
				score += 20
				break
			}
		}
	}
	if score > 0 {
		switch {
		case strings.Contains(path, "internal/"):
			score += 4
		case strings.Contains(path, "cmd/"):
			score += 3
		}
	}
	return score
}

func EstimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]byte(trimmed))
}

func TrimExcerpt(text string) string {
	lines := strings.Split(text, "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kept = append(kept, line)
		if EstimateTokens(strings.Join(kept, "\n")) >= MaxExcerptTokens {
			break
		}
	}
	return Compress(strings.Join(kept, "\n"))
}

func Compress(text string) string {
	runes := []rune(strings.TrimSpace(text))
	limit := MaxExcerptTokens
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit])) + " ..."
}

func Normalize(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func dedupKey(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 {
			lastColon := strings.LastIndex(fields[0], ":")
			if lastColon > 0 && allDigits(fields[0][lastColon+1:]) {
				lines = append(lines, strings.Join(fields[1:], " "))
				continue
			}
		}
		lines = append(lines, line)
	}
	return Normalize(strings.Join(lines, "\n"))
}

func allDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func StableID(parts ...string) string {
	joined := Normalize(strings.Join(parts, ":"))
	hash := uint32(2166136261)
	for _, b := range []byte(joined) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return fmt.Sprintf("ctx-%08x", hash)
}

func Terms(text string) map[string]bool {
	result := map[string]bool{}
	for _, field := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(field) >= 3 && !stopwords[field] {
			result[field] = true
		}
	}
	return result
}

var stopwords = map[string]bool{
	"about": true, "after": true, "all": true, "also": true, "and": true, "are": true,
	"can": true, "does": true, "for": true, "from": true, "has": true, "have": true,
	"how": true, "into": true, "its": true, "not": true, "the": true, "their": true,
	"then": true, "this": true, "that": true, "what": true, "when": true, "where": true,
	"which": true, "who": true, "why": true, "with": true,
}

func variants(term string) []string {
	result := []string{term}
	switch {
	case strings.HasSuffix(term, "s") && len(term) > 3:
		result = append(result, strings.TrimSuffix(term, "s"))
	case strings.HasSuffix(term, "ed") && len(term) > 4:
		result = append(result, strings.TrimSuffix(term, "ed"))
	case strings.HasSuffix(term, "ing") && len(term) > 5:
		result = append(result, strings.TrimSuffix(term, "ing"))
	}
	if strings.HasSuffix(term, "authenticated") {
		result = append(result, "authentication", "auth")
	}
	if strings.HasSuffix(term, "persisted") {
		result = append(result, "persistence", "persist")
	}
	if strings.HasSuffix(term, "monitored") {
		result = append(result, "monitoring", "monitor")
	}
	return result
}

func appendUnique(values []string, more ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range more {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
