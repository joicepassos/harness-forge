//go:build !windows

// Package safefile provides small, cross-platform file replacement helpers.
package safefile

import "os"

// Replace publishes a closed temporary file at destination.
func Replace(temporary, destination string) error {
	return os.Rename(temporary, destination)
}
