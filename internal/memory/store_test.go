package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(path, repo, checkout string) *Store {
	return &Store{root: path, repositoryID: repo, checkoutID: checkout}
}

func TestCaptureStoresExplicitCandidateAndDeduplicatesOnlySameSource(t *testing.T) {
	store := testStore(filepath.Join(t.TempDir(), "local"), "repo", "worktree-a")
	first, err := store.Capture("Keep evidence before publishing.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != Candidate || first.Reviewer != "" || first.Source != "manual" {
		t.Fatalf("capture was implicitly promoted: %#v", first)
	}
	duplicate, err := store.Capture(first.Content, "manual", nil)
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("same source wasn't idempotent: %#v %v", duplicate, err)
	}
	other, err := store.Capture(first.Content, "command-result", nil)
	if err != nil || other.ID == first.ID {
		t.Fatalf("different provenance was conflated: %#v %v", other, err)
	}
	items, err := store.List()
	if err != nil || len(items) != 2 {
		t.Fatalf("list=%d err=%v", len(items), err)
	}
}

func TestReviewRequiresReviewerAndGarbageCollectionPreservesPending(t *testing.T) {
	store := testStore(filepath.Join(t.TempDir(), "state"), "repo", "worktree-a")
	pending, err := store.Capture("Never discard pending work.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Capture("Resolved old note.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Review(resolved.ID, Approved, ""); err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("missing reviewer accepted: %v", err)
	}
	resolved, err = store.Review(resolved.ID, Rejected, "alice")
	if err != nil {
		t.Fatal(err)
	}
	resolved.CreatedAt = time.Now().Add(-48 * time.Hour)
	items, _ := store.List()
	for i := range items {
		if items[i].ID == resolved.ID {
			items[i].CreatedAt = resolved.CreatedAt
		}
	}
	if err := store.save(items); err != nil {
		t.Fatal(err)
	}
	plan, err := store.PreviewGC(time.Now(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Remove) != 1 || plan.Remove[0] != resolved.ID || plan.PreservePending != 1 {
		t.Fatalf("GC plan did not preserve candidate: %#v", plan)
	}
	if _, err := store.ApplyGC(time.Now(), 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	items, err = store.List()
	if err != nil || len(items) != 1 || items[0].ID != pending.ID {
		t.Fatalf("pending observation lost: %#v %v", items, err)
	}
}

func TestStorePartitionsCheckoutIdentityAndDetectsTampering(t *testing.T) {
	root := t.TempDir()
	one := testStore(filepath.Join(root, "one"), "same-repository", "worktree-one")
	two := testStore(filepath.Join(root, "two"), "same-repository", "worktree-two")
	item, err := one.Capture("Local only.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if items, err := two.List(); err != nil || len(items) != 0 {
		t.Fatalf("worktree received another checkout state: %#v %v", items, err)
	}
	path := filepath.Join(one.root, "observations.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), item.Content, "Edited local state", 1)
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := one.List(); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("tampered observation accepted: %v", err)
	}
}

func TestOpenKeepsObservationsSeparateAcrossGitWorktrees(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	main := filepath.Join(root, "main")
	worktree := filepath.Join(root, "worktree")
	runGit := func(dir string, args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-C", dir}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if err := os.MkdirAll(main, 0700); err != nil {
		t.Fatal(err)
	}
	// Initialize a self-contained repository; no user configuration, providers,
	// or repositories outside the test's temporary directory are involved.
	command := exec.Command(git, "-C", main, "init", "-q")
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("git repository initialization unavailable: %v\n%s", err, output)
	}
	runGit(main, "-c", "user.name=HarnessForge test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "initial")
	initial := runGit(main, "rev-parse", "HEAD")
	command = exec.Command(git, "-C", main, "worktree", "add", "-b", "isolated-worktree", worktree)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("git worktree is unavailable: %v\n%s", err, output)
	}
	defer func() {
		_ = exec.Command(git, "-C", main, "worktree", "remove", "--force", worktree).Run()
	}()
	runGit(worktree, "-c", "user.name=HarnessForge test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "worktree revision")
	secondary := runGit(worktree, "rev-parse", "HEAD")
	if initial == secondary {
		t.Fatalf("test worktrees unexpectedly share HEAD %s", initial)
	}

	mainStore, err := Open(main)
	if err != nil {
		t.Fatal(err)
	}
	worktreeStore, err := Open(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if mainStore.repositoryID != worktreeStore.repositoryID {
		t.Fatalf("worktrees do not share repository identity: %q != %q", mainStore.repositoryID, worktreeStore.repositoryID)
	}
	if mainStore.checkoutID == worktreeStore.checkoutID || mainStore.root == worktreeStore.root {
		t.Fatalf("worktrees share local state identity: %#v %#v", mainStore, worktreeStore)
	}

	fromMain, err := mainStore.Capture("Observation captured in the main checkout.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	fromWorktree, err := worktreeStore.Capture("Observation captured in the linked worktree.", "manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if fromMain.Revision != initial || fromWorktree.Revision != secondary {
		t.Fatalf("observations did not record their checkout HEADs: main=%q worktree=%q", fromMain.Revision, fromWorktree.Revision)
	}
	mainItems, err := mainStore.List()
	if err != nil {
		t.Fatal(err)
	}
	worktreeItems, err := worktreeStore.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(mainItems) != 1 || mainItems[0].ID != fromMain.ID || mainItems[0].Content != fromMain.Content {
		t.Fatalf("main checkout observations leaked or disappeared: %#v", mainItems)
	}
	if len(worktreeItems) != 1 || worktreeItems[0].ID != fromWorktree.ID || worktreeItems[0].Content != fromWorktree.Content {
		t.Fatalf("linked worktree observations leaked or disappeared: %#v", worktreeItems)
	}
}

func TestGCQuotaRemovesOldestResolvedButNeverPending(t *testing.T) {
	store := testStore(filepath.Join(t.TempDir(), "state"), "repo", "checkout")
	now := time.Now().UTC()
	first, _ := store.Capture("old rejected", "manual", nil)
	second, _ := store.Capture("new rejected", "manual", nil)
	pending, _ := store.Capture("pending candidate", "manual", nil)
	items, _ := store.List()
	for i := range items {
		switch items[i].ID {
		case first.ID:
			items[i].State = Rejected
			items[i].Reviewer = "alice"
			items[i].CreatedAt = now.Add(-3 * time.Hour)
		case second.ID:
			items[i].State = Rejected
			items[i].Reviewer = "alice"
			items[i].CreatedAt = now.Add(-time.Hour)
		}
	}
	if err := store.save(items); err != nil {
		t.Fatal(err)
	}
	plan, err := store.PreviewGC(now, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Remove) != 1 || plan.Remove[0] != first.ID || plan.PreservePending != 1 || plan.QuotaExceeded {
		t.Fatalf("quota preview=%#v", plan)
	}
	if _, err := store.ApplyGC(now, 0, 2); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.List()
	if err != nil || len(remaining) != 2 {
		t.Fatalf("quota apply=%#v err=%v", remaining, err)
	}
	seen := map[string]bool{}
	for _, item := range remaining {
		seen[item.ID] = true
	}
	if !seen[pending.ID] || !seen[second.ID] {
		t.Fatalf("wrong observations survived quota: %#v", remaining)
	}
}
