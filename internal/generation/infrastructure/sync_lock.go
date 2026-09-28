package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type syncFileLock struct {
	file   *os.File
	unlock func() error
}

func acquireSyncFileLock(ctx context.Context, root string) (*syncFileLock, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	canonicalRoot = filepath.Clean(canonicalRoot)
	if runtime.GOOS == "windows" {
		canonicalRoot = strings.ToLower(canonicalRoot)
	}
	rootHash := sha256.Sum256([]byte(canonicalRoot))
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	lockDir := filepath.Join(cache, "harnessforge", "sync-locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(lockDir, hex.EncodeToString(rootHash[:])+".lock")
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("sync lock must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		unlock, acquired, err := tryLockSyncFile(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if acquired {
			return &syncFileLock{file: file, unlock: unlock}, nil
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (lock *syncFileLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	unlockErr := lock.unlock()
	closeErr := lock.file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
