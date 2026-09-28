package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"harnessforge/internal/contextpack/application"
	"harnessforge/internal/contextpack/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"harnessforge/internal/inputlimits"

	"go.yaml.in/yaml/v3"
)

// forgeKnowledgeCandidates loads only explicitly referenced, approved Forge
// knowledge. It opens manifest and evidence files directly beneath the project
// root; it does not perform another repository scan or execute project code.
func forgeKnowledgeCandidates(root, selection, prompt string, taskPaths []string) ([]domain.Excerpt, error) {
	forgeConfig := filepath.Join(root, ".forge", "forge.yaml")
	harnessConfig := filepath.Join(root, ".harness", "harness.yaml")
	forgeExists, err := regularLayoutConfig(forgeConfig)
	if err != nil {
		return nil, err
	}
	harnessExists, err := regularLayoutConfig(harnessConfig)
	if err != nil {
		return nil, err
	}
	if !forgeExists && !harnessExists {
		if selection == "forge" {
			return nil, fmt.Errorf("selected forge layout is missing %s", forgeConfig)
		}
		if selection != "" && selection != "harness" {
			return nil, fmt.Errorf("unknown layout %q; expected harness or forge", selection)
		}
		return nil, nil
	}
	layout, err := harnessinfra.ResolveLayout(root, selection)
	if err != nil {
		return nil, err
	}
	if layout.Kind != harnessinfra.LayoutForge {
		return nil, nil
	}
	manifest, err := (harnessinfra.ManifestLoader{}).Load(layout.ManifestPath)
	if err != nil {
		return nil, err
	}
	if len(manifest.References.Knowledge) == 0 {
		return nil, nil
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer rootHandle.Close()
	var candidates []domain.Excerpt
	for _, reference := range manifest.References.Knowledge {
		data, err := readSafeProjectFile(rootHandle, reference.Path, inputlimits.HarnessYAMLBytes)
		if err != nil {
			return nil, fmt.Errorf("knowledge %q: %w", reference.ID, err)
		}
		item, err := parseKnowledgeDocument(data)
		if err != nil {
			return nil, fmt.Errorf("knowledge %q: %w", reference.ID, err)
		}
		if item.ID != reference.ID {
			return nil, fmt.Errorf("knowledge reference %q points to item %q", reference.ID, item.ID)
		}
		// Candidates, rejected items, and deprecated items are never context.
		if item.Review != harnessdomain.KnowledgeApproved {
			continue
		}
		if got := harnessdomain.HashKnowledgeContent(item.Content); !strings.EqualFold(got, item.ContentSHA256) {
			return nil, fmt.Errorf("knowledge %q content hash mismatch; review is no longer valid", item.ID)
		}
		if got := harnessdomain.HashKnowledgeReviewMetadata(item); !strings.EqualFold(got, item.ReviewMetadataSHA256) {
			return nil, fmt.Errorf("knowledge %q review metadata hash mismatch; review is no longer valid", item.ID)
		}
		if item.Health == harnessdomain.KnowledgeStale || item.Health == harnessdomain.KnowledgeMissing {
			reason := "approved knowledge health is stale"
			if item.Health == harnessdomain.KnowledgeMissing {
				reason = "approved knowledge health is missing"
			}
			candidates = append(candidates, excludedKnowledge(item, reference.Path, prompt, reason))
			continue
		}
		if !knowledgePathApplies(item.Scope.Paths, taskPaths) {
			origins := []string{"forge-knowledge:" + item.ID, "forge-document:" + reference.Path}
			candidates = append(candidates, domain.Excerpt{
				ID: application.StableID("forge-knowledge", item.ID), Source: "forge-knowledge:" + item.ID,
				Path: reference.Path, Text: item.Content, Relevance: application.Relevance(prompt, strings.Join(item.Scope.Paths, " "), item.Content),
				EstimatedTokens: application.EstimateTokens(item.Content), Status: "excluded",
				Reason: "knowledge scope does not match the task paths", Origins: origins,
				KnowledgeID: item.ID, KnowledgeScope: append([]string(nil), item.Scope.Paths...),
			})
			continue
		}
		if reason := knowledgeKeywordExclusionReason(item.Keywords, prompt); reason != "" {
			candidates = append(candidates, excludedKnowledge(item, reference.Path, prompt, reason))
			continue
		}
		evidenceHash, err := harnessinfra.KnowledgeFingerprint(root, reference.Path, reference.ID, item.Evidence)
		if err != nil {
			return nil, fmt.Errorf("knowledge %q evidence: %w", item.ID, err)
		}
		if !strings.EqualFold(evidenceHash, item.EvidenceSHA256) {
			return nil, fmt.Errorf("knowledge %q evidence changed after review", item.ID)
		}
		origins := []string{"forge-knowledge:" + item.ID, "forge-document:" + reference.Path}
		for _, evidence := range item.Evidence {
			origins = append(origins, "evidence:"+evidence.Path)
		}
		scope := append([]string(nil), item.Scope.Paths...)
		text := item.Content
		candidates = append(candidates, domain.Excerpt{
			ID:              application.StableID("forge-knowledge", item.ID),
			Source:          "forge-knowledge:" + item.ID,
			Path:            reference.Path,
			Text:            text,
			Relevance:       application.Relevance(prompt, strings.Join(append(append([]string(nil), scope...), item.Keywords...), " "), text),
			EstimatedTokens: application.EstimateTokens(text),
			Origins:         origins,
			KnowledgeID:     item.ID,
			KnowledgeScope:  scope,
		})
	}
	return candidates, nil
}

func excludedKnowledge(item harnessdomain.KnowledgeItem, documentPath, prompt, reason string) domain.Excerpt {
	origins := []string{"forge-knowledge:" + item.ID, "forge-document:" + documentPath}
	return domain.Excerpt{
		ID: application.StableID("forge-knowledge", item.ID), Source: "forge-knowledge:" + item.ID,
		Path: documentPath, Text: item.Content,
		Relevance:       application.Relevance(prompt, strings.Join(item.Scope.Paths, " "), item.Content),
		EstimatedTokens: application.EstimateTokens(item.Content), Status: "excluded", Reason: reason,
		Origins: origins, KnowledgeID: item.ID, KnowledgeScope: append([]string(nil), item.Scope.Paths...),
	}
}

func knowledgeKeywordExclusionReason(keywords []string, prompt string) string {
	if len(keywords) == 0 || knowledgeKeywordMatches(keywords, prompt) {
		return ""
	}
	if len(application.Terms(prompt)) == 0 {
		return "task prompt contains no relevant keywords for approved knowledge"
	}
	return "approved knowledge keywords do not match the task prompt"
}

func knowledgeKeywordMatches(keywords []string, prompt string) bool {
	lowerPrompt := strings.ToLower(prompt)
	for _, keyword := range keywords {
		if strings.Contains(lowerPrompt, strings.ToLower(strings.TrimSpace(keyword))) {
			return true
		}
	}
	return false
}

// knowledgeApplies treats scope paths and explicit keywords as independent
// constraints. Unscoped knowledge remains generally available; scoped
// knowledge requires at least one matching task path. When keywords are
// present, at least one must occur in the task prompt (case-insensitively).
func knowledgeApplies(scopes, keywords, taskPaths []string, prompt string) bool {
	if !knowledgePathApplies(scopes, taskPaths) {
		return false
	}
	return len(keywords) == 0 || knowledgeKeywordMatches(keywords, prompt)
}

func knowledgePathApplies(scopes, taskPaths []string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, taskPath := range taskPaths {
		for _, scope := range scopes {
			if knowledgePathMatch(scope, taskPath) {
				return true
			}
		}
	}
	return false
}

// knowledgePathMatch uses repository-relative slash paths and supports ** as
// a whole path segment matching zero or more directories.
func knowledgePathMatch(pattern, candidate string) bool {
	pattern = strings.TrimPrefix(strings.ReplaceAll(pattern, "\\", "/"), "./")
	candidate = strings.TrimPrefix(strings.ReplaceAll(candidate, "\\", "/"), "./")
	return matchKnowledgeSegments(strings.Split(pattern, "/"), strings.Split(candidate, "/"))
}

func matchKnowledgeSegments(pattern, candidate []string) bool {
	if len(pattern) == 0 {
		return len(candidate) == 0
	}
	if pattern[0] == "**" {
		if matchKnowledgeSegments(pattern[1:], candidate) {
			return true
		}
		return len(candidate) > 0 && matchKnowledgeSegments(pattern, candidate[1:])
	}
	if len(candidate) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], candidate[0])
	return err == nil && matched && matchKnowledgeSegments(pattern[1:], candidate[1:])
}

