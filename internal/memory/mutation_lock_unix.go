//go:build !windows

package memory

import (
	"errors"
	"os"
	"syscall"
)

func tryLockMutationFile(file *os.File) (func() error, bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return func() error { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }, true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return nil, false, nil
	}
	return nil, false, err
}
