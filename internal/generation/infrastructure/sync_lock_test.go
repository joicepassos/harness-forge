package infrastructure

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSyncFileLockWaitsAndRespectsCancellation(t *testing.T) {
	root := forgeSyncFixture(t)
	first, err := acquireSyncFileLock(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := acquireSyncFileLock(ctx, root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second lock acquisition error = %v, want deadline exceeded", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := acquireSyncFileLock(context.Background(), root)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSyncFileLockSerializesAcrossProcesses(t *testing.T) {
	if root := os.Getenv("HARNESSFORGE_SYNC_LOCK_HELPER_ROOT"); root != "" {
		lock, err := acquireSyncFileLock(context.Background(), root)
		if err != nil {
			fmt.Fprintln(os.Stdout, "lock error:", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "locked")
		_, err = bufio.NewReader(os.Stdin).ReadString('\n')
		if closeErr := lock.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			fmt.Fprintln(os.Stdout, "release error:", err)
			os.Exit(3)
		}
		return
	}

	root := forgeSyncFixture(t)
	command := exec.Command(os.Args[0], "-test.run=^TestSyncFileLockSerializesAcrossProcesses$")
	command.Env = append(os.Environ(), "HARNESSFORGE_SYNC_LOCK_HELPER_ROOT="+root)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()

	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			ready <- scanner.Text()
			return
		}
		if err := scanner.Err(); err != nil {
			ready <- "child output error: " + err.Error()
		} else {
			ready <- "child exited before acquiring the lock"
		}
	}()
	select {
	case line := <-ready:
		if line != "locked" {
			t.Fatalf("child lock status = %q, want locked", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the child process to acquire the lock")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if second, err := acquireSyncFileLock(ctx, root); !errors.Is(err, context.DeadlineExceeded) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("second process lock acquisition error = %v, want deadline exceeded", err)
	}
	if _, err := stdin.Write([]byte("release\n")); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		finished = true
		if err != nil {
			t.Fatalf("child process failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for child process to release the lock")
	}

	third, err := acquireSyncFileLock(context.Background(), root)
	if err != nil {
		t.Fatalf("lock was not released by the child process: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}
