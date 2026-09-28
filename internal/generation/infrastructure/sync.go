package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/agentskills"
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
const syncJournal = ".forge-sync-journal.json"

type syncJournalEntry struct {
	Path        string `json:"path"`
	HadOriginal bool   `json:"had_original"`
	Original    []byte `json:"original,omitempty"`
	NewSHA256   string `json:"new_sha256,omitempty"`
}

type syncJournalFile struct {
	Version int                `json:"version"`
	Stage   string             `json:"stage"`
	Entries []syncJournalEntry `json:"entries"`
}

var errSyncInterrupted = errors.New("simulated sync interruption")

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
	Files     []GeneratedFile   `json:"files"`
	Diff      map[string]string `json:"diff"`
	Changed   bool              `json:"changed"`
	Conflicts []string          `json:"conflicts,omitempty"`
}

func containsGeneratedPath(files []GeneratedFile, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func isSupportedGeneratedPath(value string) bool {
	if value == "AGENTS.md" || value == "CLAUDE.md" {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	for _, prefix := range []string{".agents/skills/", ".claude/skills/"} {
		if strings.HasPrefix(clean, prefix) && clean == value && !strings.Contains(value, "\\") {
			parts := strings.Split(strings.TrimPrefix(value, prefix), "/")
			return len(parts) >= 2 && parts[0] != "" && parts[0] != "." && parts[0] != ".." && parts[len(parts)-1] != "" && parts[len(parts)-1] != "." && parts[len(parts)-1] != ".."
		}
	}
	return false
}

func validateOutputParents(root, relative string) error {
	if relative != generatedManifest && !isSupportedGeneratedPath(relative) {
		return fmt.Errorf("unsupported generated path %q", relative)
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	current := root
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("generated path parent must be a regular directory: %s", relative)
		}
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !os.IsPermission(err) {
		return err
	}
	return nil
}

func writeDurableFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".forge-sync-write-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func createDurableFileExclusive(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".forge-sync-journal-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

// recoverSyncJournal rolls back an interrupted apply. It only changes a file
// when it still contains the transaction's published bytes (or is absent when
// the transaction intended deletion), so a human edit made after interruption
// is preserved and reported as a conflict.
func recoverSyncJournal(root string) error {
	path := filepath.Join(root, syncJournal)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("sync recovery journal must be a regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var journal syncJournalFile
	if err := json.Unmarshal(data, &journal); err != nil || journal.Version != 1 {
		return fmt.Errorf("invalid sync recovery journal")
	}
	seen := map[string]bool{}
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		entry := journal.Entries[i]
		if seen[entry.Path] || (entry.Path != filepath.ToSlash(generatedManifest) && !isSupportedGeneratedPath(entry.Path)) {
			return fmt.Errorf("invalid path in sync recovery journal")
		}
		seen[entry.Path] = true
		if err := validateOutputParents(root, entry.Path); err != nil {
			return err
		}
		dest := filepath.Join(root, filepath.FromSlash(entry.Path))
		currentInfo, statErr := os.Lstat(dest)
		if os.IsNotExist(statErr) {
			if !entry.HadOriginal {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			if err := writeDurableFile(dest, entry.Original, 0644); err != nil {
				return err
			}
			continue
		}
		if statErr != nil {
			return statErr
		}
		if currentInfo.Mode()&os.ModeSymlink != 0 || !currentInfo.Mode().IsRegular() {
			return fmt.Errorf("refusing to recover unsafe generated file %s", entry.Path)
		}
		current, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		if entry.HadOriginal && bytes.Equal(current, entry.Original) {
			continue
		}
		if entry.NewSHA256 == "" {
			return fmt.Errorf("refusing to recover changed file %s", entry.Path)
		}
		sum := sha256.Sum256(current)
		if hex.EncodeToString(sum[:]) != entry.NewSHA256 {
			return fmt.Errorf("refusing to overwrite post-interruption edit %s", entry.Path)
		}
		if entry.HadOriginal {
			if err := writeDurableFile(dest, entry.Original, 0644); err != nil {
				return err
			}
		} else if err := os.Remove(dest); err != nil {
			return err
		} else if err := syncDirectory(filepath.Dir(dest)); err != nil {
			return err
		}
	}
	if journal.Stage != "" {
		if filepath.Base(journal.Stage) != journal.Stage || !strings.HasPrefix(journal.Stage, ".forge-sync-stage-") {
			return fmt.Errorf("invalid staging path in sync recovery journal")
		}
		stage := filepath.Join(root, journal.Stage)
		if stageInfo, err := os.Lstat(stage); err == nil {
			if stageInfo.Mode()&os.ModeSymlink != 0 || !stageInfo.IsDir() {
				return fmt.Errorf("refusing to remove unsafe sync staging path")
			}
			if err := os.RemoveAll(stage); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(root)
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
	input := domain.Input{Project: project.Manifest.Project.Name, Architecture: append([]string(nil), project.Manifest.Architecture.Styles...)}
	rootHandle, err := os.OpenRoot(layout.Root)
	if err != nil {
		return SyncResult{}, err
	}
	defer rootHandle.Close()
	for _, ref := range project.Manifest.References.Knowledge {
		data, err := harnessinfra.ReadProjectFile(rootHandle, ref.Path, inputlimits.HarnessYAMLBytes)
		if err != nil {
			return SyncResult{}, fmt.Errorf("knowledge %q: %w", ref.ID, err)
		}
		item, err := parseKnowledgeDocument(data)
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
		if item.ReviewMetadataSHA256 != harnessdomain.HashKnowledgeReviewMetadata(item) {
			return SyncResult{}, fmt.Errorf("approved knowledge %q review metadata changed after review", item.ID)
		}
		fingerprint, err := harnessinfra.KnowledgeFingerprint(root, ref.Path, ref.ID, item.Evidence)
		if err != nil {
			return SyncResult{}, fmt.Errorf("approved knowledge %q evidence: %w", item.ID, err)
		}
		if !strings.EqualFold(fingerprint, item.EvidenceSHA256) {
			return SyncResult{}, fmt.Errorf("approved knowledge %q evidence changed after review", item.ID)
		}
		input.Rules = append(input.Rules, domain.Rule{ID: item.ID, Description: item.Content, Paths: append([]string(nil), item.Scope.Paths...)})
	}
	for _, s := range project.Manifest.References.Skills {
		input.Skills = append(input.Skills, domain.Skill{ID: s.ID, Description: s.Description, Path: s.Path})
	}
	skillBundles := make([]agentskills.Bundle, 0, len(project.Manifest.References.Skills))
	for _, ref := range project.Manifest.References.Skills {
		bundle, err := agentskills.Read(rootHandle, ref.Path)
		if err != nil {
			return SyncResult{}, fmt.Errorf("skill %q: %w", ref.ID, err)
		}
		skillBundles = append(skillBundles, bundle)
	}
	for _, g := range project.Manifest.QualityGates {
		input.Gates = append(input.Gates, domain.QualityGate{ID: g.ID, Command: g.Command, Workspace: g.Workspace, Workspaces: append([]string(nil), g.Workspaces...)})
	}
	for _, policy := range project.Manifest.Policies {
		input.Policies = append(input.Policies, domain.Policy{ID: policy.ID, Description: policy.Description, Capability: policy.Capability, Executor: policy.Executor})
	}
	result := SyncResult{Diff: map[string]string{}}
	for _, target := range project.Manifest.Targets {
		targetInput := input
		targetInput.Skills = append([]domain.Skill(nil), input.Skills...)
		for i := range targetInput.Skills {
			prefix := ".agents/skills/"
			if target == "claude" {
				prefix = ".claude/skills/"
			}
			targetInput.Skills[i].Path = filepath.ToSlash(filepath.Join(prefix, skillBundles[i].Name, "SKILL.md"))
		}
		var doc domain.Document
		switch target {
		case "codex":
			doc, err = (CodexAdapter{}).Render(targetInput)
		case "claude":
			doc, err = (ClaudeAdapter{}).Render(targetInput)
		default:
			return SyncResult{}, fmt.Errorf("unsupported target %q", target)
		}
		if err != nil {
			return SyncResult{}, err
		}
		sum := sha256.Sum256(doc.Content)
		policyCapability := "unsupported"
		if len(input.Policies) > 0 {
			policyCapability = "declared per policy"
		}
		entry := GeneratedFile{Path: doc.Path, SHA256: hex.EncodeToString(sum[:]), Target: target, AdapterVersion: "1", Capabilities: map[string]string{"scope": "textual", "skills": "native", "quality_gates": "advisory", "policy": policyCapability}}
		result.Files = append(result.Files, entry)
		result.Diff[doc.Path] = string(doc.Content)
		for _, bundle := range skillBundles {
			prefix := ".agents/skills/"
			if target == "claude" {
				prefix = ".claude/skills/"
			}
			for _, relative := range bundle.SortedPaths() {
				output := filepath.ToSlash(filepath.Join(prefix, bundle.Name, filepath.FromSlash(relative)))
				content := bundle.Files[relative]
				sum := sha256.Sum256(content)
				result.Files = append(result.Files, GeneratedFile{Path: output, SHA256: hex.EncodeToString(sum[:]), Target: target, AdapterVersion: "1", Capabilities: map[string]string{"skills": "agent-skills"}})
				result.Diff[output] = string(content)
			}
		}
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
	frontMatter, _, err := splitKnowledgeFrontMatter(data)
	if err != nil {
		return harnessdomain.KnowledgeItem{}, err
	}
	dec := yaml.NewDecoder(strings.NewReader(frontMatter))
	dec.KnownFields(true)
	var item harnessdomain.KnowledgeItem
	if err := dec.Decode(&item); err != nil {
		return item, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return item, fmt.Errorf("expected one YAML document")
	}
	if err := validateKnowledgeItem(item); err != nil {
		return item, err
	}
	return item, nil
}

// splitKnowledgeFrontMatter recognizes delimiters only when they occupy a
// complete line. A plain substring split corrupts values such as review_diff
// that may themselves contain strings beginning with "---".
func splitKnowledgeFrontMatter(data []byte) (frontMatter, body string, err error) {
	text := strings.TrimSpace(string(data))
	lines := strings.SplitAfter(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\n")) != "---" {
		return "", "", fmt.Errorf("expected YAML front matter")
	}
	var offset int
	for i, line := range lines {
		if i == 0 {
			offset += len(line)
			continue
		}
		if strings.TrimSpace(strings.TrimSuffix(line, "\n")) == "---" {
			end := offset
			return strings.TrimSpace(text[len(lines[0]):end]), strings.TrimSpace(text[end+len(line):]), nil
		}
		offset += len(line)
	}
	return "", "", fmt.Errorf("unterminated YAML front matter")
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

func parseKnowledgeDocument(data []byte) (harnessdomain.KnowledgeItem, error) {
	var item harnessdomain.KnowledgeItem
	frontMatter, _, err := splitKnowledgeFrontMatter(data)
	if err != nil {
		return item, err
	}
	dec := yaml.NewDecoder(strings.NewReader(frontMatter))
	dec.KnownFields(true)
	if err := dec.Decode(&item); err != nil {
		return item, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return item, fmt.Errorf("expected one YAML document")
	}
	if err := validateKnowledgeItem(item); err != nil {
		return item, err
	}
	return item, nil
}

// SyncForge supports read-only preview/check and staged apply for static agent files.
func SyncForge(ctx context.Context, root, mode string) (SyncResult, error) {
	return syncForge(ctx, root, mode, nil)
}

func syncForge(ctx context.Context, root, mode string, afterMutation func(int) error) (SyncResult, error) {
	if mode != "dry-run" && mode != "check" && mode != "apply" {
		return SyncResult{}, fmt.Errorf("mode must be dry-run, check, or apply")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return SyncResult{}, err
	}
	rootInfo, err := os.Lstat(abs)
	if err != nil {
		return SyncResult{}, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return SyncResult{}, fmt.Errorf("repository must be a non-symlink directory")
	}
	lock, err := acquireSyncFileLock(ctx, abs)
	if err != nil {
		return SyncResult{}, fmt.Errorf("acquire sync lock: %w", err)
	}
	result, syncErr := syncForgeLocked(ctx, abs, mode, afterMutation)
	closeErr := lock.Close()
	if syncErr != nil {
		return result, syncErr
	}
	if closeErr != nil {
		return result, fmt.Errorf("release sync lock: %w", closeErr)
	}
	return result, nil
}

func syncForgeLocked(ctx context.Context, abs, mode string, afterMutation func(int) error) (SyncResult, error) {
	if mode == "apply" {
		if err := recoverSyncJournal(abs); err != nil {
			return SyncResult{}, err
		}
	}
	result, err := CompileForge(abs)
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
		conflicts, err := syncOwnershipConflicts(abs, result)
		if err != nil {
			return result, err
		}
		result.Conflicts = conflicts
		if len(conflicts) > 0 {
			return result, fmt.Errorf("generated output conflicts: %s", strings.Join(conflicts, ", "))
		}
		return result, nil
	}
	if mode == "check" {
		manifestPath := filepath.Join(abs, filepath.FromSlash(generatedManifest))
		if info, err := os.Lstat(manifestPath); err != nil {
			return result, fmt.Errorf("generated manifest unavailable: %w", err)
		} else if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return result, fmt.Errorf("generated manifest must be a regular non-symlink file")
		}
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return result, fmt.Errorf("generated manifest unavailable: %w", readErr)
		}
		var recorded GeneratedManifest
		if err := json.Unmarshal(data, &recorded); err != nil || recorded.Version != 1 {
			return result, fmt.Errorf("invalid generated manifest")
		}
		seen := map[string]bool{}
		stalePaths := []string{}
		for _, owned := range recorded.Files {
			if !isSupportedGeneratedPath(owned.Path) || seen[owned.Path] {
				return result, fmt.Errorf("invalid generated manifest file entry")
			}
			seen[owned.Path] = true
			if len(owned.SHA256) != sha256.Size*2 {
				return result, fmt.Errorf("invalid generated manifest hash")
			}
			path := filepath.Join(abs, filepath.FromSlash(owned.Path))
			if err := validateOutputParents(abs, owned.Path); err != nil {
				return result, err
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				result.Changed = true
				stalePaths = append(stalePaths, owned.Path)
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return result, err
			}
			sum := sha256.Sum256(content)
			if hex.EncodeToString(sum[:]) != owned.SHA256 {
				result.Changed = true
				stalePaths = append(stalePaths, owned.Path)
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
		if len(seen) != len(result.Files) {
			result.Changed = true
		}
		if len(seen) != len(result.Files) {
			for _, old := range recorded.Files {
				if !containsGeneratedPath(result.Files, old.Path) {
					stalePaths = append(stalePaths, old.Path)
				}
			}
			for _, current := range result.Files {
				if !seen[current.Path] {
					stalePaths = append(stalePaths, current.Path)
				}
			}
		}
		if result.Changed {
			return result, fmt.Errorf("generated files are out of date: %s", strings.Join(stalePaths, ", "))
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
		if info, err := os.Lstat(manifestPath); err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return result, fmt.Errorf("generated manifest must be a regular non-symlink file")
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	owned := map[string]string{}
	seenOwned := map[string]bool{}
	for _, f := range prior.Files {
		if !isSupportedGeneratedPath(f.Path) || seenOwned[f.Path] || len(f.SHA256) != sha256.Size*2 {
			return result, fmt.Errorf("invalid generated manifest file entry")
		}
		if _, err := hex.DecodeString(f.SHA256); err != nil {
			return result, fmt.Errorf("invalid generated manifest hash for %s", f.Path)
		}
		seenOwned[f.Path] = true
		owned[f.Path] = f.SHA256
	}
	staleOwned := make([]string, 0)
	for _, previous := range prior.Files {
		if containsGeneratedPath(result.Files, previous.Path) {
			continue
		}
		path := filepath.Join(abs, filepath.FromSlash(previous.Path))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			delete(owned, previous.Path)
			continue
		}
		if err != nil {
			return result, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return result, fmt.Errorf("refusing to remove unsafe generated file %s", previous.Path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return result, err
		}
		sum := sha256.Sum256(content)
		if hex.EncodeToString(sum[:]) != previous.SHA256 {
			return result, fmt.Errorf("refusing to remove manually edited generated file %s", previous.Path)
		}
		staleOwned = append(staleOwned, previous.Path)
		delete(owned, previous.Path)
	}
	for _, f := range result.Files {
		if err := validateOutputParents(abs, f.Path); err != nil {
			return result, err
		}
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		info, e := os.Lstat(dest)
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			return result, fmt.Errorf("refusing to overwrite unsafe generated file %s", f.Path)
		}
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
		staged := filepath.Join(stage, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(staged), 0755); err != nil {
			return result, err
		}
		if err := os.WriteFile(staged, []byte(result.Diff[f.Path]), 0644); err != nil {
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
	for _, old := range staleOwned {
		path := filepath.Join(abs, filepath.FromSlash(old))
		content, err := os.ReadFile(path)
		if err != nil {
			return result, err
		}
		backups[path] = content
	}
	for _, f := range result.Files {
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		if err := validateOutputParents(abs, f.Path); err != nil {
			return result, err
		}
		if info, e := os.Lstat(dest); e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			return result, fmt.Errorf("refusing to back up unsafe generated file %s", f.Path)
		}
		if old, e := os.ReadFile(dest); e == nil {
			backups[dest] = old
		}
	}
	manifestDest := manifestPath
	if old, e := os.ReadFile(manifestDest); e == nil {
		backups[manifestDest] = old
	}
	journal := syncJournalFile{Version: 1, Stage: filepath.Base(stage)}
	newHashes := map[string]string{}
	for _, f := range result.Files {
		newHashes[f.Path] = f.SHA256
	}
	manifestSum := sha256.Sum256(raw)
	newHashes[generatedManifest] = hex.EncodeToString(manifestSum[:])
	paths := make([]string, 0, len(staleOwned)+len(result.Files)+1)
	paths = append(paths, staleOwned...)
	for _, f := range result.Files {
		paths = append(paths, f.Path)
	}
	paths = append(paths, generatedManifest)
	for _, relative := range paths {
		entry := syncJournalEntry{Path: relative, NewSHA256: newHashes[relative]}
		if old, ok := backups[filepath.Join(abs, filepath.FromSlash(relative))]; ok {
			entry.HadOriginal = true
			entry.Original = old
		}
		journal.Entries = append(journal.Entries, entry)
	}
	journalData, err := json.Marshal(journal)
	if err != nil {
		return result, err
	}
	journalPath := filepath.Join(abs, syncJournal)
	if err := createDurableFileExclusive(journalPath, append(journalData, '\n'), 0600); err != nil {
		return result, fmt.Errorf("create sync recovery journal: %w", err)
	}
	rollbackOnError := func(cause error) error {
		if recoveryErr := recoverSyncJournal(abs); recoveryErr != nil {
			return errors.Join(cause, fmt.Errorf("sync recovery failed: %w", recoveryErr))
		}
		return cause
	}
	mutation := 0
	after := func() error {
		mutation++
		if afterMutation != nil {
			return afterMutation(mutation)
		}
		return nil
	}
	for _, old := range staleOwned {
		if err := ctx.Err(); err != nil {
			return result, rollbackOnError(err)
		}
		path := filepath.Join(abs, filepath.FromSlash(old))
		if err := os.Remove(path); err != nil {
			return result, rollbackOnError(err)
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return result, rollbackOnError(err)
		}
		if err := after(); err != nil {
			if errors.Is(err, errSyncInterrupted) {
				return result, err
			}
			return result, rollbackOnError(err)
		}
	}
	for _, f := range result.Files {
		if err := ctx.Err(); err != nil {
			return result, rollbackOnError(err)
		}
		dest := filepath.Join(abs, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return result, rollbackOnError(err)
		}
		if err := validateOutputParents(abs, f.Path); err != nil {
			return result, rollbackOnError(err)
		}
		if err := os.Rename(filepath.Join(stage, filepath.FromSlash(f.Path)), dest); err != nil {
			return result, rollbackOnError(err)
		}
		if err := syncDirectory(filepath.Dir(dest)); err != nil {
			return result, rollbackOnError(err)
		}
		if err := after(); err != nil {
			if errors.Is(err, errSyncInterrupted) {
				return result, err
			}
			return result, rollbackOnError(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(manifestDest), 0755); err != nil {
		return result, rollbackOnError(err)
	}
	if err := os.Rename(filepath.Join(stage, "generated-manifest.json"), manifestDest); err != nil {
		return result, rollbackOnError(err)
	}
	if err := syncDirectory(filepath.Dir(manifestDest)); err != nil {
		return result, rollbackOnError(err)
	}
	if err := after(); err != nil {
		if errors.Is(err, errSyncInterrupted) {
			return result, err
		}
		return result, rollbackOnError(err)
	}
	if err := os.Remove(journalPath); err != nil {
		return result, fmt.Errorf("sync completed but recovery journal could not be removed: %w", err)
	}
	if err := syncDirectory(abs); err != nil {
		return result, fmt.Errorf("sync completed but recovery journal removal was not durable: %w", err)
	}
	return result, nil
}

// syncOwnershipConflicts diagnoses preview collisions using the recorded hashes
// without modifying outputs or ownership metadata.
func syncOwnershipConflicts(root string, result SyncResult) ([]string, error) {
	path := filepath.Join(root, filepath.FromSlash(generatedManifest))
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		info = nil
	} else if err != nil {
		return nil, err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("generated manifest must be a regular non-symlink file")
	}
	owned := map[string]string{}
	if info != nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var manifest GeneratedManifest
		if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version != 1 {
			return nil, fmt.Errorf("invalid generated manifest")
		}
		for _, file := range manifest.Files {
			if !isSupportedGeneratedPath(file.Path) || owned[file.Path] != "" || len(file.SHA256) != sha256.Size*2 {
				return nil, fmt.Errorf("invalid generated manifest file entry")
			}
			if _, err := hex.DecodeString(file.SHA256); err != nil {
				return nil, fmt.Errorf("invalid generated manifest hash for %s", file.Path)
			}
			owned[file.Path] = file.SHA256
		}
	}
	var conflicts []string
	for _, file := range result.Files {
		rel := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := validateOutputParents(root, file.Path); err != nil {
			return nil, err
		}
		fileInfo, err := os.Lstat(rel)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
			conflicts = append(conflicts, file.Path+" (unsafe output)")
			continue
		}
		content, err := os.ReadFile(rel)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		actual := hex.EncodeToString(sum[:])
		if owned[file.Path] == "" {
			conflicts = append(conflicts, file.Path+" (unmanaged output)")
		} else if owned[file.Path] != actual {
			conflicts = append(conflicts, file.Path+" (manually edited managed output)")
		}
	}
	sort.Strings(conflicts)
	return conflicts, nil
}

// CheckForge reports generated drift without writing any project files.
func CheckForge(ctx context.Context, root string) (SyncResult, error) {
	return SyncForge(ctx, root, "check")
}

// CheckLegacyForgeOutputs compiles the existing Harness layout without writing
// and compares it to its generated agent files. A project with no layout is
// outside this check and succeeds without findings.
func CheckLegacyForgeOutputs(ctx context.Context, root string) (SyncResult, error) {
	layout, err := harnessinfra.ResolveLayout(root, "harness")
	if err != nil {
		if strings.Contains(err.Error(), "missing") || strings.Contains(err.Error(), "no supported") {
			return SyncResult{}, nil
		}
		return SyncResult{}, err
	}
	ownership, err := os.ReadFile(filepath.Join(root, ownershipFile))
	if os.IsNotExist(err) {
		return SyncResult{}, nil
	}
	if err != nil {
		return SyncResult{}, err
	}
	if parseOwnedHash(ownership, "AGENTS.md") == "" {
		return SyncResult{}, nil
	}
	h, err := (harnessinfra.YAMLLoader{}).Load(layout.HarnessPath)
	if err != nil {
		return SyncResult{}, err
	}
	input := domain.Input{Project: h.Project.Name}
	for _, r := range h.Rules {
		if r.Status == "approved" {
			input.Rules = append(input.Rules, domain.Rule{ID: r.ID, Description: r.Description, Paths: append([]string(nil), r.Scope.Paths...)})
		}
	}
	for _, s := range h.Skills {
		if s.Status == "" || s.Status == "approved" {
			input.Skills = append(input.Skills, domain.Skill{ID: s.ID, Description: s.Description, Path: s.Path})
		}
	}
	for _, g := range h.QualityGates {
		input.Gates = append(input.Gates, domain.QualityGate{ID: g.ID, Command: g.Command, Workspace: g.Workspace, Workspaces: append([]string(nil), g.Workspaces...)})
	}
	doc, err := (CodexAdapter{}).Render(input)
	if err != nil {
		return SyncResult{}, err
	}
	outputPath := filepath.Join(root, doc.Path)
	info, err := os.Lstat(outputPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return SyncResult{}, fmt.Errorf("managed generated output %s is unavailable or unsafe", doc.Path)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return SyncResult{}, fmt.Errorf("generated output %s unavailable: %w", doc.Path, err)
	}
	sum := sha256.Sum256(data)
	if parseOwnedHash(ownership, doc.Path) != hex.EncodeToString(sum[:]) {
		return SyncResult{}, fmt.Errorf("generated output %s was edited after generation", doc.Path)
	}
	if !bytes.Equal(data, doc.Content) {
		return SyncResult{}, fmt.Errorf("generated output %s is out of date", doc.Path)
	}
	return SyncResult{Files: []GeneratedFile{{Path: doc.Path}}, Changed: false}, nil
}
