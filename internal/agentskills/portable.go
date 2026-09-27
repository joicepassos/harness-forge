// Package agentskills validates and reads portable Agent Skills bundles.
package agentskills

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	maxFileBytes   = 1 << 20
	maxBundleBytes = 8 << 20
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Bundle contains the validated Agent Skills files keyed by paths relative to
// the skill directory. It includes SKILL.md and only the portable resource
// directories supported by the Agent Skills format.
type Bundle struct {
	Name        string
	Description string
	Files       map[string][]byte
}

// Read loads a referenced SKILL.md file or skill directory beneath root. It
// refuses symlinks, special files, unsupported top-level resources, and
// oversized bundles so sync can safely publish a deterministic snapshot.
func Read(root *os.Root, reference string) (Bundle, error) {
	var result Bundle
	clean, err := cleanRelative(reference)
	if err != nil {
		return result, err
	}
	current := "."
	for _, part := range strings.Split(clean, "/") {
		current = path.Join(current, part)
		component, err := root.Lstat(current)
		if err != nil {
			return result, err
		}
		if component.Mode()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("skill path component %q cannot be a symlink", current)
		}
		if current != clean && !component.IsDir() {
			return result, fmt.Errorf("skill path component %q must be a directory", current)
		}
	}
	info, err := root.Lstat(clean)
	if err != nil {
		return result, err
	}
	skillDir := clean
	skillFile := "SKILL.md"
	if info.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("skill reference cannot be a symlink")
	}
	if info.IsDir() {
	} else if info.Mode().IsRegular() && path.Base(clean) == "SKILL.md" {
		skillDir = path.Dir(clean)
	} else {
		return result, fmt.Errorf("skill reference must be a directory or SKILL.md file")
	}
	if skillDir == "." || skillDir == "" {
		return result, fmt.Errorf("skill directory must have a name")
	}
	entries, err := readTree(root, skillDir)
	if err != nil {
		return result, err
	}
	if _, ok := entries[skillFile]; !ok {
		return result, fmt.Errorf("skill directory is missing SKILL.md")
	}
	for file := range entries {
		if file == skillFile {
			continue
		}
		first := strings.SplitN(file, "/", 2)[0]
		if first != "scripts" && first != "references" && first != "assets" {
			return result, fmt.Errorf("unsupported skill resource %q; allowed directories are scripts, references, and assets", file)
		}
	}
	name, description, err := validateDocument(entries[skillFile], path.Base(skillDir))
	if err != nil {
		return result, err
	}
	result = Bundle{Name: name, Description: description, Files: entries}
	return result, nil
}

func cleanRelative(value string) (string, error) {
	if value == "" || strings.Contains(value, `\`) || strings.Contains(value, ":") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("skill path must be repository-relative")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("skill path escapes project root")
	}
	return clean, nil
}

func readTree(root *os.Root, directory string) (map[string][]byte, error) {
	result := map[string][]byte{}
	total := 0
	var walk func(string, string) error
	walk = func(relative, prefix string) error {
		directory, err := root.Open(relative)
		if err != nil {
			return err
		}
		entries, err := directory.ReadDir(-1)
		closeErr := directory.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		for _, entry := range entries {
			name := entry.Name()
			if name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
				return fmt.Errorf("invalid skill resource name %q", name)
			}
			rel := path.Join(relative, name)
			info, err := root.Lstat(rel)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("skill resource %q cannot be a symlink", path.Join(prefix, name))
			}
			if info.IsDir() {
				if err := walk(rel, path.Join(prefix, name)); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("skill resource %q must be a regular file", path.Join(prefix, name))
			}
			if info.Size() > maxFileBytes || total+int(info.Size()) > maxBundleBytes {
				return fmt.Errorf("skill bundle exceeds size limit")
			}
			file, err := root.Open(rel)
			if err != nil {
				return err
			}
			data, readErr := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
			closeErr := file.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if len(data) > maxFileBytes || total+len(data) > maxBundleBytes {
				return fmt.Errorf("skill bundle exceeds size limit")
			}
			total += len(data)
			result[path.Join(prefix, name)] = data
		}
		return nil
	}
	if err := walk(directory, "."); err != nil {
		return nil, err
	}
	return result, nil
}

func validateDocument(data []byte, directoryName string) (string, string, error) {
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", "", fmt.Errorf("SKILL.md must begin with YAML front matter")
	}
	front, body, ok := splitFrontMatter(text)
	if !ok || strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("SKILL.md must include terminated front matter and instructions")
	}
	var metadata struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	decoder := yaml.NewDecoder(bytes.NewBufferString(front))
	if err := decoder.Decode(&metadata); err != nil {
		return "", "", fmt.Errorf("SKILL.md front matter: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", "", fmt.Errorf("SKILL.md must contain one YAML front matter document")
	}
	if len(metadata.Name) < 1 || len(metadata.Name) > 64 || !skillNamePattern.MatchString(metadata.Name) || strings.Contains(metadata.Name, "--") {
		return "", "", fmt.Errorf("SKILL.md name must use lowercase letters, numbers, and single hyphens (1-64 characters)")
	}
	if metadata.Name != directoryName {
		return "", "", fmt.Errorf("SKILL.md name %q must match skill directory %q", metadata.Name, directoryName)
	}
	if strings.TrimSpace(metadata.Description) == "" || len(metadata.Description) > 1024 {
		return "", "", fmt.Errorf("SKILL.md description must contain 1-1024 characters")
	}
	return metadata.Name, metadata.Description, nil
}

func splitFrontMatter(text string) (front, body string, ok bool) {
	lines := strings.SplitAfter(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\n")) != "---" {
		return "", "", false
	}
	offset := len(lines[0])
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(strings.TrimSuffix(line, "\n")) == "---" {
			return strings.TrimSpace(text[len(lines[0]):offset]), strings.TrimSpace(text[offset+len(line):]), true
		}
		offset += len(line)
	}
	return "", "", false
}

// SortedPaths returns bundle paths in stable order for deterministic plans.
func (b Bundle) SortedPaths() []string {
	paths := make([]string, 0, len(b.Files))
	for name := range b.Files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}
