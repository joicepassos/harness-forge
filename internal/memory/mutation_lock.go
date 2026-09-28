package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// mutationLock serializes read-modify-write operations between processes. List
// does not acquire it: saves replace the JSON snapshot atomically.
type mutationLock struct {
	file   *os.File
	unlock func() error
}

func (s *Store) lockMutation() (*mutationLock, error) {
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(s.root)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("memory store path must be a regular directory")
	}
	path := filepath.Join(s.root, "mutations.lock")
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("memory mutation lock must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		unlock, acquired, err := tryLockMutationFile(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if acquired {
			return &mutationLock{file: file, unlock: unlock}, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (lock *mutationLock) Close() error {
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
