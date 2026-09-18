package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"harnessforge/internal/indexing/domain"
	"harnessforge/internal/inputlimits"
	"math"
	"path"
	"strings"
)

func validateIndex(index domain.Index) error {
	if index.Version != "index-v1" {
		return fmt.Errorf("unsupported version")
	}
	if index.Model == "" {
		return fmt.Errorf("missing model")
	}
	if len(index.Chunks) == 0 {
		if index.Dimensions != 0 {
			return fmt.Errorf("empty index has dimensions")
		}
		return nil
	}
	if index.Dimensions <= 0 || len(index.Chunks) > inputlimits.IndexChunks {
		return fmt.Errorf("invalid dimensions or chunk count")
	}
	sources, ids := map[string]string{}, map[string]bool{}
	for n, chunk := range index.Chunks {
		if ids[chunk.ID] || !isHash(chunk.ID) || !isHash(chunk.Hash) || !isHash(chunk.SourceHash) {
			return fmt.Errorf("chunk %d has invalid identifiers", n)
		}
		ids[chunk.ID] = true
		if !safeSource(chunk.Source) {
			return fmt.Errorf("chunk %d has unsafe source", n)
		}
		if prior, ok := sources[chunk.Source]; ok && prior != chunk.SourceHash {
			return fmt.Errorf("chunk %d has inconsistent source hash", n)
		}
		sources[chunk.Source] = chunk.SourceHash
		if chunk.Model != index.Model || chunk.StartLine < 1 || chunk.EndLine < chunk.StartLine || int64(len(chunk.Text)) > inputlimits.ChunkTextBytes || chunk.Text == "" {
			return fmt.Errorf("chunk %d has inconsistent metadata", n)
		}
		sum := sha256.Sum256([]byte(chunk.Text))
		if hex.EncodeToString(sum[:]) != chunk.Hash {
			return fmt.Errorf("chunk %d text hash mismatch", n)
		}
		if len(chunk.Vector) != index.Dimensions {
			return fmt.Errorf("chunk %d vector dimensions mismatch", n)
		}
		for _, value := range chunk.Vector {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("chunk %d vector is non-finite", n)
			}
		}
	}
	return nil
}

func isHash(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && len(value) == 64
}
func safeSource(value string) bool {
	return value != "" && !strings.Contains(value, "\\") && !strings.Contains(value, "\x00") && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../")
}
