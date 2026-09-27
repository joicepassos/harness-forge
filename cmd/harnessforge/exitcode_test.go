package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLIExitCodesFollowADRContract(t *testing.T) {
	if os.Getenv("HARNESSFORGE_EXITCODE_HELPER") == "1" {
		os.Exit(cliExitCode(runCLIForExitCode()))
	}
	root := t.TempDir()
	manifest := filepath.Join(root, ".forge", "forge.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "check failure", args: []string{"check", "--repository", root}, want: 1},
		{name: "configuration error", args: []string{"validate", "--repository", root, "--layout", "invalid"}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCLIExitCodesFollowADRContract$")
			cmd.Env = append(os.Environ(), "HARNESSFORGE_EXITCODE_HELPER=1", "HARNESSFORGE_EXITCODE_ARGS="+joinArgs(tc.args))
			err := cmd.Run()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != tc.want {
				t.Fatalf("exit error=%v, want code %d", err, tc.want)
			}
		})
	}
}

func runCLIForExitCode() error {
	cmd := newRootCommand()
	cmd.SetArgs(splitArgs(os.Getenv("HARNESSFORGE_EXITCODE_ARGS")))
	return cmd.Execute()
}

func joinArgs(args []string) string {
	data, _ := json.Marshal(args)
	return string(data)
}

func splitArgs(encoded string) []string {
	var args []string
	_ = json.Unmarshal([]byte(encoded), &args)
	return args
}
