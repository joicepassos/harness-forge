package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository must be a directory")
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
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
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
	file, err := os.Open(resolved)
	if err != nil {
		return false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil {
		return false, err
	}
	if len(data) > 4<<20 {
		return false, fmt.Errorf("evidence exceeds 4 MiB")
	}
	return strings.Contains(string(data), symbol), ctx.Err()
}

func (r *FileReader) ContainsAt(ctx context.Context, path, symbol, revision string) (bool, error) {
	if path == "" || symbol == "" || filepath.IsAbs(path) || strings.ContainsAny(path, "\\:\x00") {
		return false, fmt.Errorf("evidence path must be repository-relative")
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return false, fmt.Errorf("evidence path is outside repository")
		}
	}
	resolved, err := r.git(ctx, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return false, fmt.Errorf("resolve evidence revision: %w", err)
	}
	oid := strings.TrimSpace(string(resolved))
	if len(oid) != 40 && len(oid) != 64 {
		return false, fmt.Errorf("invalid resolved revision")
	}
	data, err := r.git(ctx, "show", oid+":"+path)
	if err != nil {
		return false, fmt.Errorf("read historical evidence: %w", err)
	}
	return strings.Contains(string(data), symbol), ctx.Err()
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 4<<20 {
		return 0, fmt.Errorf("historical evidence exceeds 4 MiB")
	}
	return b.Buffer.Write(data)
}

func (r *FileReader) git(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.root
	var output boundedOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
