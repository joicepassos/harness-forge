package repository

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func TestScanGoMonorepoFixturePreservesWorkspaceRoots(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "repos", "go-monorepo")
	snapshot, err := Scan(context.Background(), root, ScanOptions{HonorIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	type entry struct {
		path, kind, workspace string
	}
	got := make([]entry, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		got = append(got, entry{file.Path, file.Kind, file.Workspace})
	}
	sort.Slice(got, func(i, j int) bool { return got[i].path < got[j].path })
	want := []entry{
		{path: "apps/api/go.mod", kind: "manifest", workspace: "apps/api"},
		{path: "apps/api/main.go", kind: "source", workspace: "apps/api"},
		{path: "go.work", kind: "other", workspace: "."},
		{path: "libs/core/core.go", kind: "source", workspace: "libs/core"},
		{path: "libs/core/go.mod", kind: "manifest", workspace: "libs/core"},
	}
	if len(got) != len(want) {
		t.Fatalf("fixture files = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("fixture file %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
