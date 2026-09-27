package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LayoutKind string

const (
	LayoutHarness LayoutKind = "harness"
	LayoutForge   LayoutKind = "forge"
)

type ProjectLayout struct {
	Root         string
	Kind         LayoutKind
	HarnessPath  string
	ManifestPath string
}

// ResolveLayout discovers the project's single configuration source. If both
// layouts exist, callers must select one explicitly to avoid source drift.
func ResolveLayout(root, selection string) (ProjectLayout, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ProjectLayout{}, err
	}
	if selection != "" && selection != string(LayoutHarness) && selection != string(LayoutForge) {
		return ProjectLayout{}, fmt.Errorf("unknown layout %q; expected harness or forge", selection)
	}
	harnessPath := filepath.Join(absolute, ".harness", "harness.yaml")
	forgePath := filepath.Join(absolute, ".forge", "forge.yaml")
	harnessExists, err := regularConfig(harnessPath)
	if err != nil {
		return ProjectLayout{}, err
	}
	forgeExists, err := regularConfig(forgePath)
	if err != nil {
		return ProjectLayout{}, err
	}
	if selection == "" {
		switch {
		case harnessExists && forgeExists:
			return ProjectLayout{}, fmt.Errorf("both .harness/harness.yaml and .forge/forge.yaml exist; select --layout harness or --layout forge")
		case harnessExists:
			selection = string(LayoutHarness)
		case forgeExists:
			selection = string(LayoutForge)
		default:
			return ProjectLayout{}, fmt.Errorf("no supported project layout found under %s", absolute)
		}
	}
	if selection == string(LayoutHarness) {
		if !harnessExists {
			return ProjectLayout{}, fmt.Errorf("selected harness layout is missing %s", harnessPath)
		}
		return ProjectLayout{Root: absolute, Kind: LayoutHarness, HarnessPath: harnessPath}, nil
	}
	if !forgeExists {
		return ProjectLayout{}, fmt.Errorf("selected forge layout is missing %s", forgePath)
	}
	return ProjectLayout{Root: absolute, Kind: LayoutForge, ManifestPath: forgePath}, nil
}

// ResolveReference resolves a manifest path against the project root and
// rejects absolute paths, alternate separators, and traversal outside the root.
func (l ProjectLayout) ResolveReference(reference string) (string, error) {
	if strings.TrimSpace(reference) == "" || filepath.IsAbs(reference) || strings.Contains(reference, "\\") || strings.Contains(reference, ":") {
		return "", fmt.Errorf("reference must be a non-empty repository-relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(reference))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reference escapes the project root")
	}
	resolved := filepath.Join(l.Root, clean)
	relative, err := filepath.Rel(l.Root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("reference escapes the project root")
	}
	return resolved, nil
}

func regularConfig(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("configuration must be a regular non-symlink file: %s", path)
	}
	return true, nil
}
