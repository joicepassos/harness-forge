package infrastructure

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHistoricalEvidenceAndCurrentChange(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, output)
		}
	}
	git("init")
	path := filepath.Join(dir, "code.go")
	if err := os.WriteFile(path, []byte("package fixture\nfunc Original() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "code.go")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "Create synthetic baseline")
	if err := os.WriteFile(path, []byte("package fixture\nfunc Renamed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := NewFileReader(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := reader.ContainsAt(context.Background(), "code.go", "Original", "HEAD"); err != nil || !found {
		t.Fatalf("baseline: %v %v", found, err)
	}
	if found, err := reader.Contains(context.Background(), "code.go", "Original"); err != nil || found {
		t.Fatalf("current: %v %v", found, err)
	}
	for _, path := range []string{"../code.go", "/code.go", "C:/code.go", "a\\code.go"} {
		if _, err := reader.ContainsAt(context.Background(), path, "Original", "HEAD"); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	if _, err := reader.ContainsAt(context.Background(), "code.go", "Original", "--invalid"); err == nil {
		t.Fatal("accepted invalid revision")
	}
}
