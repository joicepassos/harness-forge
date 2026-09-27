package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"io"
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
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()
	for i, rule := range h.Rules {
		for j, e := range rule.Evidence {
			field := fmt.Sprintf("rules[%d].evidence[%d]", i, j)
			if e.File == "" || filepath.IsAbs(e.File) || strings.Contains(e.File, "\\") || strings.Contains(e.File, ":") {
				return fmt.Errorf("%s.file: expected repository-relative path", field)
			}
			rel := filepath.ToSlash(filepath.Clean(e.File))
			if rel == ".." || strings.HasPrefix(rel, "../") {
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
				file, err := rootHandle.Open(rel)
				if err != nil {
					return fmt.Errorf("%s.file: %w", field, err)
				}
				info, err := file.Stat()
				if err != nil {
					file.Close()
					return fmt.Errorf("%s.file: %w", field, err)
				}
				if !info.Mode().IsRegular() || info.Size() > inputlimits.HistoricalEvidenceBytes {
					file.Close()
					return fmt.Errorf("%s.file: expected regular file up to 4 MiB", field)
				}
				data, err = io.ReadAll(io.LimitReader(file, inputlimits.HistoricalEvidenceBytes+1))
				file.Close()
				if err != nil {
					return err
				}
				if int64(len(data)) > inputlimits.HistoricalEvidenceBytes {
					return fmt.Errorf("%s.file: expected regular file up to 4 MiB", field)
				}
				actual := sha256.Sum256(data)
				actualHash := hex.EncodeToString(actual[:])
				if e.SHA256 != "" && !strings.EqualFold(actualHash, e.SHA256) {
					return fmt.Errorf("%s.sha256: content hash mismatch", field)
				}
			}
			if e.Symbol != "" && !strings.Contains(string(data), e.Symbol) {
				return fmt.Errorf("%s.symbol: literal not found", field)
			}
			if e.Quote != "" && !strings.Contains(string(data), e.Quote) {
				return fmt.Errorf("%s.quote: literal not found", field)
			}
			if e.SHA256 != "" && e.Revision != "" {
				sum := sha256.Sum256(data)
				actual := hex.EncodeToString(sum[:])
				if !strings.EqualFold(actual, e.SHA256) {
					return fmt.Errorf("%s.sha256: content hash mismatch", field)
				}
			}
		}
	}
	return ctx.Err()
}

// EvidenceRevalidator computes a fingerprint from the current bytes of all evidence files.
type EvidenceRevalidator struct{ Root string }

func (r EvidenceRevalidator) Fingerprint(path, id string, evidence []domain.Evidence) (string, error) {
	root := r.Root
	if root == "" {
		root = filepath.Dir(path)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer rootHandle.Close()
	values := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if item.File == "" || filepath.IsAbs(item.File) || strings.Contains(item.File, "\\") || strings.Contains(item.File, ":") {
			return "", fmt.Errorf("evidence file must be repository-relative")
		}
		rel := filepath.ToSlash(filepath.Clean(item.File))
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return "", fmt.Errorf("evidence file outside repository")
		}
		var data []byte
		if item.Revision != "" {
			resolve := exec.Command("git", "rev-parse", "--verify", "--end-of-options", item.Revision+"^{commit}")
			resolve.Dir = root
			revision, err := resolve.Output()
			if err != nil {
				return "", err
			}
			show := exec.Command("git", "show", strings.TrimSpace(string(revision))+":"+rel)
			show.Dir = root
			var output boundedOutput
			show.Stdout = &output
			if err := show.Run(); err != nil {
				return "", err
			}
			if output.exceeded {
				return "", fmt.Errorf("evidence file too large")
			}
			data = output.Bytes()
		} else {
			file, err := rootHandle.Open(rel)
			if err != nil {
				return "", err
			}
			info, err := file.Stat()
			if err != nil {
				file.Close()
				return "", err
			}
			if !info.Mode().IsRegular() || info.Size() > inputlimits.HistoricalEvidenceBytes {
				file.Close()
				return "", fmt.Errorf("evidence file must be a regular file up to 4 MiB")
			}
			data, err = io.ReadAll(io.LimitReader(file, inputlimits.HistoricalEvidenceBytes+1))
			file.Close()
			if err != nil {
				return "", err
			}
			if int64(len(data)) > inputlimits.HistoricalEvidenceBytes {
				return "", fmt.Errorf("evidence file too large")
			}
		}
		if item.Symbol != "" && !strings.Contains(string(data), item.Symbol) {
			return "", fmt.Errorf("symbol %q not found in %s", item.Symbol, item.File)
		}
		if item.Quote != "" && !strings.Contains(string(data), item.Quote) {
			return "", fmt.Errorf("quote not found in %s", item.File)
		}
		fileHash := sha256.Sum256(data)
		if item.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(fileHash[:]), item.SHA256) {
			return "", fmt.Errorf("evidence hash mismatch for %s", item.File)
		}
		values = append(values, item.File+":"+hex.EncodeToString(fileHash[:]))
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
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
