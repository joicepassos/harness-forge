package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"harnessforge/internal/indexing/domain"
	"math"
	"os"
	"path/filepath"
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
			if path != root && (entry.Name() == ".git" || entry.Name() == ".harness" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		if count > 10000 {
			return fmt.Errorf("document scan exceeds 10000 files")
		}
		name := strings.ToLower(entry.Name())
		if !entry.Type().IsRegular() || !(strings.HasSuffix(name, ".md") || name == "readme") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 1<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		result = append(result, domain.Document{Path: filepath.ToSlash(rel), Text: string(data), Hash: hash(data)})
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

type JSONStore struct{}

func (JSONStore) Load(root string) (domain.Index, error) {
	path := filepath.Join(root, ".harness", "index.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return domain.Index{}, nil
	}
	if err != nil {
		return domain.Index{}, err
	}
	var index domain.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	return index, nil
}
func (JSONStore) Save(root string, index domain.Index) error {
	dir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
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
	return os.Rename(name, filepath.Join(dir, "index.json"))
}
func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
