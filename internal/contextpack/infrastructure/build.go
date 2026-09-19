package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/contextpack/application"
	"harnessforge/internal/contextpack/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/internal/securityboundary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxFiles = 200
	defaultMaxBytes = 8192
)

func Build(ctx context.Context, repositoryPath, prompt, model string, options domain.Options) (*domain.Plan, error) {
	if strings.TrimSpace(repositoryPath) == "" {
		return nil, fmt.Errorf("repository path must not be empty")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(repositoryPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository path must be a directory")
	}

	options = defaults(options, model)
	if required := application.RequiredTokens(prompt); required > options.BudgetTokens {
		return nil, fmt.Errorf("context budget %d is smaller than required prompt envelope %d", options.BudgetTokens, required)
	}
	analysis, err := analyzer.AnalyzeWithOptions(ctx, root, analyzer.Options{})
	if err != nil {
		return nil, err
	}
	files, fileExclusions, err := collectFiles(ctx, root, options.MaxFiles)
	if err != nil {
		return nil, err
	}

	candidates, previousAnalyzerTokens := candidatesFromAnalysis(analysis)
	fileCandidates, fileBaseline, err := candidatesFromFiles(ctx, root, files, prompt, options.MaxBytesPerFile)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, fileCandidates...)
	candidates = append(candidates, fileExclusions...)
	metrics := domain.Metrics{
		PreviousAnalyzerJSONEstimatedTokens: previousAnalyzerTokens + application.RequiredTokens(prompt),
		UnfilteredCandidateEstimatedTokens:  previousAnalyzerTokens + fileBaseline + application.RequiredTokens(prompt),
		PreviousRelevantRecallPercent:       previousAnalyzerRecall(candidates),
	}

	return application.Select(candidates, prompt, options.BudgetTokens, metrics), nil
}

func defaults(options domain.Options, model string) domain.Options {
	if options.BudgetTokens <= 0 {
		options.BudgetTokens = domain.DefaultBudgetTokens
		switch strings.ToLower(model) {
		case "gpt-4o-mini":
			options.BudgetTokens = 2400
		case "deepseek-v4-flash":
			options.BudgetTokens = 2000
		}
	}
	if options.MaxFiles <= 0 || options.MaxFiles > defaultMaxFiles {
		options.MaxFiles = defaultMaxFiles
	}
	if options.MaxBytesPerFile <= 0 || options.MaxBytesPerFile > defaultMaxBytes {
		options.MaxBytesPerFile = defaultMaxBytes
	}
	return options
}

func candidatesFromAnalysis(analysis *analyzer.Analysis) ([]domain.Excerpt, int) {
	data, _ := json.Marshal(analysis)
	baseline := application.EstimateTokens(string(data))
	var candidates []domain.Excerpt
	addFindings := func(group string, findings []analyzer.Finding) {
		for _, finding := range findings {
			text := group + ": " + finding.Value
			if len(finding.Evidence) > 0 {
				text += " evidence: " + strings.Join(finding.Evidence, ", ")
			}
			candidates = append(candidates, domain.Excerpt{
				ID:              application.StableID("analysis", group, finding.Value),
				Source:          "repository-analysis:" + group + ":" + finding.Value,
				Text:            text,
				EstimatedTokens: application.EstimateTokens(text),
				Origins:         []string{"repository-analysis"},
			})
		}
	}
	addFindings("languages", analysis.Languages)
	addFindings("build", analysis.Build)
	addFindings("frameworks", analysis.Frameworks)
	addFindings("infrastructure", analysis.Infrastructure)
	addFindings("database", analysis.Database)
	addFindings("tests", analysis.Tests)
	return candidates, baseline
}

