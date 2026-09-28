package infrastructure

import (
	"context"
	"errors"
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
