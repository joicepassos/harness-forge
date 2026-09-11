package analyzer

import (
	"path/filepath"
	"strings"
)

type Rule interface {
	Apply(repository Repository) (Finding, bool)
}

type extensionRule struct {
	value     string
	extension string
}

func (rule extensionRule) Apply(repository Repository) (Finding, bool) {
	matches := repository.FilesWithExtension(rule.extension)
	if len(matches) == 0 {
		return Finding{}, false
	}

	return finding(rule.value, matches...), true
}

type fileRule struct {
	value string
	paths []string
}

func (rule fileRule) Apply(repository Repository) (Finding, bool) {
	for _, path := range rule.paths {
		if repository.HasFile(path) {
			return finding(rule.value, path), true
		}
	}

	return Finding{}, false
}

type contentRule struct {
	value string
	path  string
	text  string
}

func (rule contentRule) Apply(repository Repository) (Finding, bool) {
	if !repository.FileContains(rule.path, rule.text) {
		return Finding{}, false
	}

	return finding(rule.value, rule.path+" contains "+rule.text), true
}

type prefixRule struct {
	value  string
	prefix string
}

func (rule prefixRule) Apply(repository Repository) (Finding, bool) {
	for _, file := range repository.Files {
		if strings.HasPrefix(file, rule.prefix) {
			return finding(rule.value, rule.prefix), true
		}
	}

	return Finding{}, false
}

type suffixRule struct {
	value  string
	suffix string
}

func (rule suffixRule) Apply(repository Repository) (Finding, bool) {
	for _, file := range repository.Files {
		if strings.HasSuffix(strings.ToLower(file), strings.ToLower(rule.suffix)) {
			return finding(rule.value, file), true
		}
	}

	return Finding{}, false
}

type anyRule struct {
	rules []Rule
}

func (rule anyRule) Apply(repository Repository) (Finding, bool) {
	for _, candidate := range rule.rules {
		result, found := candidate.Apply(repository)
		if found {
			return result, true
		}
	}

	return Finding{}, false
}

func (repository Repository) HasFile(name string) bool {
	for _, file := range repository.Files {
		if strings.EqualFold(filepath.Base(file), name) || strings.EqualFold(file, name) {
			return true
		}
	}

	return false
}

func (repository Repository) FilesWithExtension(extension string) []string {
	var matches []string

	for _, file := range repository.Files {
		if strings.EqualFold(filepath.Ext(file), extension) {
			matches = append(matches, file)
		}
	}

	return matches
}