func candidatesFromFiles(ctx context.Context, root string, files []string, prompt string, maxBytes int) ([]domain.Excerpt, int, error) {
	var candidates []domain.Excerpt
	baseline := 0
	for _, rel := range files {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil {
			candidates = append(candidates, excluded(rel, "unreadable file"))
			continue
		}
		if unsafePath(rel) {
			candidates = append(candidates, excluded(rel, "unsafe relative path"))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			candidates = append(candidates, excluded(rel, "symlink skipped"))
			continue
		}
		if !info.Mode().IsRegular() {
			candidates = append(candidates, excluded(rel, "non-regular file skipped"))
			continue
		}
		if looksSecret(rel) || securityboundary.SensitivePath(rel) {
			candidates = append(candidates, excluded(rel, "secret-like path skipped"))
			continue
		}
		if agentInstruction(rel) {
			candidates = append(candidates, excluded(rel, "agent instruction file skipped"))
			continue
		}
		content, truncated, err := readLimited(full, maxBytes)
		if err != nil {
			candidates = append(candidates, excluded(rel, "unreadable file"))
			continue
		}
		if !utf8.Valid(content) || binary(content) {
			candidates = append(candidates, excluded(rel, "binary or non-UTF-8 file skipped"))
			continue
		}
		if securityboundary.ContainsSensitiveContent(content) {
			candidates = append(candidates, excluded(rel, "sensitive content skipped"))
			continue
		}
		text := strings.TrimSpace(string(content))
		if text == "" {
			candidates = append(candidates, excluded(rel, "empty file skipped"))
			continue
		}
		baseline += application.EstimateTokens(text)
		snippet := matchingExcerpt(prompt, rel, text)
		reason := ""
		if truncated {
			reason = "source file read was byte-limited"
		}
		candidates = append(candidates, domain.Excerpt{
			ID:              application.StableID("file", rel, snippet),
			Source:          "repository-file:" + rel,
			Path:            rel,
			Text:            snippet,
			Relevance:       application.Relevance(prompt, rel, snippet),
			EstimatedTokens: application.EstimateTokens(snippet),
			Reason:          reason,
			Origins:         []string{rel},
		})
	}
	return candidates, baseline, nil
}

func collectFiles(ctx context.Context, root string, limit int) ([]string, []domain.Excerpt, error) {
	ignored, err := securityboundary.LoadGitIgnoreContext(ctx, root, inputlimits.RepositoryFiles)
	if err != nil {
		return nil, nil, err
	}
	var files []string
	var exclusions []domain.Excerpt
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if securityboundary.SkipRepositoryDirectory(name) || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if ignored.Match(rel) {
			return nil
		}
		if len(files) >= limit {
			exclusions = append(exclusions, excluded(rel, "file limit reached"))
			return filepath.SkipAll
		}
		files = append(files, rel)
		return nil
	})
	return files, exclusions, err
}

func excluded(path string, reason string) domain.Excerpt {
	return domain.Excerpt{
		ID:              application.StableID("excluded", path, reason),
		Source:          "repository-file:" + path,
		Path:            path,
		Status:          "excluded",
		Reason:          reason,
		EstimatedTokens: 0,
		Origins:         []string{path},
	}
}

func readLimited(path string, maxBytes int) ([]byte, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}

func matchingExcerpt(prompt, path, text string) string {
	terms := application.Terms(prompt)
	lines := strings.Split(text, "\n")
	var kept []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(path + " " + trimmed)
		for term := range terms {
			if strings.Contains(lower, term) {
				kept = append(kept, fmt.Sprintf("line %d: %s", i+1, trimmed))
				break
			}
		}
		if application.EstimateTokens(strings.Join(kept, "\n")) > application.MaxExcerptTokens*2 {
			break
		}
	}
	if len(kept) > 0 {
		return strings.Join(kept, "\n")
	}
	return firstExcerpt(path, lines)
}

func firstExcerpt(path string, lines []string) string {
	var kept []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		kept = append(kept, fmt.Sprintf("line %d: %s", i+1, trimmed))
		if application.EstimateTokens(strings.Join(kept, "\n")) > application.MaxExcerptTokens*2 {
			break
		}
	}
	return strings.Join(kept, "\n")
}

func binary(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func looksSecret(path string) bool {
	if securityboundary.SensitivePath(path) {
		return true
	}
	name := strings.ToLower(filepath.Base(path))
	extension := strings.ToLower(filepath.Ext(name))
	configLike := extension == ".env" || extension == ".properties" || extension == ".yml" || extension == ".yaml" || extension == ".json" || extension == ".toml" || extension == ".ini" || extension == ".conf" || extension == ".config" || extension == ".xml"
	if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
		return true
	}
	if strings.Contains(name, "secret") || strings.Contains(name, "credential") || strings.Contains(name, "token") || strings.Contains(name, "password") || strings.Contains(name, "passwd") {
		return true
	}
	authConfigNames := map[string]bool{"auth": true, "authn": true, "authz": true, "authentication": true, "authorization": true}
	base := strings.TrimSuffix(name, extension)
	return configLike && authConfigNames[base]
}

func agentInstruction(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return name == "agents.md" || name == "claude.md" || name == "codex.md" || name == "gemini.md" || name == ".cursorrules"
}

func unsafePath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	return filepath.IsAbs(path) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".."
}

func previousAnalyzerRecall(candidates []domain.Excerpt) int {
	relevant := 0
	covered := 0
	for _, candidate := range candidates {
		if candidate.Status == "excluded" || candidate.Relevance <= 0 {
			continue
		}
		relevant++
		if strings.HasPrefix(candidate.Source, "repository-analysis:") {
			covered++
		}
	}
	if relevant == 0 {
		return 100
	}
	return covered * 100 / relevant
}
