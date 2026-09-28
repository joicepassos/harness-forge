package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go.yaml.in/yaml/v3"
	harnessdomain "harnessforge/internal/harness/domain"
	"harnessforge/internal/safefile"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// ImportKnowledgeCandidate adds explicit material as a candidate document.
// It preserves the Forge manifest's YAML nodes and never executes imported text.
func ImportKnowledgeCandidate(root string, item harnessdomain.KnowledgeItem) (string, error) {
	return importKnowledgeCandidate(root, item, false)
}

// ImportKnowledgeCandidateDeduplicatingEquivalent is used when publishing a
// reviewed local observation. It reuses an existing candidate/approved claim
// with identical semantic content while preserving its original audit record.
func ImportKnowledgeCandidateDeduplicatingEquivalent(root string, item harnessdomain.KnowledgeItem) (string, error) {
	return importKnowledgeCandidate(root, item, true)
}

func importKnowledgeCandidate(root string, item harnessdomain.KnowledgeItem, deduplicateEquivalent bool) (string, error) {
	if item.Review != harnessdomain.KnowledgeCandidate {
		return "", fmt.Errorf("imported knowledge must remain candidate")
	}
	if item.ContentSHA256 == "" {
		item.ContentSHA256 = harnessdomain.HashKnowledgeContent(item.Content)
	}
	if err := item.Validate(); err != nil {
		return "", err
	}
	project, err := LoadProject(root, "forge")
	if err != nil {
		return "", err
	}
	for _, ref := range project.Manifest.References.Knowledge {
		if ref.ID != item.ID && !deduplicateEquivalent {
			continue
		}
		path, _ := project.Layout.ResolveReference(ref.Path)
		existing, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		current, err := parseKnowledgeFrontMatter(existing)
		if err != nil {
			return "", err
		}
		if ref.ID == item.ID {
			if sameKnowledgeImport(current, item) {
				return ref.Path, nil
			}
			return "", fmt.Errorf("knowledge ID %q already exists with different content", item.ID)
		}
		// A repeated observation can have a different local ID and audit
		// origin while representing the same reviewed claim. Reuse its
		// existing reference without touching the source document or manifest.
		if deduplicateEquivalent && equivalentKnowledgePublication(current, item) {
			return ref.Path, nil
		}
	}
	identifier := sha256.Sum256([]byte(item.ID))
	relative := ".forge/knowledge/items/" + hex.EncodeToString(identifier[:]) + ".md"
	path, err := project.Layout.ResolveReference(relative)
	if err != nil {
		return "", err
	}
	if err := mkdirPathNoSymlinks(project.Layout.Root, filepath.Dir(path)); err != nil {
		return "", err
	}
	encoded, err := yaml.Marshal(item)
	if err != nil {
		return "", err
	}
	document := append(append([]byte("---\n"), encoded...), []byte("---\n")...)
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("unreferenced knowledge file already exists at %s", relative)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	manifestPath := project.Layout.ManifestPath
	original, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(original, &node); err != nil {
		return "", err
	}
	if len(node.Content) == 0 || node.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("Forge manifest must be a mapping")
	}
	mapping := node.Content[0]
	refs := mapValue(mapping, "references")
	if refs == nil {
		refs = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "references"}, refs)
	}
	knowledge := mapValue(refs, "knowledge")
	if knowledge == nil {
		knowledge = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		refs.Content = append(refs.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "knowledge"}, knowledge)
	}
	if knowledge.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("references.knowledge must be a sequence")
	}
	var reference yaml.Node
	if err := yaml.Unmarshal([]byte("id: "+strconvQuote(item.ID)+"\npath: "+strconvQuote(relative)+"\n"), &reference); err != nil {
		return "", err
	}
	knowledge.Content = append(knowledge.Content, reference.Content[0])
	updatedManifest, err := yaml.Marshal(&node)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "knowledge-import-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(document); err == nil {
		err = tmp.Chmod(0600)
	}
	if err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err != nil {
		return "", err
	}
	if err := safefile.Replace(tmpName, path); err != nil {
		return "", err
	}
	current, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(current, original) {
		_ = os.Remove(path)
		return "", fmt.Errorf("Forge manifest changed during knowledge import")
	}
	manifestTmp, err := os.CreateTemp(filepath.Dir(manifestPath), "forge-manifest-import-*")
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	manifestTemp := manifestTmp.Name()
	defer os.Remove(manifestTemp)
	if _, err := manifestTmp.Write(updatedManifest); err == nil {
		err = manifestTmp.Chmod(0600)
	}
	if err == nil {
		err = manifestTmp.Close()
	} else {
		_ = manifestTmp.Close()
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := safefile.Replace(manifestTemp, manifestPath); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return relative, nil
}

func equivalentKnowledgePublication(existing, candidate harnessdomain.KnowledgeItem) bool {
	if existing.Review != harnessdomain.KnowledgeCandidate && existing.Review != harnessdomain.KnowledgeApproved {
		return false
	}
	return existing.Content == candidate.Content &&
		existing.Kind == candidate.Kind &&
		reflect.DeepEqual(existing.Scope, candidate.Scope) &&
		reflect.DeepEqual(existing.Keywords, candidate.Keywords) &&
		reflect.DeepEqual(existing.Evidence, candidate.Evidence)
}

func sameKnowledgeImport(existing, candidate harnessdomain.KnowledgeItem) bool {
	return existing.Content == candidate.Content &&
		existing.Origin == candidate.Origin &&
		existing.Kind == candidate.Kind &&
		reflect.DeepEqual(existing.Scope, candidate.Scope) &&
		reflect.DeepEqual(existing.Keywords, candidate.Keywords) &&
		existing.EvidenceSHA256 == candidate.EvidenceSHA256 &&
		reflect.DeepEqual(existing.Evidence, candidate.Evidence)
}

func parseKnowledgeFrontMatter(data []byte) (harnessdomain.KnowledgeItem, error) {
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "---") {
		return harnessdomain.KnowledgeItem{}, fmt.Errorf("expected YAML front matter")
	}
	parts := strings.SplitN(strings.TrimPrefix(text, "---"), "---", 2)
	if len(parts) != 2 {
		return harnessdomain.KnowledgeItem{}, fmt.Errorf("unterminated front matter")
	}
	var item harnessdomain.KnowledgeItem
	decoder := yaml.NewDecoder(strings.NewReader(strings.TrimSpace(parts[0])))
	decoder.KnownFields(true)
	if err := decoder.Decode(&item); err != nil {
		return item, err
	}
	if err := item.Validate(); err != nil {
		return item, err
	}
	return item, nil
}

func mkdirPathNoSymlinks(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("knowledge directory escapes project root")
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0700); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("knowledge directory contains a non-directory or symlink")
		}
	}
	return nil
}

func mapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func strconvQuote(value string) string {
	data, _ := yaml.Marshal(value)
	return strings.TrimSpace(string(data))
}
