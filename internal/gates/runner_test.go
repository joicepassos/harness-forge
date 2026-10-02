package gates

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestRunCancellationAndTimeoutStopGateDescendants(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a helper process")
	}
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "survived")
			started := filepath.Join(root, "started")
			command := descendantCommand(t, root, started, marker)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := 10 * time.Second
			if mode == "timeout" {
				timeout = 500 * time.Millisecond
				if runtime.GOOS == "windows" {
					timeout = 3 * time.Second
				}
			}
			type outcome struct {
				results []Result
				err     error
			}
			done := make(chan outcome, 1)
			go func() {
				results, runErr := Run(ctx, root, []Gate{{ID: "tree", Command: command}}, timeout)
				done <- outcome{results: results, err: runErr}
			}()
			if mode == "cancel" {
				// Ensure the descendant has started before canceling its shell.
				waitForFile(t, started, 8*time.Second)
				cancel()
			} else {
				waitForFile(t, started, 8*time.Second)
			}
			select {
			case result := <-done:
				if mode == "cancel" && !errors.Is(result.err, context.Canceled) {
					t.Fatalf("cancellation error = %v, want context.Canceled", result.err)
				}
				if mode == "timeout" && (result.err != nil || len(result.results) != 1 || !result.results[0].TimedOut) {
					t.Fatalf("timeout result = %#v, err=%v", result.results, result.err)
				}
			case <-time.After(12 * time.Second):
				t.Fatal("gate did not stop after cancellation/timeout")
			}
			// The child waits ten seconds before writing this marker.
			time.Sleep(1200 * time.Millisecond)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("descendant survived process-tree termination (marker stat error: %v)", err)
			}
		})
	}
}

func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func descendantCommand(t *testing.T, root, started, marker string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		// A batch file avoids cmd.exe /C's nested-quote parsing differences.
		batch := "@echo off\r\necho started>" + started + "\r\npowershell.exe -NoProfile -NonInteractive -Command \"Start-Sleep -Seconds 10\"\r\necho alive>" + marker + "\r\npowershell.exe -NoProfile -NonInteractive -Command \"Start-Sleep -Seconds 30\"\r\n"
		path := filepath.Join(root, "gate-tree.cmd")
		if err := os.WriteFile(path, []byte(batch), 0600); err != nil {
			t.Fatal(err)
		}
		return "call " + path
	}
	return "(echo started > " + shellQuote(started) + "; sleep 4; echo alive > " + shellQuote(marker) + ") & wait"
}

func shellQuote(value string) string { return "'" + value + "'" }

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

func TestRunInheritsProcessEnvironmentAndAppliesGateOverrides(t *testing.T) {
	t.Setenv("HARNESSFORGE_GATE_PARENT", "inherited")
	t.Setenv("HARNESSFORGE_GATE_OVERRIDE", "parent")
	command := `printf '%s|%s' "$HARNESSFORGE_GATE_PARENT" "$HARNESSFORGE_GATE_OVERRIDE"`
	if runtime.GOOS == "windows" {
		command = `echo %HARNESSFORGE_GATE_PARENT%^|%HARNESSFORGE_GATE_OVERRIDE%`
	}
	results, err := Run(context.Background(), t.TempDir(), []Gate{{ID: "env", Command: command, Env: map[string]string{"HARNESSFORGE_GATE_OVERRIDE": "gate"}}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "passed" {
		t.Fatalf("results=%#v", results)
	}
	if !strings.Contains(results[0].Output, "inherited") || !strings.Contains(results[0].Output, "gate") {
		t.Fatalf("environment not inherited/overridden: %#v", results[0])
	}
}

func TestRunRejectsInvalidEnvironment(t *testing.T) {
	for key, value := range map[string]string{"BAD-NAME": "x", "9BAD": "x", "GOOD": "bad\x00value"} {
		if _, err := Run(context.Background(), t.TempDir(), []Gate{{ID: "invalid", Command: "exit 0", Env: map[string]string{key: value}}}, time.Second); err == nil {
			t.Errorf("accepted invalid env %q=%q", key, value)
		}
	}
}
