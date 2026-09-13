package infrastructure

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileReader struct{ root string }

func NewFileReader(root string) (*FileReader, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	return &FileReader{root: resolved}, nil
}

func (r *FileReader) Contains(ctx context.Context, path, symbol string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if path == "" || symbol == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") || strings.Contains(path, ":") {
		return false, fmt.Errorf("evidence path must be repository-relative")
	}
	full := filepath.Join(r.root, filepath.FromSlash(path))
	rel, err := filepath.Rel(r.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("evidence path is outside repository")
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false, err
	}
	rel, err = filepath.Rel(r.root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("evidence path resolves outside repository")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return false, fmt.Errorf("evidence must be a regular file up to 4 MiB")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), symbol), ctx.Err()
}
