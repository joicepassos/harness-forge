package memory

import (
	"os"
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
