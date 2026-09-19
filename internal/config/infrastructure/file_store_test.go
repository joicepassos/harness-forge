package infrastructure

import (
	"harnessforge/internal/config/domain"
	"harnessforge/internal/inputlimits"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileStoreReplacesPreferencesAndBoundsInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preferences.json")
	store := FileStore{Path: path}
	if err := store.Save(domain.Preferences{Provider: "openai"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(domain.Preferences{Provider: "deepseek"}); err != nil {
		t.Fatal(err)
	}
	preferences, err := store.Load()
	if err != nil || preferences.Provider != "deepseek" {
		t.Fatalf("preferences=%#v err=%v", preferences, err)
	}
	if err := os.WriteFile(path, make([]byte, inputlimits.PreferencesBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("oversized preferences accepted")
	}
}

func TestFileStoreRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires developer mode or elevated privileges")
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"provider":"openai"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	store := FileStore{Path: path}
	if _, err := store.Load(); err == nil {
		t.Fatal("symlink accepted for load")
	}
	if err := store.Save(domain.Preferences{Provider: "deepseek"}); err == nil {
		t.Fatal("symlink accepted for save")
	}
}
