// Package securityboundary contains conservative local trust-boundary checks.
package securityboundary

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sensitiveContent = []*regexp.Regexp{
	regexp.MustCompile(`(?im)\b(?:api[_-]?key|access[_-]?token|secret(?:[_-]?key)?|password|passwd)\b\s*[:=]\s*[^\s"']{8,}`),
	regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16})\b`),
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

// Ignored reports paths matched by simple repository-root .gitignore rules.
// It deliberately supports the common exact, directory, wildcard and negation
// forms used by local project ignores without treating ignore files as code.
type Ignored struct{ rules []ignoreRule }
type ignoreRule struct {
	pattern           string
	negate, directory bool
}

func LoadGitIgnore(root string) (Ignored, error) {
	file, err := os.Open(filepath.Join(root, ".gitignore"))
	if os.IsNotExist(err) {
		return Ignored{}, nil
	}
	if err != nil {
		return Ignored{}, err
	}
	defer file.Close()
	var out Ignored
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := ignoreRule{negate: strings.HasPrefix(line, "!")}
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
	return out, scanner.Err()
}

func (i Ignored) Match(rel string) bool {
	rel = filepath.ToSlash(rel)
	matched := false
	for _, rule := range i.rules {
		pattern := rule.pattern
		ok, _ := filepath.Match(pattern, rel)
		if !ok && !strings.Contains(pattern, "/") {
			ok, _ = filepath.Match(pattern, filepath.Base(rel))
		}
		if !ok && rule.directory {
			ok = rel == pattern || strings.HasPrefix(rel, pattern+"/")
		}
		if ok {
			matched = !rule.negate
		}
	}
	return matched
}