func regularLayoutConfig(path string) (bool, error) {
	parent, err := os.Lstat(filepath.Dir(path))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if parent.Mode()&os.ModeSymlink != 0 || !parent.IsDir() {
		return false, fmt.Errorf("layout directory must be a regular non-symlink directory: %s", filepath.Dir(path))
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("layout configuration must be a regular non-symlink file: %s", path)
	}
	return true, nil
}

func parseKnowledgeDocument(data []byte) (harnessdomain.KnowledgeItem, error) {
	var item harnessdomain.KnowledgeItem
	frontMatter, err := knowledgeFrontMatter(data)
	if err != nil {
		return item, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(frontMatter))
	decoder.KnownFields(true)
	if err := decoder.Decode(&item); err != nil {
		return item, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return item, fmt.Errorf("expected one YAML document")
	}
	if err := validateKnowledgeItem(item); err != nil {
		return item, err
	}
	return item, nil
}

// knowledgeFrontMatter recognizes delimiters only when they occupy a whole
// line. YAML values such as review_diff may contain text beginning with "---".
func knowledgeFrontMatter(data []byte) (string, error) {
	text := strings.TrimSpace(string(data))
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\n")) != "---" {
		return "", fmt.Errorf("expected YAML front matter")
	}
	offset := len(lines[0])
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(strings.TrimSuffix(line, "\n")) == "---" {
			return strings.TrimSpace(text[len(lines[0]):offset]), nil
		}
		offset += len(line)
	}
	return "", fmt.Errorf("unterminated YAML front matter")
}

