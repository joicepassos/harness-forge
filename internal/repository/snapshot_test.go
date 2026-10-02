package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestScanGoSingleFixtureMatchesCharacterization(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "repos", "go-single")
	snapshot, err := Scan(context.Background(), root, ScanOptions{HonorIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	type entry struct {
		Path      string `json:"path"`
		Kind      string `json:"kind"`
		Workspace string `json:"workspace"`
	}
	got := make([]entry, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		got = append(got, entry{file.Path, file.Kind, file.Workspace})
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if file.Size != info.Size() {
			t.Errorf("%s size = %d, want actual checkout size %d", file.Path, file.Size, info.Size())
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Path < got[j].Path })
	golden, err := os.ReadFile(filepath.Join(filepath.Dir(root), "go-single.snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Files []entry `json:"files"`
	}
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want.Files) {
		t.Fatalf("fixture files = %d, want %d: %#v", len(got), len(want.Files), got)
	}
	for i := range got {
		if got[i] != want.Files[i] {
			t.Fatalf("fixture file %d = %#v, want %#v", i, got[i], want.Files[i])
		}
	}
}

func TestScanHonorsIgnoreAndReadIsBounded(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "ignored"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Scan(context.Background(), root, ScanOptions{HonorIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if len(snapshot.Files) != 2 {
		t.Fatalf("files = %#v", snapshot.Files)
	}
	for _, file := range snapshot.Files {
		if file.Path == "main.go" && (file.Kind != "source" || file.Workspace != ".") {
			t.Fatalf("classification = %+v", file)
		}
	}
	data, truncated, err := snapshot.Read("main.go", 4)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "0123" || !truncated {
		t.Fatalf("read = %q, truncated=%v", data, truncated)
	}
	if _, _, err := snapshot.Read("../outside", 4); err == nil {
		t.Fatal("path escape accepted")
	}
}

func TestSnapshotReadRejectsClosedSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := snapshot.Read("README.md", 100); err == nil {
		t.Fatal("Read succeeded after snapshot.Close")
	}
}

func TestScanEnforcesFileLimitAfterMetadataEnumeration(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Scan(context.Background(), root, ScanOptions{MaxFiles: 2}); err == nil {
		t.Fatal("expected file limit error")
	}
}

func TestReadRejectsSymlinkOutsideRoot(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("symlink setup requires platform support")
	}
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Fatal(err)
	}
	snapshot := &Snapshot{Root: root}
	if _, _, err := snapshot.Read("link.txt", 100); err == nil {
		t.Fatal("outside symlink was read")
	}
}

func TestWorkspaceClassificationUsesCommonMonorepoRoots(t *testing.T) {
	cases := map[string]string{
		"packages/auth/service.go": "packages/auth",
		"apps/web/main.ts":         "apps/web",
		"services/api/main.go":     "services/api",
		"cmd/server/main.go":       "cmd/server",
		"internal/shared/types.go": ".",
	}
	for path, want := range cases {
		if got := workspace(path); got != want {
			t.Errorf("workspace(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDefaultSkipDirsExcludesLocalTreesButKeepsRepositoryMetadata(t *testing.T) {
	skipped := DefaultSkipDirs()
	for _, name := range []string{"node_modules", ".codex", ".agents", ".next", "build", "vendor"} {
		if !skipped[name] {
			t.Fatalf("%q is not in default skip directories", name)
		}
	}
	if skipped[".github"] {
		t.Fatal(".github must remain available as repository metadata")
	}
}

func TestScanDefaultSkipDirsDoesNotReadNodeModules(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"node_modules/pkg/index.js", ".github/workflows/ci.yml"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(path), 0600); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := Scan(context.Background(), root, ScanOptions{HonorIgnores: true, SkipDirs: DefaultSkipDirs()})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	for _, file := range snapshot.Files {
		if strings.HasPrefix(file.Path, "node_modules/") {
			t.Fatalf("node_modules file was scanned: %s", file.Path)
		}
	}
	foundGitHub := false
	for _, file := range snapshot.Files {
		if file.Path == ".github/workflows/ci.yml" {
			foundGitHub = true
		}
	}
	if !foundGitHub {
		t.Fatal(".github metadata was incorrectly skipped")
	}
}

func TestWorkspaceClassificationDerivesManifestRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "frontend", "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "frontend", "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "frontend", "src", "app.ts"), []byte("export const app = true"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Scan(context.Background(), root, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	for _, file := range snapshot.Files {
		if strings.HasPrefix(file.Path, "frontend/") && file.Workspace != "frontend" {
			t.Fatalf("%s workspace = %q", file.Path, file.Workspace)
		}
	}
}

func TestFileKindClassification(t *testing.T) {
	cases := map[string]string{
		"go.mod":          "manifest",
		"README.md":       "doc",
		"config/app.yml":  "config",
		"internal/app.go": "source",
		"assets/logo.bin": "other",
	}
	for path, want := range cases {
		if got := classifyKind(path); got != want {
			t.Errorf("classifyKind(%q) = %q, want %q", path, got, want)
		}
	}
}
