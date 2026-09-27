package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/generation/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"harnessforge/internal/inputlimits"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const generatedManifest = ".forge/generated-manifest.json"

type GeneratedFile struct {
	Path           string            `json:"path"`
	SHA256         string            `json:"sha256"`
	Target         string            `json:"target"`
	AdapterVersion string            `json:"adapter_version"`
	Capabilities   map[string]string `json:"capabilities"`
}
type GeneratedManifest struct {
	Version int             `json:"version"`
	Files   []GeneratedFile `json:"files"`
}
type SyncResult struct {
	Files   []GeneratedFile   `json:"files"`
	Diff    map[string]string `json:"diff"`
	Changed bool              `json:"changed"`
}

// CompileForge loads approved knowledge and renders deterministic target files without writing.
func CompileForge(root string) (SyncResult, error) {
	layout, err := harnessinfra.ResolveLayout(root, "forge")
	if err != nil {
		return SyncResult{}, err
	}
	project, err := harnessinfra.LoadProject(root, "forge")
	if err != nil {
		return SyncResult{}, err
	}
	if err := harnessinfra.ValidateManifestReferences(layout, *project.Manifest); err != nil {
		return SyncResult{}, err
	}
	input := domain.Input{Project: project.Manifest.Project.Name}
	for _, ref := range project.Manifest.References.Knowledge {
		path, _ := layout.ResolveReference(ref.Path)
		item, err := loadKnowledge(path)
		if err != nil {
			return SyncResult{}, fmt.Errorf("knowledge %q: %w", ref.ID, err)
		}
		if item.ID != ref.ID {
			return SyncResult{}, fmt.Errorf("knowledge reference %q contains ID %q", ref.ID, item.ID)
		}
		if item.Review != harnessdomain.KnowledgeApproved {
			continue
		}
		if item.ContentSHA256 != harnessdomain.HashKnowledgeContent(item.Content) {
			return SyncResult{}, fmt.Errorf("approved knowledge %q changed after review", item.ID)
		}
		input.Rules = append(input.Rules, domain.Rule{ID: item.ID, Description: item.Content, Paths: append([]string(nil), item.Scope.Paths...)})
	}
	for _, s := range project.Manifest.References.Skills {
		input.Skills = append(input.Skills, domain.Skill{ID: s.ID, Description: s.Description, Path: s.Path})
	}
	for _, g := range project.Manifest.QualityGates {
		input.Gates = append(input.Gates, domain.QualityGate{ID: g.ID, Command: g.Command, Workspace: g.Workspace, Workspaces: append([]string(nil), g.Workspaces...)})
	}
	result := SyncResult{Diff: map[string]string{}}
	for _, target := range project.Manifest.Targets {
		var doc domain.Document
		switch target {
		case "codex":
			doc, err = (CodexAdapter{}).Render(input)
		case "claude":
			doc, err = (ClaudeAdapter{}).Render(input)
		default:
			return SyncResult{}, fmt.Errorf("unsupported target %q", target)
		}
		if err != nil {
			return SyncResult{}, err
		}
		sum := sha256.Sum256(doc.Content)
		entry := GeneratedFile{Path: doc.Path, SHA256: hex.EncodeToString(sum[:]), Target: target, AdapterVersion: "1", Capabilities: map[string]string{"scope": "textual", "skills": "reference", "quality_gates": "advisory"}}
		result.Files = append(result.Files, entry)
		result.Diff[doc.Path] = string(doc.Content)
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	seen := map[string]bool{}
	for _, f := range result.Files {
		if seen[f.Path] {
			return SyncResult{}, fmt.Errorf("target collision at %s", f.Path)
		}
		seen[f.Path] = true
	}
	return result, nil
}

func loadKnowledge(path string) (harnessdomain.KnowledgeItem, error) {
	data, err := inputlimits.ReadFile(path, inputlimits.HarnessYAMLBytes, "Forge knowledge item")
	if err != nil {
		return harnessdomain.KnowledgeItem{}, err
	}
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "---") {
		return harnessdomain.KnowledgeItem{}, fmt.Errorf("expected YAML front matter")
	}
	parts := strings.SplitN(strings.TrimPrefix(text, "---"), "---", 2)
	if len(parts) != 2 {
		return harnessdomain.KnowledgeItem{}, fmt.Errorf("unterminated YAML front matter")
	}
	dec := yaml.NewDecoder(strings.NewReader(strings.TrimSpace(parts[0])))
	dec.KnownFields(true)
	var item harnessdomain.KnowledgeItem
	if err := dec.Decode(&item); err != nil {
		return item, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return item, fmt.Errorf("expected one YAML document")
	}
	if err := item.Validate(); err != nil {
		return item, err
	}
	return item, nil
}

