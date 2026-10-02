package infrastructure

import (
	"context"
	"encoding/json"
	"harnessforge/internal/indexing/domain"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDocumentsAndStructuralChunksRespectBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0700)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# First\nAlpha\n# Second\nBeta"), 0600)
	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package fixture"), 0600)
	os.WriteFile(filepath.Join(dir, ".git", "hidden.md"), []byte("secret"), 0600)
	documents, err := (Documents{}).Documents(context.Background(), dir)
	if err != nil || len(documents) != 1 {
		t.Fatalf("%#v %v", documents, err)
	}
	chunks := (Markdown{}).Chunks(documents[0])
	if len(chunks) != 2 || chunks[0].StartLine != 1 || chunks[1].StartLine != 3 {
		t.Fatalf("%#v", chunks)
	}
}
func TestDocumentsExcludeIgnoredAndSensitiveContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("private/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "private", "notes.md"), []byte("not for indexing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("api_key=fictional-credential-value"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "safe.md"), []byte("safe documentation"), 0600); err != nil {
		t.Fatal(err)
	}
	documents, err := (Documents{}).Documents(context.Background(), dir)
	if err != nil || len(documents) != 1 || documents[0].Path != "safe.md" {
		t.Fatalf("documents=%#v err=%v", documents, err)
	}
}

func TestDocumentsHonorNestedIgnoreAndQuotedCredentials(t *testing.T) {
	dir := t.TempDir()
	for path, content := range map[string]string{
		".gitignore":            "private/\n",
		"sub/.gitignore":        "notes.md\n",
		"docs/private/notes.md": "private document",
		"sub/notes.md":          "nested private document",
		"quoted.md":             `password = "fictional-password-for-test"`,
		"credentials.md":        "non-secret wording",
		"safe.md":               "safe documentation",
	} {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	documents, err := (Documents{}).Documents(context.Background(), dir)
	if err != nil || len(documents) != 1 || documents[0].Path != "safe.md" {
		t.Fatalf("documents=%#v err=%v", documents, err)
	}
}
func TestJSONStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	text := "document text"
	index := domain.Index{
		Version: "index-v1", Model: "m", Dimensions: 2,
		Chunks: []domain.Chunk{{
			ID: hash([]byte("one")), Source: "README.md", SourceHash: hash([]byte("source")),
			Text: text, Hash: hash([]byte(text)), Model: "m", StartLine: 1, EndLine: 1, Vector: []float64{1, 0},
		}},
	}
	store := testJSONStore(t)
	if err := store.Save(dir, index); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harness")); !os.IsNotExist(err) {
		t.Fatalf("index cache created repository state at .harness: %v", err)
	}
	index.Model = "updated"
	index.Chunks[0].Model = "updated"
	if err := store.Save(dir, index); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(dir)
	if err != nil || loaded.Version != "index-v1" || loaded.Model != "updated" || len(loaded.Chunks) != 1 {
		t.Fatal(loaded, err)
	}
}

