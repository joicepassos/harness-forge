package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckEvidence checks file and literal symbol references, not architectural correctness.
func CheckEvidence(ctx context.Context, root string, h domain.Harness) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	for i, rule := range h.Rules {
		for j, e := range rule.Evidence {
			field := fmt.Sprintf("rules[%d].evidence[%d]", i, j)
			if e.File == "" || filepath.IsAbs(e.File) || strings.Contains(e.File, "\\") || strings.Contains(e.File, ":") {
				return fmt.Errorf("%s.file: expected repository-relative path", field)
			}
			path := filepath.Join(root, filepath.FromSlash(e.File))
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s.file: outside repository", field)
			}
			var data []byte
			if e.Revision != "" {
				resolve := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--end-of-options", e.Revision+"^{commit}")
				resolve.Dir = root
				revision, err := resolve.Output()
				if err != nil {
					return fmt.Errorf("%s.revision: %w", field, err)
				}
				show := exec.CommandContext(ctx, "git", "show", strings.TrimSpace(string(revision))+":"+filepath.ToSlash(rel))
				show.Dir = root
				var output boundedOutput
				show.Stdout = &output
				err = show.Run()
				if output.exceeded {
					return fmt.Errorf("%s.file: historical evidence exceeds %d bytes", field, inputlimits.HistoricalEvidenceBytes)
				}
				if err != nil {
					return fmt.Errorf("%s.file: %w", field, err)
				}
				data = output.Bytes()
			} else {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("%s.file: %w", field, err)
				}
				rel, err := filepath.Rel(root, resolved)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return fmt.Errorf("%s.file: symlink outside repository", field)
				}
				info, err := os.Stat(resolved)
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() || info.Size() > inputlimits.HistoricalEvidenceBytes {
					return fmt.Errorf("%s.file: expected regular file up to 4 MiB", field)
				}
				data, err = inputlimits.ReadFile(resolved, inputlimits.HistoricalEvidenceBytes, "evidence")
				if err != nil {
					return err
				}
			}
			if e.Symbol != "" && !strings.Contains(string(data), e.Symbol) {
				return fmt.Errorf("%s.symbol: literal not found", field)
			}
		}
	}
	return ctx.Err()
}

type boundedOutput struct {
	bytes.Buffer
	exceeded bool
}

func (b *boundedOutput) Write(data []byte) (int, error) {
	if int64(b.Len()+len(data)) > inputlimits.HistoricalEvidenceBytes {
		b.exceeded = true
		return 0, fmt.Errorf("historical evidence exceeds limit")
	}
	return b.Buffer.Write(data)
}
