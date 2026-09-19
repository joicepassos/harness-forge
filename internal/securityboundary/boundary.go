// Package securityboundary contains conservative local trust-boundary checks.
package securityboundary

import (
	"bufio"
	"context"
	"fmt"
	"harnessforge/internal/inputlimits"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sensitiveContent = []*regexp.Regexp{
	regexp.MustCompile(`(?im)\b(?:api[_-]?key|access[_-]?token|secret(?:[_-]?key)?|password|passwd)\b["']?\s*[:=]\s*(?:"[^"\r\n]{8,}"|'[^'\r\n]{8,}'|[^\s"']{8,})`),
	regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16})\b`),
}

// SensitivePath identifies common credential-bearing paths before their
// contents can be indexed or included in provider context.
func SensitivePath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
		return true
	}
	return strings.Contains(name, "secret") || strings.Contains(name, "credential") || strings.Contains(name, "token") || strings.Contains(name, "password") || strings.Contains(name, "passwd")
}

// ContainsSensitiveContent reports recognizable credentials. It is a guardrail,
// not a claim to provide complete secret detection.
func ContainsSensitiveContent(data []byte) bool {
	for _, pattern := range sensitiveContent {
		if pattern.Match(data) {
			return true
		}
	}
	return false
}

// PrepareDirectory creates a repository-relative directory only when each
// existing path component is a real directory, never a link or reparse point.
func PrepareDirectory(repository, relative string) (string, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("repository must be a non-symlink directory")
	}
	parts := strings.FieldsFunc(filepath.Clean(relative), func(r rune) bool { return r == filepath.Separator || r == '/' || r == '\\' })
	if len(parts) == 0 || filepath.IsAbs(relative) {
		return "", fmt.Errorf("unsafe repository-relative directory")
	}
	current := root
	for _, part := range parts {
		if part == "." || part == ".." {
			return "", fmt.Errorf("unsafe repository-relative directory")
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
				return "", err
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("repository directory %s cannot be a symlink or non-directory", relative)
		}
	}
	return current, nil
}

// Ignored reports paths matched by simple .gitignore rules.
// It deliberately supports the common exact, directory, wildcard and negation
// forms used by local project ignores without treating ignore files as code.
type Ignored struct{ rules []ignoreRule }
type ignoreRule struct {
	base, pattern     string
	negate, directory bool
}

// LoadGitIgnore loads repository ignore rules with the standard repository scan limit.
func LoadGitIgnore(root string) (Ignored, error) {
	return LoadGitIgnoreContext(context.Background(), root, inputlimits.RepositoryFiles)
}

// LoadGitIgnoreContext loads ignore rules without traversing excluded directories
// and stops when the caller cancels or the entry limit is exceeded.
func LoadGitIgnoreContext(ctx context.Context, root string, limit int) (Ignored, error) {
	if err := ctx.Err(); err != nil {
		return Ignored{}, err
	}
	if limit < 1 {
		return Ignored{}, fmt.Errorf("repository entry limit must be positive")
	}
	var out Ignored
	entries := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if path != root {
			entries++
			if entries > limit {
				return fmt.Errorf("repository scan exceeds %d entries", limit)
			}
		}
		if entry.IsDir() {
			if path != root && SkipRepositoryDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != ".gitignore" || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		base, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		if base == "." {
			base = ""
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			rule := ignoreRule{base: filepath.ToSlash(base), negate: strings.HasPrefix(line, "!")}
			if rule.negate {
				line = strings.TrimPrefix(line, "!")
			}
			line = strings.TrimPrefix(filepath.ToSlash(line), "/")
			rule.directory = strings.HasSuffix(line, "/")
			rule.pattern = strings.TrimSuffix(line, "/")
			if rule.pattern != "" {
				out.rules = append(out.rules, rule)
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return Ignored{}, err
	}
	return out, nil
}

// SkipRepositoryDirectory reports directories that local repository readers do
// not traverse because they contain metadata, dependencies, or build output.
func SkipRepositoryDirectory(name string) bool {
	switch name {
	case ".git", ".harness", ".next", "build", "dist", "node_modules", "target", "vendor":
		return true
	}
	return false
}

func (i Ignored) Match(rel string) bool {
	rel = filepath.ToSlash(rel)
	matched := false
	for _, rule := range i.rules {
		candidate := rel
		if rule.base != "" {
			if rel == rule.base {
				candidate = ""
			} else if strings.HasPrefix(rel, rule.base+"/") {
				candidate = strings.TrimPrefix(rel, rule.base+"/")
			} else {
				continue
			}
		}
		pattern := rule.pattern
		ok, _ := filepath.Match(pattern, candidate)
		if !ok && !strings.Contains(pattern, "/") {
			ok, _ = filepath.Match(pattern, filepath.Base(candidate))
		}
		if !ok && rule.directory {
			if strings.Contains(pattern, "/") {
				ok = candidate == pattern || strings.HasPrefix(candidate, pattern+"/")
			} else {
				for _, segment := range strings.Split(candidate, "/") {
					if segment == pattern {
						ok = true
						break
					}
				}
			}
		}
		if ok {
			matched = !rule.negate
		}
	}
	return matched
}