func TestJSONStoreSeparatesRepositoryAndWorktreeCaches(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	store := JSONStore{CacheDir: cache}
	repo := t.TempDir()
	worktree := t.TempDir()
	if err := store.Save(repo, validTestIndex("repo")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(worktree, validTestIndex("worktree")); err != nil {
		t.Fatal(err)
	}
	repoPath, err := store.indexPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	worktreePath, err := store.indexPath(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if repoPath == worktreePath {
		t.Fatalf("repository and worktree share cache path %q", repoPath)
	}
	for root, want := range map[string]string{repo: "repo", worktree: "worktree"} {
		loaded, err := store.Load(root)
		if err != nil || loaded.Chunks[0].Text != want {
			t.Fatalf("load %s = %#v, %v; want text %q", root, loaded, err, want)
		}
	}
}

func TestJSONStoreSeparatesLinkedGitWorktreeCaches(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	main := filepath.Join(root, "main")
	worktree := filepath.Join(root, "worktree")
	if err := os.MkdirAll(main, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(git, "-C", main, "init", "-q")
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("git repository initialization unavailable: %v\n%s", err, output)
	}
	runGit := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-C", dir}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	runGit(main, "-c", "user.name=HarnessForge test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial")
	command = exec.Command(git, "-C", main, "worktree", "add", "-b", "index-worktree", worktree)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("git worktree is unavailable: %v\n%s", err, output)
	}
	defer func() {
		_ = exec.Command(git, "-C", main, "worktree", "remove", "--force", worktree).Run()
	}()
	if mainGitDir := runGit(main, "rev-parse", "--git-dir"); !filepath.IsAbs(mainGitDir) {
		mainGitDir = filepath.Join(main, mainGitDir)
		if _, err := os.Stat(filepath.Join(mainGitDir, "worktrees")); err != nil {
			t.Fatalf("main checkout is not linked to a worktree administration directory: %v", err)
		}
	}
	store := JSONStore{CacheDir: filepath.Join(root, "cache")}
	if err := store.Save(main, validTestIndex("main checkout")); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(worktree, validTestIndex("linked worktree")); err != nil {
		t.Fatal(err)
	}
	mainPath, err := store.indexPath(main)
	if err != nil {
		t.Fatal(err)
	}
	worktreePath, err := store.indexPath(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if mainPath == worktreePath {
		t.Fatalf("linked worktree shares generated index cache path %q", mainPath)
	}
	for checkout, want := range map[string]string{main: "main checkout", worktree: "linked worktree"} {
		loaded, err := store.Load(checkout)
		if err != nil || len(loaded.Chunks) != 1 || loaded.Chunks[0].Text != want {
			t.Fatalf("load %s = %#v, %v; want text %q", checkout, loaded, err, want)
		}
	}
}

func TestJSONStoreReadsLegacyIndexWithoutDeletingIt(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, ".harness")
	if err := os.Mkdir(legacyDir, 0700); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "index.json")
	legacyBytes, err := json.Marshal(validTestIndex("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, legacyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	store := JSONStore{CacheDir: filepath.Join(t.TempDir(), "cache")}
	loaded, err := store.Load(root)
	if err != nil || loaded.Chunks[0].Text != "legacy" {
		t.Fatalf("legacy load = %#v, %v", loaded, err)
	}
	if err := store.Save(root, validTestIndex("cached")); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.Load(root)
	if err != nil || loaded.Chunks[0].Text != "cached" {
		t.Fatalf("cache should take precedence after save: %#v, %v", loaded, err)
	}
	stillThere, err := os.ReadFile(legacyPath)
	if err != nil || string(stillThere) != string(legacyBytes) {
		t.Fatalf("legacy index was changed or deleted: %v", err)
	}
}

func validTestIndex(text string) domain.Index {
	return domain.Index{
		Version: "index-v1", Model: "m", Dimensions: 1,
		Chunks: []domain.Chunk{{
			ID: hash([]byte("id-" + text)), Source: "README.md", SourceHash: hash([]byte("source")),
			Text: text, Hash: hash([]byte(text)), Model: "m", StartLine: 1, EndLine: 1, Vector: []float64{1},
		}},
	}
}
func TestJSONStoreDoesNotWriteThroughHarnessDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires developer mode or elevated privileges")
	}
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, ".harness")); err != nil {
		t.Fatal(err)
	}
	store := testJSONStore(t)
	if err := store.Save(dir, domain.Index{Version: "index-v1", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "index.json")); !os.IsNotExist(err) {
		t.Fatalf("cache save wrote through .harness symlink: %v", err)
	}
}

func TestJSONStoreRejectsMalformedAndOversizedIndexes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".harness"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".harness", "index.json")
	if err := os.WriteFile(path, []byte(`{"Version":"old"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := testJSONStore(t).Load(dir); err == nil {
		t.Fatal("malformed index accepted")
	}
	if err := os.WriteFile(path, make([]byte, 16<<20+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := testJSONStore(t).Load(dir); err == nil {
		t.Fatal("oversized index accepted")
	}
}

func testJSONStore(t *testing.T) JSONStore {
	t.Helper()
	return JSONStore{CacheDir: filepath.Join(t.TempDir(), "user-cache")}
}
