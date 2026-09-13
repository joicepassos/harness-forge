package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"harnessforge/internal/doctor/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	"harnessforge/internal/harness/infrastructure"
	"io"
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
	file, err := os.Open(harnessPath)
	if err != nil {
		return []domain.Diagnostic{{Code: "harness.unreadable", Severity: domain.SeverityError, Message: "Harness IR cannot be read", Evidence: []string{harnessPath}, Suggestion: "Provide a readable Harness IR file."}}, nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read Harness IR: %w", err)
	}
	if len(data) > 4<<20 {
		return []domain.Diagnostic{{Code: "context.oversized", Severity: domain.SeverityError, Message: "Harness IR exceeds the 4 MiB diagnostic input limit", Suggestion: "Reduce the file before requesting schema and reference diagnostics."}}, nil
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
		diagnostics, err := inspectRepository(ctx, repository, raw)
		if err != nil {
			return nil, err
		}
		result = append(result, diagnostics...)
		h, err := (infrastructure.YAMLLoader{}).Load(harnessPath)
		if err == nil {
			owners := append([]harnessdomain.Rule{}, h.Rules...)
			for _, skill := range h.Skills {
				owners = append(owners, harnessdomain.Rule{ID: "skill:" + skill.ID, Evidence: skill.Evidence})
			}
			for _, owner := range owners {
				for _, evidence := range owner.Evidence {
					check := harnessdomain.Harness{Rules: []harnessdomain.Rule{{Evidence: []harnessdomain.Evidence{evidence}}}}
					if err := infrastructure.CheckEvidence(ctx, repository, check); err != nil {
						if ctx.Err() != nil {
							return nil, ctx.Err()
						}
						result = append(result, domain.Diagnostic{Code: "evidence.invalid", Severity: domain.SeverityError, Message: "Evidence cannot be verified against the repository", Evidence: []string{owner.ID, evidence.File, err.Error()}, Suggestion: "Correct missing files, unsafe paths, revisions, or symbols; do not approve unsupported rules."})
					}
				}
			}
		}
	}
	return result, ctx.Err()
}

func inspectRepository(ctx context.Context, root string, h rawHarness) ([]domain.Diagnostic, error) {
	var diagnostics []domain.Diagnostic
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return []domain.Diagnostic{repositoryFailure()}, nil
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return []domain.Diagnostic{repositoryFailure()}, nil
	}
	hasTests := false
	visited := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		visited++
		if visited > 20000 {
			return fmt.Errorf("repository scan exceeds 20000 entries")
		}
		if entry.IsDir() && path != root {
			switch entry.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
		}
		if !entry.IsDir() && entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), "_test.go") {
			hasTests = true
		}
		return nil
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		diagnostics = append(diagnostics, domain.Diagnostic{Code: "repository.incomplete", Severity: domain.SeverityError, Message: "Repository scan could not complete", Suggestion: "Check directory permissions and the 20000-entry scan limit before relying on this diagnosis."})
	}
	if !hasTests && err == nil {
		diagnostics = append(diagnostics, domain.Diagnostic{Code: "tests.missing", Severity: domain.SeverityWarning, Message: "No Go test files were found in the repository", Suggestion: "Add behavioral tests or document why this repository does not use Go tests."})
	}
	for _, skill := range h.Skills {
		if skill.Path == "" || unsafe(skill.Path) {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(skill.Path))
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			code := "skill.unreadable"
			if errors.Is(err, os.ErrNotExist) {
				code = "skill.missing"
			}
			diagnostics = append(diagnostics, domain.Diagnostic{Code: code, Severity: domain.SeverityError, Message: "Referenced skill cannot be read", Evidence: []string{skill.ID, skill.Path}, Suggestion: "Restore the file or correct its access permissions."})
			continue
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			diagnostics = append(diagnostics, unsafePath("skill path", skill.ID, skill.Path))
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			diagnostics = append(diagnostics, domain.Diagnostic{Code: "skill.missing", Severity: domain.SeverityError, Message: "Referenced skill file is missing", Evidence: []string{skill.ID + ": " + skill.Path}, Suggestion: "Correct the skill path or restore the referenced file."})
		}
	}
	return diagnostics, nil
}

func repositoryFailure() domain.Diagnostic {
	return domain.Diagnostic{Code: "repository.invalid", Severity: domain.SeverityError, Message: "Repository is not an accessible directory", Suggestion: "Provide an existing readable repository directory."}
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
