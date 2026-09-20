package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"harnessforge/internal/securityboundary"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	setupMaxDocuments     = 24
	setupMaxDocumentBytes = 16 << 10
	setupMaxContextBytes  = 128 << 10
	setupMaxNotesBytes    = 8 << 10
)

type setupDocument struct {
	Source    string
	Path      string
	Text      string
	Truncated bool
	Relative  bool
}

type setupSession struct {
	reader *bufio.Reader
	output io.Writer
}

func (s setupSession) ask(question string) (string, error) {
	if _, err := fmt.Fprint(s.output, question); err != nil {
		return "", err
	}
	line, err := s.reader.ReadString('\n')
	if len(line) > 16<<10 {
		return "", fmt.Errorf("input line is too long")
	}
	if err != nil && err != io.EOF {
		return "", err
	}
	if err == io.EOF && line == "" {
		return "", io.EOF
	}
	return strings.TrimSpace(line), nil
}

func (s setupSession) confirm(question string) (bool, error) {
	answer, err := s.ask(question + " [y/N]: ")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

func (s setupSession) notes() (string, error) {
	fmt.Fprintln(s.output, "Additional observations or instructions (one per line; empty line to continue):")
	var lines []string
	length := 0
	for {
		line, err := s.ask("> ")
		if err != nil {
			return "", err
		}
		if line == "" {
			return strings.Join(lines, "\n"), nil
		}
		length += len(line) + 1
		if length > setupMaxNotesBytes {
			return "", fmt.Errorf("observations exceed 8 KiB")
		}
		lines = append(lines, line)
	}
}

func (s setupSession) selectedDocuments(ctx context.Context, root string, existing []setupDocument) ([]setupDocument, error) {
	fmt.Fprintln(s.output, "Add context files or directories (one path per line; empty line to continue).")
	files := append([]setupDocument(nil), existing...)
	seen := map[string]bool{}
	for _, file := range files {
		seen[file.Path] = true
	}
	for {
		answer, err := s.ask("Path: ")
		if err != nil {
			return nil, err
		}
		if answer == "" {
			return files, nil
		}
		added, err := readSetupPath(ctx, root, answer)
		if err != nil {
			fmt.Fprintf(s.output, "Skipped %q: %v\n", answer, err)
			continue
		}
		count := 0
		for _, file := range added {
			if seen[file.Path] {
				continue
			}
			if len(files) >= setupMaxDocuments {
				return nil, fmt.Errorf("context exceeds %d files", setupMaxDocuments)
			}
			files = append(files, file)
			seen[file.Path] = true
			count++
		}
		fmt.Fprintf(s.output, "Added %d text document(s).\n", count)
	}
}

func defaultSetupDocuments(ctx context.Context, root string) []setupDocument {
	var documents []setupDocument
	for _, name := range []string{"README.md", "AGENTS.md", "CLAUDE.md", "package.json", "go.mod", "pyproject.toml", "pom.xml", "build.gradle", "build.gradle.kts"} {
		path := filepath.Join(root, name)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		items, err := readSetupPath(ctx, root, name)
		if err == nil {
			documents = append(documents, items...)
		}
	}
	return documents
}

func readSetupPath(ctx context.Context, root, requested string) ([]setupDocument, error) {
	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if securityboundary.SensitivePath(path) {
		return nil, fmt.Errorf("sensitive path is not accepted")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("links are not accepted")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if securityboundary.SensitivePath(resolved) {
		return nil, fmt.Errorf("sensitive path is not accepted")
	}
	if setupInside(root, path) {
		canonicalRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, err
		}
		if !setupInside(canonicalRoot, resolved) {
			return nil, fmt.Errorf("path leaves the project through a link")
		}
	}
	if !info.IsDir() {
		return readSetupFile(root, path)
	}
	var paths []string
	err = filepath.WalkDir(path, func(child string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if child != path && (securityboundary.SkipRepositoryDirectory(entry.Name()) || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		if !setupTextExtension(child) || securityboundary.SensitivePath(child) {
			return nil
		}
		paths = append(paths, child)
		if len(paths) > setupMaxDocuments {
			return fmt.Errorf("directory contains more than %d text documents", setupMaxDocuments)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no supported text documents found")
	}
	var out []setupDocument
	for _, child := range paths {
		items, err := readSetupFile(root, child)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", child, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

func readSetupFile(root, path string) ([]setupDocument, error) {
	if !setupTextExtension(path) {
		return nil, fmt.Errorf("unsupported format; use a UTF-8 text document")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("not a regular file")
	}
	stream, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, setupMaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	truncated := len(data) > setupMaxDocumentBytes
	if truncated {
		data = data[:setupMaxDocumentBytes]
	}
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return nil, fmt.Errorf("binary or non-UTF-8 document")
	}
	if securityboundary.ContainsSensitiveContent(data) {
		return nil, fmt.Errorf("document appears to contain credentials")
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, fmt.Errorf("document is empty")
	}
	inside := setupInside(root, path)
	digest := sha256.Sum256([]byte(path))
	source := fmt.Sprintf("user-file:%s-%x", filepath.Base(path), digest[:4])
	if inside {
		rel, _ := filepath.Rel(root, path)
		source = "repository-file:" + filepath.ToSlash(rel)
	}
	return []setupDocument{{Source: source, Path: path, Text: text, Truncated: truncated, Relative: inside}}, nil
}

func setupInside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func setupTextExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".txt", ".rst", ".adoc", ".yaml", ".yml", ".json", ".toml", ".xml", ".gradle":
		return true
	}
	return filepath.Base(path) == "go.mod" || filepath.Base(path) == "Dockerfile"
}

func setupContextBytes(documents []setupDocument, notes string) error {
	total := len(notes)
	for _, document := range documents {
		total += len(document.Text)
	}
	if total > setupMaxContextBytes {
		return fmt.Errorf("selected context exceeds 128 KiB; choose fewer documents")
	}
	return nil
}
