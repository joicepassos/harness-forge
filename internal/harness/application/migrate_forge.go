package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	"harnessforge/schemas"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MigrationFile struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Content string `json:"content"`
}

type ForgeMigrationPlan struct {
	SourcePath string          `json:"source_path"`
	SourceHash string          `json:"source_sha256"`
	FromIR     int             `json:"from_ir_version"`
	ToLayout   int             `json:"to_layout_version"`
	Files      []MigrationFile `json:"files"`
	Losses     []string        `json:"losses"`
	Unmapped   []string        `json:"unmapped"`
	Warnings   []string        `json:"warnings"`
}

type forgeMigrationReport struct {
	Version    int               `json:"version"`
	SourcePath string            `json:"source_path"`
	SourceHash string            `json:"source_sha256"`
	Files      map[string]string `json:"files"`
}

// PreviewToForge builds a deterministic file plan without writing to disk.
// A rule kind must be supplied when legacy rules have no equivalent type.
func PreviewToForge(loader Loader, root, source string, targets []string, defaultRuleKind domain.KnowledgeKind) (ForgeMigrationPlan, error) {
	var plan ForgeMigrationPlan
	root, err := filepath.Abs(root)
	if err != nil {
		return plan, err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return plan, err
	}
	relativeSource, err := filepath.Rel(root, source)
	if err != nil || relativeSource == ".." || strings.HasPrefix(relativeSource, ".."+string(filepath.Separator)) {
		return plan, fmt.Errorf("migration source must be inside the project root")
	}
	if _, err := os.Lstat(filepath.Join(root, ".forge")); err == nil {
		return plan, fmt.Errorf(".forge already exists; migration will not overwrite it")
	} else if !os.IsNotExist(err) {
		return plan, err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return plan, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return plan, fmt.Errorf("migration source must be a regular non-symlink file")
	}
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		return plan, err
	}
	sourceSum := sha256.Sum256(sourceBytes)
	plan.SourcePath = filepath.ToSlash(relativeSource)
	plan.SourceHash = hex.EncodeToString(sourceSum[:])
	plan.ToLayout = 1
	h, err := loader.Load(source)
	if err != nil {
		return plan, err
	}
	plan.FromIR = h.Version
	if len(targets) == 0 {
		plan.Unmapped = append(plan.Unmapped, "targets (select at least one with --target)")
	}
	if len(h.Rules) > 0 && !validKnowledgeKind(defaultRuleKind) {
		plan.Unmapped = append(plan.Unmapped, "rules[*].kind (supply --rule-kind because the legacy IR does not classify rules)")
	}
	if len(plan.Unmapped) > 0 {
		return plan, nil
	}
	manifest := domain.Manifest{
		LayoutVersion: 1,
		IRVersion:     h.Version,
		Project:       h.Project,
		Targets:       append([]string(nil), targets...),
		References: domain.ManifestReferences{
			Knowledge: make([]domain.KnowledgeReference, 0, len(h.Rules)),
			Skills:    make([]domain.SkillReference, 0, len(h.Skills)),
		},
		QualityGates: append([]domain.QualityGate(nil), h.QualityGates...),
	}
	if err := manifest.Validate(); err != nil {
		return plan, fmt.Errorf("migration choices are invalid: %w", err)
	}
	for _, rule := range h.Rules {
		item := domain.KnowledgeItem{
			ID: rule.ID, Kind: defaultRuleKind, Scope: rule.Scope,
			Content: rule.Description, Origin: rule.Origin,
			Review: domain.KnowledgeCandidate, Health: domain.KnowledgeUnknown,
			ContentSHA256:      domain.HashKnowledgeContent(rule.Description),
			LegacyReviewStatus: rule.Status, LegacyReview: rule.Review,
			Evidence: make([]domain.KnowledgeEvidence, len(rule.Evidence)),
		}
		for i, evidence := range rule.Evidence {
			item.Evidence[i] = domain.KnowledgeEvidence{Path: evidence.File, Workspace: evidence.Workspace, Kind: evidence.Kind, Symbol: evidence.Symbol, Quote: evidence.Quote, StartLine: evidence.StartLine, EndLine: evidence.EndLine, SHA256: evidence.SHA256, Revision: evidence.Revision}
		}
		if err := item.Validate(); err != nil {
			return plan, fmt.Errorf("cannot migrate rule %q: %w", rule.ID, err)
		}
		encoded, err := yaml.Marshal(item)
		if err != nil {
			return plan, err
		}
		idSum := sha256.Sum256([]byte(rule.ID))
		itemPath := ".forge/knowledge/items/" + hex.EncodeToString(idSum[:]) + ".md"
		content := "---\n" + string(encoded) + "---\n"
		plan.Files = append(plan.Files, migrationFile(itemPath, content))
		manifest.References.Knowledge = append(manifest.References.Knowledge, domain.KnowledgeReference{ID: rule.ID, Path: itemPath})
		if rule.Status != "candidate" {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("rule %q is imported as candidate; legacy status %q is preserved for explicit re-review", rule.ID, rule.Status))
		}
	}
	for _, skill := range h.Skills {
		manifest.References.Skills = append(manifest.References.Skills, domain.SkillReference{
			ID: skill.ID, Description: skill.Description, Path: skill.Path,
			Evidence: append([]domain.Evidence(nil), skill.Evidence...),
		})
	}
	manifestYAML, err := yaml.Marshal(manifest)
	if err != nil {
		return plan, err
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return plan, err
	}
	if err := schemas.Validate("forge-layout-v1.schema.json", manifestJSON); err != nil {
		return plan, err
	}
	plan.Files = append(plan.Files, migrationFile(".forge/forge.yaml", string(manifestYAML)))
	sort.Slice(plan.Files, func(i, j int) bool { return plan.Files[i].Path < plan.Files[j].Path })
	return plan, nil
}

