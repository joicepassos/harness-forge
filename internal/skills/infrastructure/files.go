package infrastructure

import (
	"context"
	"fmt"
	"harnessforge/internal/skills/application"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type RepositoryFiles struct{}

func (RepositoryFiles) Files(ctx context.Context, repository string) ([]application.File, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository must be a directory")
	}
	var result []application.File
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" || entry.Name() == "build" || entry.Name() == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || len(result) >= 500 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unsafe repository path")
		}
		if !strings.HasSuffix(rel, ".java") && !strings.HasSuffix(rel, ".sql") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 128*1024))
		if err != nil {
			return nil
		}
		result = append(result, application.File{Path: filepath.ToSlash(rel), Text: string(data)})
		return nil
	})
	return result, err
}
