package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLLoaderKeepsWorkspaceV2Only(t *testing.T) {
	workspace := "workspace: services/auth\n"
	v1 := "version: 1\nproject: {name: sample}\nrules:\n  - id: rule\n    description: test\n    origin: ai\n    status: candidate\n    evidence:\n      - file: auth.go\n        " + workspace
	v1Path := filepath.Join(t.TempDir(), "v1.yaml")
	if err := os.WriteFile(v1Path, []byte(v1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := (YAMLLoader{}).Load(v1Path); err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Fatalf("v1 workspace was accepted: %v", err)
	}

	v2Path := filepath.Join(t.TempDir(), "v2.yaml")
	v2 := strings.Replace(v1, "version: 1", "version: 2", 1)
	if err := os.WriteFile(v2Path, []byte(v2), 0600); err != nil {
		t.Fatal(err)
	}
	harness, err := (YAMLLoader{}).Load(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := harness.Rules[0].Evidence[0].Workspace; got != "services/auth" {
		t.Fatalf("workspace = %q", got)
	}
}

func FuzzYAMLLoaderNeverPanics(f *testing.F) {
	f.Add([]byte("version: 1\nproject: {name: sample}\nrules: []\n"))
	f.Add([]byte("---\n- arbitrary\n---\nsecond\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 256<<10 {
			t.Skip()
		}
		path := filepath.Join(t.TempDir(), "harness.yaml")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, _ = (YAMLLoader{}).Load(path)
	})
}
