package repository

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"harnessforge/internal/inputlimits"
	"harnessforge/internal/securityboundary"
)

// File is a metadata-only entry in a repository snapshot.
type File struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Kind      string `json:"kind"`
	Workspace string `json:"workspace"`
}

// Snapshot is the authoritative inventory produced by one repository scan.
type Snapshot struct {
	Root  string
	Files []File
	root  *os.Root
}

// WorkspaceForPath returns the workspace recorded for a repository-relative
// path. It keeps analyzer findings tied to the same inventory used by the
// context and indexing pipelines.
func (snapshot *Snapshot) WorkspaceForPath(path string) string {
	if snapshot == nil {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	for _, file := range snapshot.Files {
		if filepath.ToSlash(filepath.Clean(file.Path)) == path {
			return file.Workspace
		}
	}
	return ""
}

// RepositorySnapshot is the descriptive name used by the architecture plan;
// Snapshot remains the concise compatibility name.
type RepositorySnapshot = Snapshot

type ScanOptions struct {
	MaxFiles     int
	HonorIgnores bool
	SkipHidden   bool
	SkipDirs     map[string]bool
}

// DefaultSkipDirs excludes local tooling, dependency, and build trees while
// preserving meaningful repository metadata such as .github.
func DefaultSkipDirs() map[string]bool {
	return map[string]bool{
		".agents": true, ".codex": true, ".idea": true, ".next": true,
		"node_modules": true, "vendor": true, "build": true, "dist": true,
		"target": true, "out": true,
	}
}

func Scan(ctx context.Context, root string, options ScanOptions) (*Snapshot, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository must be a non-symlink directory")
	}
	limit := options.MaxFiles
	if limit <= 0 || limit > inputlimits.RepositoryFiles {
		limit = inputlimits.RepositoryFiles
	}
	var ignored securityboundary.Ignored
	if options.HonorIgnores {
		ignored, err = securityboundary.LoadGitIgnoreContext(ctx, absolute, limit)
		if err != nil {
			return nil, err
		}
	}
	snapshot := &Snapshot{Root: absolute}
	snapshot.root, err = os.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(absolute, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if securityboundary.SkipRepositoryDirectory(entry.Name()) || options.SkipDirs[entry.Name()] || (options.SkipHidden && strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if options.HonorIgnores && ignored.Match(rel) {
			return nil
		}
		if len(snapshot.Files) >= limit {
			return fmt.Errorf("repository scan exceeds %d files", limit)
		}
		stat, err := entry.Info()
		if err != nil {
			return err
		}
		snapshot.Files = append(snapshot.Files, File{Path: rel, Size: stat.Size(), Kind: classifyKind(rel), Workspace: workspace(rel)})
		return nil
	})
	if err != nil {
		snapshot.root.Close()
		return nil, err
	}
	assignManifestWorkspaces(snapshot.Files)
	return snapshot, nil
}

// Close releases the root handle held by the snapshot.
func (s *Snapshot) Close() error {
	if s == nil || s.root == nil {
		return nil
	}
	return s.root.Close()
}

func classifyKind(path string) string {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case name == "go.mod" || name == "package.json" || name == "pyproject.toml" || name == "pom.xml" || name == "build.gradle":
		return "manifest"
	case ext == ".md" || name == "readme":
		return "doc"
	case ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml" || ext == ".ini":
		return "config"
	case ext == ".go" || ext == ".ts" || ext == ".js" || ext == ".py" || ext == ".java" || ext == ".rs" || ext == ".cs":
		return "source"
	default:
		return "other"
	}
}

func workspace(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) > 1 && (parts[0] == "packages" || parts[0] == "apps" || parts[0] == "services" || parts[0] == "cmd") {
		return parts[0] + "/" + parts[1]
	}
	return "."
}

func assignManifestWorkspaces(files []File) {
	roots := map[string]bool{}
	for _, file := range files {
		name := strings.ToLower(filepath.Base(file.Path))
		if name == "package.json" || name == "go.mod" || name == "pyproject.toml" || name == "pom.xml" || name == "build.gradle" || name == "build.gradle.kts" || name == "cargo.toml" || strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".csproj") {
			root := filepath.ToSlash(filepath.Dir(file.Path))
			if root != "." && root != "" {
				roots[root] = true
			}
		}
	}
	for i := range files {
		if files[i].Workspace != "." {
			continue
		}
		path := filepath.ToSlash(files[i].Path)
		best := ""
		for root := range roots {
			if (path == root || strings.HasPrefix(path, root+"/")) && len(root) > len(best) {
				best = root
			}
		}
		if best != "" {
			files[i].Workspace = best
		}
	}
}

func (s *Snapshot) Read(path string, maxBytes int64) ([]byte, bool, error) {
	if s == nil || unsafePath(path) {
		return nil, false, fmt.Errorf("unsafe repository path")
	}
	if maxBytes <= 0 {
		return nil, false, fmt.Errorf("maxBytes must be positive")
	}
	if s.root == nil {
		return nil, false, fmt.Errorf("repository snapshot is closed")
	}
	file, err := s.root.Open(filepath.ToSlash(filepath.Clean(path)))
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		return data[:maxBytes], true, nil
	}
	return data, false, nil
}

func unsafePath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	return filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
