package main

import (
	"context"
	"fmt"
	"harnessforge/internal/securityboundary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The numbered browser also works with redirected input and screen readers.
func (s setupSession) browseSetupDocuments(ctx context.Context, root string, existing []setupDocument) ([]setupDocument, error) {
	files := append([]setupDocument(nil), existing...)
	current := root
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := setupBrowserEntries(root, current)
		if err != nil {
			return nil, err
		}
		rel, _ := filepath.Rel(root, current)
		fmt.Fprintf(s.output, "\nContext browser: %s\n", filepath.ToSlash(rel))
		for i, entry := range entries {
			mark, suffix := " ", ""
			if entry.IsDir() {
				suffix = "/"
			}
			for _, file := range files {
				path := filepath.Join(current, entry.Name())
				if file.Path == path || (entry.IsDir() && setupInside(path, file.Path)) {
					mark = "x"
				}
			}
			fmt.Fprintf(s.output, "  %d) [%s] %s%s\n", i+1, mark, entry.Name(), suffix)
		}
		fmt.Fprintf(s.output, "Selected context: %d document(s).\n", len(files))
		answer, err := s.ask("Number: open directory / toggle file; m NUMBER: mark directory; ..: parent; r: review/remove; p: advanced paths; Enter: continue: ")
		if err != nil {
			return nil, err
		}
		switch answer {
		case "":
			return files, nil
		case "..":
			if current != root {
				current = filepath.Dir(current)
			}
			continue
		case "p":
			files, err = s.setupDocumentPaths(ctx, root, files)
			if err != nil {
				return nil, err
			}
			continue
		case "r":
			for i, file := range files {
				fmt.Fprintf(s.output, "  %d) %s\n", i+1, file.Source)
			}
			remove, err := s.ask("Remove document number (Enter: return): ")
			if err != nil {
				return nil, err
			}
			if remove != "" {
				n, err := strconv.Atoi(remove)
				if err != nil || n < 1 || n > len(files) {
					fmt.Fprintln(s.output, "Choose a document number from the selection.")
				} else {
					files = append(files[:n-1], files[n:]...)
				}
			}
			continue
		}
		markDirectory := strings.HasPrefix(answer, "m ")
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(answer, "m ")))
		if err != nil || n < 1 || n > len(entries) {
			fmt.Fprintln(s.output, "Choose an entry number or a browser command.")
			continue
		}
		entry := entries[n-1]
		path := filepath.Join(current, entry.Name())
		if entry.IsDir() && !markDirectory {
			current = path
			continue
		}
		removed := false
		if !entry.IsDir() {
			for i, file := range files {
				if file.Path == path {
					files = append(files[:i], files[i+1:]...)
					removed = true
					break
				}
			}
		}
		if removed {
			continue
		}
		added, err := readSetupPath(ctx, root, path)
		if err == nil {
			candidate := append([]setupDocument(nil), files...)
			for _, file := range added {
				duplicate := false
				for _, selected := range candidate {
					duplicate = duplicate || selected.Path == file.Path
				}
				if !duplicate {
					candidate = append(candidate, file)
				}
			}
			if len(candidate) > setupMaxDocuments {
				err = fmt.Errorf("context exceeds %d files", setupMaxDocuments)
			} else if err = setupContextBytes(candidate, ""); err == nil {
				files = candidate
			}
		}
		if err != nil {
			fmt.Fprintf(s.output, "Skipped %q: %v\n", entry.Name(), err)
		}
	}
}

func setupBrowserEntries(root, current string) ([]os.DirEntry, error) {
	if !setupInside(root, current) {
		return nil, fmt.Errorf("browser must remain inside the project")
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil || !setupInside(canonicalRoot, resolved) {
		return nil, fmt.Errorf("browser path leaves the project through a link")
	}
	entries, err := os.ReadDir(current)
	if err != nil {
		return nil, err
	}
	var visible []os.DirEntry
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || securityboundary.SensitivePath(filepath.Join(current, entry.Name())) {
			continue
		}
		if entry.IsDir() {
			if securityboundary.SkipRepositoryDirectory(entry.Name()) || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
		} else if !entry.Type().IsRegular() || !setupTextExtension(entry.Name()) {
			continue
		}
		visible = append(visible, entry)
	}
	return visible, nil
}