func validKnowledgeKind(kind domain.KnowledgeKind) bool {
	return kind == domain.KnowledgeFact || kind == domain.KnowledgeConvention || kind == domain.KnowledgeBusinessRule || kind == domain.KnowledgeDecision || kind == domain.KnowledgeConstraint
}

func migrationFile(path, content string) MigrationFile {
	sum := sha256.Sum256([]byte(content))
	return MigrationFile{Path: filepath.ToSlash(path), SHA256: hex.EncodeToString(sum[:]), Content: content}
}

// ApplyForgeMigration stages all planned files, rechecks the source hash, then
// publishes the complete .forge directory with one same-volume rename. The
// original Harness layout remains untouched for rollback.
func ApplyForgeMigration(root string, plan ForgeMigrationPlan) error {
	if len(plan.Unmapped) > 0 {
		return fmt.Errorf("migration has unmapped choices: %s", strings.Join(plan.Unmapped, "; "))
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	source := filepath.Join(root, filepath.FromSlash(plan.SourcePath))
	relative, err := filepath.Rel(root, source)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("migration source is outside project root")
	}
	sourceBytes, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	sourceHash := sha256.Sum256(sourceBytes)
	if hex.EncodeToString(sourceHash[:]) != plan.SourceHash {
		return fmt.Errorf("migration source changed after preview; create a new preview")
	}
	destination := filepath.Join(root, ".forge")
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf(".forge already exists; migration will not overwrite it")
	} else if !os.IsNotExist(err) {
		return err
	}
	stage, err := os.MkdirTemp(root, ".forge-stage-*")
	if err != nil {
		return err
	}
	defer func() { _ = removeTreeWithin(root, stage) }()
	report := forgeMigrationReport{Version: 1, SourcePath: plan.SourcePath, SourceHash: plan.SourceHash, Files: map[string]string{}}
	for _, file := range plan.Files {
		if !strings.HasPrefix(file.Path, ".forge/") || strings.Contains(file.Path, "\\") {
			return fmt.Errorf("invalid migration output path %q", file.Path)
		}
		contentHash := sha256.Sum256([]byte(file.Content))
		if hex.EncodeToString(contentHash[:]) != file.SHA256 {
			return fmt.Errorf("migration plan file %q changed after preview", file.Path)
		}
		relativePath := strings.TrimPrefix(file.Path, ".forge/")
		clean := filepath.Clean(filepath.FromSlash(relativePath))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return fmt.Errorf("invalid migration output path %q", file.Path)
		}
		target := filepath.Join(stage, clean)
		rel, err := filepath.Rel(stage, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("migration output escaped staging directory")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0600); err != nil {
			return err
		}
		report.Files[filepath.ToSlash(clean)] = file.SHA256
	}
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "migration-report.json"), append(reportBytes, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Rename(stage, destination); err != nil {
		return err
	}
	return nil
}