// SyncForge supports read-only preview/check and staged apply for static agent files.
func SyncForge(ctx context.Context, root, mode string) (SyncResult, error) {
	if mode != "dry-run" && mode != "check" && mode != "apply" {
		return SyncResult{}, fmt.Errorf("mode must be dry-run, check, or apply")
	}
	result, err := CompileForge(root)
	if err != nil {
		return result, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return result, err
	}
	for i := range result.Files {
		entry := &result.Files[i]
		path := filepath.Join(abs, filepath.FromSlash(entry.Path))
		if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("refusing generated symlink %s", entry.Path)
		}
		old, e := os.ReadFile(path)
		if e == nil && string(old) != result.Diff[entry.Path] {
			result.Changed = true
		} else if os.IsNotExist(e) {
			result.Changed = true
		} else if e != nil {
			return result, e
		}
	}
	if mode == "dry-run" {
		return result, nil
	}
	if mode == "check" {
		manifestPath := filepath.Join(abs, filepath.FromSlash(generatedManifest))
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return result, fmt.Errorf("generated manifest unavailable: %w", readErr)
		}
		var recorded GeneratedManifest
		if err := json.Unmarshal(data, &recorded); err != nil || recorded.Version != 1 {
			return result, fmt.Errorf("invalid generated manifest")
		}
		for _, owned := range recorded.Files {
			path := filepath.Join(abs, filepath.FromSlash(owned.Path))
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				result.Changed = true
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return result, err
			}
			sum := sha256.Sum256(content)
			if hex.EncodeToString(sum[:]) != owned.SHA256 {
				result.Changed = true
			}
		}
		actual, err := json.Marshal(result.Files)
		if err != nil {
			return result, err
		}
		expected, err := json.Marshal(recorded.Files)
		if err != nil {
			return result, err
		}
		if !bytes.Equal(actual, expected) {
			result.Changed = true
		}
		if result.Changed {
			return result, fmt.Errorf("generated files are out of date")
		}
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	manifestPath := filepath.Join(abs, filepath.FromSlash(generatedManifest))
	prior := GeneratedManifest{}
	if raw, e := os.ReadFile(manifestPath); e == nil {
		if err := json.Unmarshal(raw, &prior); err != nil {
			return result, err
		}
		if prior.Version != 1 {
			return result, fmt.Errorf("unsupported generated manifest version")
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	owned := map[string]string{}
	for _, f := range prior.Files {
		if f.Path != "AGENTS.md" && f.Path != "CLAUDE.md" {
			return result, fmt.Errorf("invalid path in generated manifest")
		}
		owned[f.Path] = f.SHA256
	}
	for _, f := range result.Files {
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		old, e := os.ReadFile(dest)
		if e == nil {
			sum := sha256.Sum256(old)
			if owned[f.Path] == "" || owned[f.Path] != hex.EncodeToString(sum[:]) {
				return result, fmt.Errorf("refusing to overwrite edited or unowned %s", f.Path)
			}
		} else if !os.IsNotExist(e) {
			return result, e
		}
	}
	stage, err := os.MkdirTemp(abs, ".forge-sync-stage-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(stage)
	for _, f := range result.Files {
		if err := os.WriteFile(filepath.Join(stage, filepath.Base(f.Path)), []byte(result.Diff[f.Path]), 0644); err != nil {
			return result, err
		}
	}
	manifest := GeneratedManifest{Version: 1, Files: result.Files}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return result, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(stage, "generated-manifest.json"), raw, 0644); err != nil {
		return result, err
	}
	backups := map[string][]byte{}
	for _, f := range result.Files {
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		if old, e := os.ReadFile(dest); e == nil {
			backups[dest] = old
		}
	}
	manifestDest := manifestPath
	if old, e := os.ReadFile(manifestDest); e == nil {
		backups[manifestDest] = old
	}
	committed := []string{}
	rollback := func() {
		for _, p := range committed {
			if b, ok := backups[p]; ok {
				_ = os.WriteFile(p, b, 0644)
			} else {
				_ = os.Remove(p)
			}
		}
	}
	for _, f := range result.Files {
		if err := ctx.Err(); err != nil {
			rollback()
			return result, err
		}
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		if err := os.Rename(filepath.Join(stage, filepath.Base(f.Path)), dest); err != nil {
			rollback()
			return result, err
		}
		committed = append(committed, dest)
	}
	if err := os.MkdirAll(filepath.Dir(manifestDest), 0755); err != nil {
		rollback()
		return result, err
	}
	if err := os.Rename(filepath.Join(stage, "generated-manifest.json"), manifestDest); err != nil {
		rollback()
		return result, err
	}
	committed = append(committed, manifestDest)
	return result, nil
}
