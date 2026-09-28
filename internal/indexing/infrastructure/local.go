package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"harnessforge/internal/indexing/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/internal/safefile"
	"harnessforge/internal/securityboundary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

type Documents struct{}

func (Documents) Documents(ctx context.Context, root string) ([]domain.Document, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	ignored, err := securityboundary.LoadGitIgnoreContext(ctx, root, inputlimits.RepositoryFiles)
	if err != nil {
		return nil, err
	}
	var result []domain.Document
	count := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if entry.IsDir() {
			if path != root && securityboundary.SkipRepositoryDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		if count > inputlimits.RepositoryFiles {
			return fmt.Errorf("document scan exceeds %d files", inputlimits.RepositoryFiles)
		}
		name := strings.ToLower(entry.Name())
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if ignored.Match(rel) {
			return nil
		}
		if securityboundary.SensitivePath(rel) {
			return nil
		}
		if !entry.Type().IsRegular() || !(strings.HasSuffix(name, ".md") || name == "readme") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > inputlimits.SourceFileBytes {
			return nil
		}
		data, err := inputlimits.ReadFile(path, inputlimits.SourceFileBytes, "source file")
		if err != nil {
			return err
		}
		if securityboundary.ContainsSensitiveContent(data) {
			return nil
		}
		result = append(result, domain.Document{Path: rel, Text: string(data), Hash: hash(data)})
		return nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, err
}

type Markdown struct{}

func (Markdown) Chunks(document domain.Document) []domain.Chunk {
	lines := strings.Split(document.Text, "\n")
	start := 0
	var out []domain.Chunk
	flush := func(end int) {
		text := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
		if text == "" {
			start = end
			return
		}
		data := []byte(text)
		id := hash([]byte(document.Path + ":" + fmt.Sprint(start+1) + ":" + document.Hash))
		out = append(out, domain.Chunk{ID: id, Source: document.Path, SourceHash: document.Hash, Text: text, Hash: hash(data), StartLine: start + 1, EndLine: end})
		start = end
	}
	for i, line := range lines {
		if i > start && strings.HasPrefix(strings.TrimSpace(line), "#") {
			flush(i)
		} else if i-start >= 80 {
			flush(i)
		}
	}
	flush(len(lines))
	return out
}

type Lexical struct{}

func (Lexical) Embed(ctx context.Context, text string) ([]float64, int, error) {
	vector := make([]float64, 64)
	tokens := 0
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		sum := sha256.Sum256([]byte(word))
		vector[int(sum[0])%len(vector)]++
		tokens++
	}
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	if norm == 0 {
		return nil, 0, fmt.Errorf("empty chunk")
	}
	norm = math.Sqrt(norm)
	for i := range vector {
		vector[i] /= norm
	}
	return vector, tokens, nil
}

// JSONStore stores generated index data in the user's OS cache, keyed by the
// canonical repository/worktree path. CacheDir is an optional cache root used
// by tests and embedders; an empty value uses os.UserCacheDir.
type JSONStore struct{ CacheDir string }

func (store JSONStore) Load(root string) (domain.Index, error) {
	path, err := store.indexPath(root)
	if err != nil {
		return domain.Index{}, err
	}
	data, err := inputlimits.ReadFile(path, inputlimits.PersistedIndexBytes, "persisted index")
	if os.IsNotExist(err) {
		// Read older installations without mutating or deleting the repository
		// file. The next successful index build writes the new cache location.
		data, err = store.loadLegacy(root)
		if err == nil && data == nil {
			return domain.Index{}, nil
		}
	}
	if err != nil {
		return domain.Index{}, err
	}
	var index domain.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return index, fmt.Errorf("decode persisted index: %w", err)
	}
	if err := validateIndex(index); err != nil {
		return domain.Index{}, fmt.Errorf("invalid persisted index: %w", err)
	}
	return index, nil
}

func (JSONStore) loadLegacy(root string) ([]byte, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(absolute, ".harness")
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("legacy index directory cannot be a symlink or non-directory")
	}
	legacy := filepath.Join(dir, "index.json")
	info, err = os.Lstat(legacy)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("legacy index must be a regular file")
	}
	data, err := inputlimits.ReadFile(legacy, inputlimits.PersistedIndexBytes, "legacy persisted index")
	if err == nil && data == nil {
		data = []byte{}
	}
	return data, err
}

func (store JSONStore) Save(root string, index domain.Index) error {
	path, err := store.indexPath(root)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create index cache: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, "index-*.json")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Close()
	}
	if err != nil {
		return err
	}
	return safefile.Replace(name, path)
}

func (store JSONStore) indexPath(root string) (string, error) {
	identity, err := repositoryIdentity(root)
	if err != nil {
		return "", err
	}
	cacheRoot := store.CacheDir
	if cacheRoot == "" {
		cacheRoot, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("locate user cache: %w", err)
		}
	}
	return filepath.Join(cacheRoot, "harnessforge", "indexes", identity+".json"), nil
}

func repositoryIdentity(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve repository path: %w", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", err
	}
	canonical = filepath.Clean(canonical)
	if runtime.GOOS == "windows" {
		canonical = strings.ToLower(canonical)
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:]), nil
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
