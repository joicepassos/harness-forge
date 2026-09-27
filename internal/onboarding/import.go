package onboarding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

const maxImportBytes = 1 << 20

// ImportKnowledgeFile stores a selected repository text document as an
// unapproved Forge candidate. It neither interprets instructions nor executes
// imported content.
func ImportKnowledgeFile(repository, sourcePath string, kind harnessdomain.KnowledgeKind) (id, targetPath string, err error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(sourcePath) == "" || filepath.IsAbs(sourcePath) || strings.ContainsAny(sourcePath, ":\\") {
		return "", "", fmt.Errorf("source must be a repository-relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(sourcePath))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("source path escapes the repository")
	}
	relative := filepath.ToSlash(clean)
	if relative == ".forge/forge.yaml" || relative == ".forge/generated-manifest.json" || strings.HasPrefix(relative, ".forge/knowledge/") || relative == ".forge/knowledge" {
		return "", "", fmt.Errorf("Forge configuration and knowledge outputs cannot be imported as source documents")
	}

	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return "", "", err
	}
	defer rootHandle.Close()
	current := "."
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, statErr := rootHandle.Lstat(current)
		if statErr != nil {
			return "", "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("source path contains a symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", "", fmt.Errorf("source parent is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", "", fmt.Errorf("source must be a regular text file")
		}
	}
	file, err := rootHandle.Open(clean)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", "", err
	}
	if info.Size() > maxImportBytes {
		return "", "", fmt.Errorf("source exceeds the 1 MiB import limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil {
		return "", "", err
	}
	if len(data) > maxImportBytes {
		return "", "", fmt.Errorf("source exceeds the 1 MiB import limit")
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return "", "", fmt.Errorf("source must be UTF-8 text")
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", "", fmt.Errorf("source document is empty")
	}
	pathHash := sha256.Sum256([]byte(relative))
	base := strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean))
	var slug strings.Builder
	for _, r := range strings.ToLower(base) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			slug.WriteRune(r)
		} else if slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-") {
			slug.WriteByte('-')
		}
		if slug.Len() >= 32 {
			break
		}
	}
	name := strings.Trim(slug.String(), "-")
	if name == "" {
		name = "document"
	}
	id = "import-" + name + "-" + hex.EncodeToString(pathHash[:6])
	contentHash := sha256.Sum256(data)
	item := harnessdomain.KnowledgeItem{
		ID:            id,
		Kind:          kind,
		Content:       content,
		Origin:        "repository-file:" + relative,
		Review:        harnessdomain.KnowledgeCandidate,
		Health:        harnessdomain.KnowledgeUnknown,
		ContentSHA256: harnessdomain.HashKnowledgeContent(content),
		Evidence: []harnessdomain.KnowledgeEvidence{{
			Path:   relative,
			SHA256: hex.EncodeToString(contentHash[:]),
		}},
	}
	targetPath, err = harnessinfra.ImportKnowledgeCandidate(root, item)
	if err != nil {
		return "", "", err
	}
	return id, targetPath, nil
}
