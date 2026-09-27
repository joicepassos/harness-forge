package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/contextpack/application"
	"harnessforge/internal/contextpack/domain"
	"harnessforge/internal/repository"
	"harnessforge/internal/securityboundary"
	"os"
	"path/filepath"
	"sort"
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
	counter := options.Counter
	if counter == nil {
		counter = application.ConservativeByteEstimator{}
	}
	required, estimateErr := counter.Count(ctx, options.Model, []byte(prompt))
	if estimateErr != nil || required < 0 {
		required = application.RequiredTokens(prompt)
		counter = application.ConservativeByteEstimator{}
	}
	promptOverflow := func() *domain.Plan {
		return &domain.Plan{BudgetTokens: options.BudgetTokens, EstimatedTokens: required, Estimator: counter.Name(), BudgetOverflow: true, OverflowTokens: required - options.BudgetTokens, Included: []domain.Excerpt{}, Excluded: []domain.Excerpt{{ID: "prompt-envelope", Source: "prompt", Text: prompt, Relevance: 1, EstimatedTokens: required, Status: "excluded", Reason: "prompt exceeds the entire context budget", Origins: []string{"prompt"}}}}
	}
	if required > options.BudgetTokens {
		return promptOverflow(), nil
	}
	snapshot, err := repository.Scan(ctx, root, repository.ScanOptions{HonorIgnores: true, SkipDirs: repository.DefaultSkipDirs()})
	if err != nil {
		return nil, err
	}
	defer snapshot.Close()
	analysis, err := analyzer.AnalyzeSnapshot(ctx, snapshot, analyzer.Options{})
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		parts := strings.Split(filepath.ToSlash(file.Path), "/")
		if len(parts) > 1 && strings.HasPrefix(parts[0], ".") && parts[0] != ".github" {
			continue
		}
		files = append(files, file.Path)
	}
	var fileExclusions []domain.Excerpt
	// Catalog the whole repository before applying the deep-read limit. This
	// prevents lexicographic walk order from deciding which files can provide
	// evidence for the prompt.
	sort.SliceStable(files, func(i, j int) bool {
		a := application.Relevance(prompt, files[i], "")
		b := application.Relevance(prompt, files[j], "")
		if a != b {
			return a > b
		}
		return files[i] < files[j]
	})
	if len(files) > options.MaxFiles {
		for _, rel := range files[options.MaxFiles:] {
			fileExclusions = append(fileExclusions, excluded(rel, "deep-read limit reached after relevance ranking"))
		}
		files = files[:options.MaxFiles]
	}

	candidates, previousAnalyzerTokens := candidatesFromAnalysis(analysis)
	fileCandidates, fileBaseline, err := candidatesFromFiles(ctx, snapshot, files, prompt, options.MaxBytesPerFile)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, fileCandidates...)
	knowledgeCandidates, err := forgeKnowledgeCandidates(root, options.Layout, prompt, options.TaskPaths)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, knowledgeCandidates...)
	if required := application.RequiredTokens(prompt); required > options.BudgetTokens {
		return promptOverflow(), nil
	}
	candidates = append(candidates, fileExclusions...)
	if options.UseBM25 {
		applyBM25(candidates, prompt)
	}
	metrics := domain.Metrics{
		PreviousAnalyzerJSONEstimatedTokens: previousAnalyzerTokens + application.RequiredTokens(prompt),
		UnfilteredCandidateEstimatedTokens:  previousAnalyzerTokens + fileBaseline + application.RequiredTokens(prompt),
		PreviousRelevantRecallPercent:       previousAnalyzerRecall(candidates),
	}

	return application.SelectWithBudget(ctx, candidates, prompt, application.Budget{
		MaxInputTokens: options.BudgetTokens,
		Model:          options.Model,
		Counter:        options.Counter,
		UseMMR:         options.UseMMR,
	}, metrics), nil
}

func applyBM25(candidates []domain.Excerpt, prompt string) {
	documents := make([]application.BM25Document, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Status != "excluded" {
			documents = append(documents, application.BM25Document{ID: candidate.ID, Text: candidate.Path + " " + candidate.Text})
		}
	}
	ranked := application.RankBM25(prompt, documents, len(documents))
	scores := map[string]float64{}
	for _, result := range ranked {
		scores[result.ID] = result.Score
	}
	for i := range candidates {
		if score, ok := scores[candidates[i].ID]; ok {
			candidates[i].Relevance += int(score * 100)
		}
	}
}

func defaults(options domain.Options, model string) domain.Options {
	if options.Model == "" {
		options.Model = model
	}
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
			if len(finding.Workspaces) > 0 {
				text += " workspaces: " + strings.Join(finding.Workspaces, ", ")
			}
			if len(finding.Evidence) > 0 {
				text += " evidence: " + strings.Join(finding.Evidence, ", ")
			}
			for _, evidence := range finding.EvidenceItems {
				text += fmt.Sprintf(" structured evidence: %s (%s, workspace=%s", evidence.Path, evidence.Kind, evidence.Workspace)
				if evidence.StartLine > 0 {
					text += fmt.Sprintf(", lines=%d-%d", evidence.StartLine, evidence.EndLine)
				}
				if evidence.SHA256 != "" {
					text += ", sha256=" + evidence.SHA256
				}
				text += ")"
			}
			candidates = append(candidates, domain.Excerpt{
				ID:              application.StableID("analysis", group, finding.Value),
				Source:          "repository-analysis:" + group + ":" + finding.Value,
				Workspace:       finding.Workspace,
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
	for _, module := range analysis.GoModules {
		text := "go module: " + module.Module + " (" + module.Path + ")"
		candidates = append(candidates, domain.Excerpt{
			ID:              application.StableID("go-module", module.Path, module.Module),
			Source:          "repository-analysis:go-module:" + module.Path,
			Path:            module.Path,
			Workspace:       module.Workspace,
			Text:            text,
			EstimatedTokens: application.EstimateTokens(text),
			Origins:         []string{"repository-analysis"},
		})
	}
	for _, gate := range analysis.QualityGates {
		text := "quality gate: " + gate.Command
		if gate.Workspace != "" {
			text += " [workspace: " + gate.Workspace + "]"
		}
		if gate.Reason != "" {
			text += " (" + gate.Reason + ")"
		}
		candidates = append(candidates, domain.Excerpt{
			ID:              application.StableID("quality-gate", gate.ID, gate.Command),
			Source:          "repository-analysis:quality-gate:" + gate.ID,
			Workspace:       gate.Workspace,
			Text:            text,
			EstimatedTokens: application.EstimateTokens(text),
			Origins:         []string{"repository-analysis"},
		})
	}
	return candidates, baseline
}

func candidatesFromFiles(ctx context.Context, snapshot *repository.RepositorySnapshot, files []string, prompt string, maxBytes int) ([]domain.Excerpt, int, error) {
	var candidates []domain.Excerpt
	baseline := 0
	for _, rel := range files {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if unsafePath(rel) {
			candidates = append(candidates, excluded(rel, "unsafe relative path"))
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
		content, truncated, err := snapshot.Read(rel, int64(maxBytes))
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
			Workspace:       snapshot.WorkspaceForPath(rel),
			Text:            snippet,
			Relevance:       application.Relevance(prompt, rel, snippet),
			EstimatedTokens: application.EstimateTokens(snippet),
			Reason:          reason,
			Origins:         []string{rel},
		})
	}
	return candidates, baseline, nil
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
