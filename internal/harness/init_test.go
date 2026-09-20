package harness

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestInitCreatesHarnessFile(t *testing.T) {
	dir := t.TempDir()

	path, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if path != ".harness/harness.yaml" {
		t.Fatalf("Init() path = %q, want %q", path, ".harness/harness.yaml")
	}

	content, err := os.ReadFile(filepath.Join(dir, ".harness", "harness.yaml"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	want := "version: 1\n\nproject:\n  name: " + strconv.Quote(filepath.Base(dir)) + "\n"
	if string(content) != want {
		t.Fatalf("harness.yaml = %q, want %q", string(content), want)
	}
}

func TestInitRejectsRedirectedHarnessDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires developer mode or elevated privileges")
	}
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, ".harness")); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(dir); err == nil {
		t.Fatal("redirected .harness directory accepted")
	}
}
