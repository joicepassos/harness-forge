package application

import (
	"context"
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

// TokenCounter makes the unit used by context budgets explicit. Providers may
// supply a model-aware implementation; local operation uses the conservative
// byte estimator below and never presents it as an exact model token count.
type TokenCounter = domain.TokenCounter

type Budget struct {
	MaxInputTokens int
	ReserveTokens  int
	Model          string
	Counter        TokenCounter
	UseMMR         bool
}

type ConservativeByteEstimator struct{}

func (ConservativeByteEstimator) Name() string { return estimatorName }

func (ConservativeByteEstimator) Count(_ context.Context, _ string, payload []byte) (int, error) {
	return len(payload), nil
}

func Select(candidates []domain.Excerpt, prompt string, budget int, metrics domain.Metrics) *domain.Plan {
	if budget <= 0 {
		budget = domain.DefaultBudgetTokens
	}
	return selectWithBudget(context.Background(), candidates, prompt, Budget{MaxInputTokens: budget}, metrics)
}

// SelectWithBudget uses an explicit token counter and reserves space for the
// provider envelope. Select remains the compatibility entry point and uses the
// conservative local estimator.
func SelectWithBudget(ctx context.Context, candidates []domain.Excerpt, prompt string, budget Budget, metrics domain.Metrics) *domain.Plan {
	if budget.MaxInputTokens <= 0 {
		budget.MaxInputTokens = domain.DefaultBudgetTokens
	}
	return selectWithBudget(ctx, candidates, prompt, budget, metrics)
}

func selectWithBudget(ctx context.Context, candidates []domain.Excerpt, prompt string, budget Budget, metrics domain.Metrics) *domain.Plan {
	// Selection annotates and reorders candidates in place. Keep an untouched
	// copy so a counter failure can restart the complete plan in one unit.
	candidates = cloneExcerpts(candidates)
	originalCandidates := cloneExcerpts(candidates)
	counter := budget.Counter
	if counter == nil {
		counter = ConservativeByteEstimator{}
	}
	reserve := budget.ReserveTokens
	if reserve <= 0 {
		reserve = framingTokens
	}
	var fallbackReasons []string
	count := func(payload string) int {
		value, err := counter.Count(ctx, budget.Model, []byte(payload))
		if err != nil {
			fallbackReasons = appendUnique(fallbackReasons, "counter error")
			return EstimateTokens(payload)
		}
		if value < 0 {
			fallbackReasons = appendUnique(fallbackReasons, "counter returned negative value")
			return EstimateTokens(payload)
		}
		return value
	}
	// When a prompt clearly identifies one of several knowledge scopes, keep
	// only knowledge from the matching scope. If the prompt names no scope,
	// scoped knowledge remains eligible and content keywords determine rank.
	scopeScores := make([]int, len(candidates))
	maxScopeScore := 0
	for i := range candidates {
		if candidates[i].KnowledgeID == "" || len(candidates[i].KnowledgeScope) == 0 {
			continue
		}
		scopeScores[i] = ScopeRelevance(prompt, candidates[i].KnowledgeScope)
		if scopeScores[i] > maxScopeScore {
			maxScopeScore = scopeScores[i]
		}
	}
	if maxScopeScore > 0 {
		for i := range candidates {
			if candidates[i].KnowledgeID != "" && len(candidates[i].KnowledgeScope) > 0 && scopeScores[i] == 0 {
				candidates[i].Status = "excluded"
				candidates[i].Reason = "knowledge scope does not match the task"
			}
		}
	}
	for i := range candidates {
		candidates[i].Relevance += Relevance(prompt, candidates[i].Path, candidates[i].Text)
		candidates[i].Relevance += scopeScores[i] * 25
		candidates[i].EstimatedTokens = count(candidates[i].Text)
		// Approved knowledge is immutable evidence: preserve its complete text
		// and let the budget decision report overflow instead of clipping it.
		if candidates[i].EstimatedTokens > MaxExcerptTokens && candidates[i].KnowledgeID == "" {
			original := candidates[i]
			candidates[i].Text = Compress(candidates[i].Text)
			candidates[i].EstimatedTokens = count(candidates[i].Text)
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
	if budget.UseMMR {
		candidates = reorderMMR(candidates)
	}

	seen := map[string]int{}
	seenAny := map[string]string{}
	relevantTotal := 0
	relevantIncluded := 0
	var included, excluded []domain.Excerpt
	var overflowIDs []string
	minimumOverflow := 0
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
		nextUsed := serializedEstimateWithCounterReserve(prompt, append(append([]domain.Excerpt{}, included...), candidate), count, reserve)
		if nextUsed > budget.MaxInputTokens {
			candidate.Status = "excluded"
			candidate.Reason = "would exceed context budget"
			overflowIDs = append(overflowIDs, candidate.ID)
			delta := nextUsed - budget.MaxInputTokens
			if minimumOverflow == 0 || delta < minimumOverflow {
				minimumOverflow = delta
			}
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
	used := serializedEstimateWithCounterReserve(prompt, included, count, reserve)

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
	estimator := counter.Name()
	if len(fallbackReasons) > 0 {
		// A provider counter can fail after earlier calls succeeded. Returning
		// this attempt would mix provider tokens and bytes in candidate sizes and
		// serialized budget checks. Restart from the original candidates and
		// recompute every value with the byte estimator instead.
		fallbackBudget := budget
		fallbackBudget.Counter = ConservativeByteEstimator{}
		fallbackPlan := selectWithBudget(ctx, originalCandidates, prompt, fallbackBudget, metrics)
		fallbackPlan.Estimator = counter.Name() + "; fallback=" + estimatorName + " (" + strings.Join(fallbackReasons, ", ") + ")"
		return fallbackPlan
	}
	return &domain.Plan{
		BudgetTokens:         budget.MaxInputTokens,
		EstimatedTokens:      used,
		Estimator:            estimator,
		PayloadReserveTokens: reserve,
		Included:             included,
		Excluded:             excluded,
		BudgetOverflow:       len(overflowIDs) > 0,
		OverflowExcerptIDs:   overflowIDs,
		OverflowTokens:       minimumOverflow,
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

func cloneExcerpts(values []domain.Excerpt) []domain.Excerpt {
	cloned := make([]domain.Excerpt, len(values))
	copy(cloned, values)
	for i := range cloned {
		cloned[i].Origins = append([]string(nil), values[i].Origins...)
		cloned[i].KnowledgeScope = append([]string(nil), values[i].KnowledgeScope...)
	}
	return cloned
}

// ScopeRelevance scores prompt terms that also occur in knowledge scope paths.
// It deliberately returns zero when no scope is named, so callers can retain
// generally applicable knowledge without guessing the task's area.
func ScopeRelevance(prompt string, scopes []string) int {
	promptTerms := Terms(prompt)
	if len(promptTerms) == 0 || len(scopes) == 0 {
		return 0
	}
	matched := map[string]bool{}
	for _, scope := range scopes {
		for term := range Terms(scope) {
			if promptTerms[term] {
				matched[term] = true
			}
		}
	}
	return len(matched)
}

// reorderMMR applies a deterministic maximal-marginal-relevance-like order.
// It rewards prompt relevance while penalizing lexical overlap with already
// selected candidates. The default selector does not call it.
func reorderMMR(candidates []domain.Excerpt) []domain.Excerpt {
	remaining := append([]domain.Excerpt(nil), candidates...)
	ordered := make([]domain.Excerpt, 0, len(candidates))
	selected := []map[string]bool{}
	for len(remaining) > 0 {
		best := 0
		bestScore := -1.0
		for i, candidate := range remaining {
			novelty := 1.0
			terms := Terms(candidate.Text)
			for _, prior := range selected {
				if overlap := setOverlap(terms, prior); overlap > 0 {
					novelty = minFloat(novelty, 1-overlap)
				}
			}
			score := 0.75*float64(candidate.Relevance) + 0.25*novelty
			if score > bestScore || (score == bestScore && candidate.ID < remaining[best].ID) {
				best, bestScore = i, score
			}
		}
		candidate := remaining[best]
		ordered = append(ordered, candidate)
		selected = append(selected, Terms(candidate.Text))
		remaining = append(remaining[:best], remaining[best+1:]...)
	}
	return ordered
}

func setOverlap(left, right map[string]bool) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	common := 0
	for term := range left {
		if right[term] {
			common++
		}
	}
	union := len(left) + len(right) - common
	return float64(common) / float64(union)
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func Sources(prompt string, plan *domain.Plan) map[string]string {
	sources := map[string]string{"prompt": prompt}
	for _, excerpt := range plan.Included {
		key := excerpt.Source
		if _, exists := sources[key]; exists {
			key += "#" + excerpt.ID
			for suffix := 2; ; suffix++ {
				if _, exists := sources[key]; !exists {
					break
				}
				key = fmt.Sprintf("%s#%s-%d", excerpt.Source, excerpt.ID, suffix)
			}
		}
		sources[key] = excerpt.Text
	}
	return sources
}

// SourceList returns a lossless, deterministic source envelope. Unlike
// Sources, it does not overwrite excerpts that share a source path.
func SourceList(prompt string, plan *domain.Plan) []domain.Source {
	sources := []domain.Source{{ID: StableID("prompt", prompt), Content: prompt}}
	for _, excerpt := range plan.Included {
		sources = append(sources, domain.Source{
			ID:          excerpt.ID,
			Path:        excerpt.Path,
			Workspace:   excerpt.Workspace,
			StartLine:   excerptStartLine(excerpt.Text),
			EndLine:     excerptEndLine(excerpt.Text),
			Content:     excerpt.Text,
			KnowledgeID: excerpt.KnowledgeID,
			Origins:     append([]string(nil), excerpt.Origins...),
		})
	}
	return sources
}

// LosslessSources adapts the new source envelope to the current provider
// contract while retaining a unique key for every excerpt.
func LosslessSources(prompt string, plan *domain.Plan) map[string]string {
	sources := map[string]string{"prompt": prompt}
	for _, excerpt := range plan.Included {
		key := excerpt.Source + "#" + excerpt.ID
		sources[key] = excerpt.Text
	}
	return sources
}

func excerptStartLine(text string) int {
	for _, line := range strings.Split(text, "\n") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(line), "line %d:", &n); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func excerptEndLine(text string) int {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(lines[i]), "line %d:", &n); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func RequiredTokens(prompt string) int {
	return serializedEstimate(prompt, nil)
}

// RequiredTokensWithCounter measures the serialized prompt envelope using the
// same counter and framing reserve as selection. It returns the counter that
// produced the result; a failing counter falls back to the local byte estimate.
func RequiredTokensWithCounter(ctx context.Context, prompt, model string, counter TokenCounter) (int, TokenCounter) {
	if counter == nil {
		counter = ConservativeByteEstimator{}
	}
	payload, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		counter = ConservativeByteEstimator{}
		return RequiredTokens(prompt), counter
	}
	count, err := counter.Count(ctx, model, payload)
	if err != nil || count < 0 {
		counter = ConservativeByteEstimator{}
		count, _ = counter.Count(ctx, model, payload)
	}
	return count + framingTokens, counter
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
	return serializedEstimateWithCounter(prompt, included, EstimateTokens)
}

func serializedEstimateWithCounter(prompt string, included []domain.Excerpt, count func(string) int) int {
	return serializedEstimateWithCounterReserve(prompt, included, count, framingTokens)
}

func serializedEstimateWithCounterReserve(prompt string, included []domain.Excerpt, count func(string) int, reserve int) int {
	sources := Sources(prompt, &domain.Plan{Included: included})
	data, err := json.Marshal(sources)
	if err != nil {
		return reserve + count(prompt)
	}
	return reserve + count(string(data))
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
	count, _ := (ConservativeByteEstimator{}).Count(context.Background(), "", []byte(trimmed))
	return count
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
	// Portuguese and Spanish stopwords keep localized CLI queries from
	// overweighting grammatical words instead of repository terminology.
	"com": true, "como": true, "das": true, "de": true, "do": true, "dos": true,
	"e": true, "em": true, "entre": true, "esta": true, "este": true, "para": true,
	"por": true, "que": true, "uma": true, "um": true, "não": true, "nao": true,
	"al": true, "del": true, "el": true, "en": true, "es": true, "la": true,
	"las": true, "los": true, "una": true, "uno": true, "con": true, "cómo": true,
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
