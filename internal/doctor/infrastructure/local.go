package infrastructure

import (
	"context"
	"fmt"
	"harnessforge/internal/doctor/domain"
	"harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const ContextWarningBytes int64 = 64 << 10

type LocalSource struct{}

func (LocalSource) Diagnose(ctx context.Context, harnessPath, repository string) ([]domain.Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(harnessPath)
	if err != nil {
		return []domain.Diagnostic{{Code: "harness.unreadable", Severity: domain.SeverityError, Message: "Harness IR cannot be read", Evidence: []string{harnessPath}, Suggestion: "Provide a readable Harness IR file."}}, nil
	}
	var raw rawHarness
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return []domain.Diagnostic{{Code: "harness.invalid", Severity: domain.SeverityError, Message: "Harness IR is not valid YAML", Evidence: []string{err.Error()}, Suggestion: "Correct the YAML syntax before relying on the harness."}}, nil
	}
	result := duplicates(raw)
	if len(raw.Rules) == 0 {
		result = append(result, domain.Diagnostic{Code: "rules.missing", Severity: domain.SeverityWarning, Message: "Harness IR declares no instructions", Suggestion: "Review whether the project needs explicit rules before relying on an empty harness."})
	}
	if _, err := (infrastructure.YAMLLoader{}).Load(harnessPath); err != nil {
		result = append(result, domain.Diagnostic{Code: "harness.invalid", Severity: domain.SeverityError, Message: "Harness IR fails schema or domain validation", Evidence: []string{err.Error()}, Suggestion: "Correct the reported field while preserving intentional manual decisions."})
	}
	if int64(len(data)) > ContextWarningBytes {
		result = append(result, domain.Diagnostic{Code: "context.oversized", Severity: domain.SeverityWarning, Message: "Harness IR exceeds the context warning threshold", Evidence: []string{fmt.Sprintf("%d bytes; threshold %d bytes", len(data), ContextWarningBytes)}, Suggestion: "Split supporting detail into focused skills or remove obsolete material after review."})
	}
	if repository != "" {
		result = append(result, inspectRepository(repository, raw)...)
		h, err := (infrastructure.YAMLLoader{}).Load(harnessPath)
		if err == nil {
			if err := infrastructure.CheckEvidence(ctx, repository, h); err != nil {
				result = append(result, domain.Diagnostic{Code: "evidence.invalid", Severity: domain.SeverityError, Message: "Evidence cannot be verified against the repository", Evidence: []string{err.Error()}, Suggestion: "Correct missing files, unsafe paths, revisions, or symbols; do not approve unsupported rules."})
			}
		}
	}
	return result, nil
}

func inspectRepository(root string, h rawHarness) []domain.Diagnostic {
	var diagnostics []domain.Diagnostic
	hasTests := false
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(entry.Name(), "_test.go") {
			hasTests = true
		}
		return nil
	})
	if !hasTests {
		diagnostics = append(diagnostics, domain.Diagnostic{Code: "tests.missing", Severity: domain.SeverityWarning, Message: "No Go test files were found in the repository", Suggestion: "Add behavioral tests or document why this repository does not use Go tests."})
	}
	for _, skill := range h.Skills {
		if skill.Path == "" || unsafe(skill.Path) {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(skill.Path))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			diagnostics = append(diagnostics, domain.Diagnostic{Code: "skill.missing", Severity: domain.SeverityError, Message: "Referenced skill file is missing", Evidence: []string{skill.ID + ": " + skill.Path}, Suggestion: "Correct the skill path or restore the referenced file."})
		}
	}
	return diagnostics
}

type rawHarness struct {
	Rules  []rawRule  `yaml:"rules"`
	Skills []rawSkill `yaml:"skills"`
}
type rawRule struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Scope       struct {
		Paths []string `yaml:"paths"`
	} `yaml:"scope"`
}
type rawSkill struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Path        string `yaml:"path"`
}

func duplicates(h rawHarness) []domain.Diagnostic {
	var out []domain.Diagnostic
	seen := map[string]string{}
	for _, r := range h.Rules {
		key := normalize(r.Description)
		if key != "" && seen[key] != "" {
			out = append(out, duplicate("instruction.duplicate", r.ID, seen[key]))
		} else if key != "" {
			seen[key] = r.ID
		}
		for _, path := range r.Scope.Paths {
			if unsafe(path) {
				out = append(out, unsafePath("rule scope", r.ID, path))
			}
		}
	}
	for _, s := range h.Skills {
		key := normalize(s.Description)
		if key != "" && seen[key] != "" {
			out = append(out, duplicate("instruction.duplicate", s.ID, seen[key]))
		} else if key != "" {
			seen[key] = s.ID
		}
		if s.Path != "" && unsafe(s.Path) {
			out = append(out, unsafePath("skill path", s.ID, s.Path))
		}
	}
	return out
}
func duplicate(code, current, previous string) domain.Diagnostic {
	return domain.Diagnostic{Code: code, Severity: domain.SeverityWarning, Message: "Potentially duplicate instruction", Evidence: []string{previous, current}, Suggestion: "Review both instructions and consolidate only if their intent is truly the same."}
}
func unsafePath(kind, id, path string) domain.Diagnostic {
	return domain.Diagnostic{Code: "path.unsafe", Severity: domain.SeverityError, Message: "Unsafe " + kind, Evidence: []string{id + ": " + path}, Suggestion: "Use a repository-relative slash-separated path without traversal."}
}
func normalize(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }
func unsafe(p string) bool {
	return filepath.IsAbs(p) || strings.Contains(p, "\\") || strings.Contains(p, ":") || strings.HasPrefix(filepath.Clean(p), "..")
}
