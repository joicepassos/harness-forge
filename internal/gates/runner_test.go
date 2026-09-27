package gates

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRunExecutesOnlyRequestedGateInWorkspaceAndReportsExit(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	results, err := Run(context.Background(), root, []Gate{{ID: "ok", Command: "echo passed", Workspace: "services/api"}, {ID: "fail", Command: "exit 7", Workspace: "."}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Status != "passed" || results[0].Workspace != "services/api" || results[1].Status != "failed" || results[1].ExitCode != 7 {
		t.Fatalf("results=%#v", results)
	}
}

func TestRunBoundsTimeoutAndRejectsUnsafeWorkspace(t *testing.T) {
	root := t.TempDir()
	slow := "sleep 2"
	if runtime.GOOS == "windows" {
		slow = "ping -n 4 127.0.0.1 >NUL"
	}
	results, err := Run(context.Background(), root, []Gate{{ID: "timeout", Command: slow, Workspace: "."}, {ID: "escape", Command: "echo must-not-run", Workspace: "../outside"}}, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].TimedOut || results[0].Status != "failed" {
		t.Fatalf("timeout not reported: %#v", results[0])
	}
	if results[1].Status != "failed" || results[1].Error == "" {
		t.Fatalf("unsafe workspace accepted: %#v", results[1])
	}
}

func TestRunRejectsSymlinkWorkspace(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	results, err := Run(context.Background(), root, []Gate{{ID: "symlink", Command: "echo no", Workspace: "linked"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "failed" {
		t.Fatalf("symlink gate workspace accepted: %#v", results[0])
	}
}