// RollbackForgeMigration removes only a migration-created .forge tree whose
// files still match the recorded hashes. Human edits or unowned files conflict.
func RollbackForgeMigration(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	destination := filepath.Join(root, ".forge")
	info, err := os.Lstat(destination)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf(".forge must be a regular directory for rollback")
	}
	reportPath := filepath.Join(destination, "migration-report.json")
	reportBytes, err := os.ReadFile(reportPath)
	if err != nil {
		return fmt.Errorf("migration report is unavailable: %w", err)
	}
	var report forgeMigrationReport
	if err := json.Unmarshal(reportBytes, &report); err != nil || report.Version != 1 || len(report.Files) == 0 {
		return fmt.Errorf("invalid migration report; refusing rollback")
	}
	expected := map[string]bool{"migration-report.json": true}
	for relative, hash := range report.Files {
		clean := filepath.Clean(filepath.FromSlash(relative))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(relative, "\\") {
			return fmt.Errorf("invalid path in migration report")
		}
		path := filepath.Join(destination, clean)
		rel, err := filepath.Rel(destination, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("migration report path escaped .forge")
		}
		if err := verifyRegularFileNoSymlinkParents(destination, clean); err != nil {
			return fmt.Errorf("migration output %q is missing or not a regular file: %w", relative, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != hash {
			return fmt.Errorf("migration output %q changed; rollback refused", relative)
		}
		expected[filepath.ToSlash(clean)] = true
	}
	actual := map[string]bool{}
	actualDirs := map[string]bool{}
	var inspect func(string) error
	inspect = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink found in migrated output")
			}
			relative, err := filepath.Rel(destination, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if info.IsDir() {
				actualDirs[relative] = true
				if err := inspect(path); err != nil {
					return err
				}
				continue
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("non-regular file found in migrated output")
			}
			actual[relative] = true
		}
		return nil
	}
	if err := inspect(destination); err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf(".forge contains unowned files; rollback refused")
	}
	for path := range actual {
		if !expected[path] {
			return fmt.Errorf(".forge contains unowned file %q; rollback refused", path)
		}
	}
	expectedDirs := map[string]bool{}
	for file := range expected {
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(file))); parent != "."; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			expectedDirs[parent] = true
		}
	}
	if len(actualDirs) != len(expectedDirs) {
		return fmt.Errorf(".forge contains an unowned directory; rollback refused")
	}
	for path := range actualDirs {
		if !expectedDirs[path] {
			return fmt.Errorf(".forge contains unowned directory %q; rollback refused", path)
		}
	}
	return removeTreeWithin(root, destination)
}

func verifyRegularFileNoSymlinkParents(root, relative string) error {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	current := root
	for index, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in migration output path")
		}
		if index == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("expected regular file")
			}
		} else if !info.IsDir() {
			return fmt.Errorf("parent is not a directory")
		}
	}
	return nil
}

func removeTreeWithin(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to remove path outside project root")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing to remove non-directory or symlink")
	}
	return os.RemoveAll(target)
}