func validateKnowledgeItem(item harnessdomain.KnowledgeItem) error {
	if item.Review != harnessdomain.KnowledgeCandidate {
		if item.ContentSHA256 == "" {
			return fmt.Errorf("content_sha256: required after candidate review")
		}
		if item.EvidenceSHA256 == "" {
			return fmt.Errorf("evidence_sha256: required after candidate review")
		}
	}
	return item.Validate()
}

func validateKnowledgeEvidence(root *os.Root, evidence []harnessdomain.KnowledgeEvidence) (string, error) {
	values := make([]string, 0, len(evidence))
	for index, item := range evidence {
		data, err := readSafeProjectFile(root, item.Path, inputlimits.HistoricalEvidenceBytes)
		if err != nil {
			return "", fmt.Errorf("evidence[%d] %q: %w", index, item.Path, err)
		}
		if item.SHA256 != "" {
			sum := sha256.Sum256(data)
			if !strings.EqualFold(hex.EncodeToString(sum[:]), item.SHA256) {
				return "", fmt.Errorf("evidence[%d] %q hash mismatch", index, item.Path)
			}
		}
		if item.Quote != "" && !bytes.Contains(data, []byte(item.Quote)) {
			return "", fmt.Errorf("evidence[%d] %q quote no longer matches", index, item.Path)
		}
		if item.Symbol != "" && !bytes.Contains(data, []byte(item.Symbol)) {
			return "", fmt.Errorf("evidence[%d] %q symbol no longer matches", index, item.Path)
		}
		sum := sha256.Sum256(data)
		values = append(values, item.Path+":"+hex.EncodeToString(sum[:]))
	}
	return harnessdomain.HashKnowledgeEvidence(values), nil
}

func readSafeProjectFile(root *os.Root, relative string, maxBytes int64) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") || strings.Contains(relative, ":") {
		return nil, fmt.Errorf("expected a repository-relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("path escapes the project root")
	}
	// Reject symlinks at every component before opening through os.Root, which
	// independently prevents path traversal outside the root even under races.
	current := "."
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for _, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := root.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink path component is not allowed")
		}
		if current != clean && !info.IsDir() {
			return nil, fmt.Errorf("parent path component is not a directory")
		}
		if current == clean && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("expected a regular non-symlink file")
		}
	}
	file, err := root.Open(clean)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxBytes {
		return nil, fmt.Errorf("file must be regular and no larger than %d bytes", maxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxBytes)
	}
	return data, nil
}
