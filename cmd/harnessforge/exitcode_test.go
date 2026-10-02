package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	generation "harnessforge/internal/generation/infrastructure"
)

func TestCLIExitCodesFollowADRContract(t *testing.T) {
	cli := filepath.Join(t.TempDir(), "harnessforge.exe")
	build := exec.Command("go", "build", "-o", cli, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := generation.SyncForge(context.Background(), root, "apply"); err != nil {
		t.Fatal(err)
	}
	failingRoot := t.TempDir()
	failingManifest := filepath.Join(failingRoot, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(failingManifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(failingManifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "successful check", args: []string{"check", "--repository", root}, want: 0},
		{name: "check failure", args: []string{"check", "--repository", failingRoot}, want: 1},
		{name: "configuration error", args: []string{"validate", "--repository", root, "--layout", "invalid"}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(cli, tc.args...)
			err := cmd.Run()
			if tc.want == 0 {
				if err != nil {
					t.Fatalf("successful command returned %v", err)
				}
				return
			}
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != tc.want {
				t.Fatalf("exit error=%v, want code %d", err, tc.want)
			}
		})
	}
}
