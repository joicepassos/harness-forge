package infrastructure

import (
	"fmt"
	"harnessforge/internal/harness/infrastructure"
	"harnessforge/internal/securityboundary"
	"harnessforge/internal/skills/domain"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Store struct{}

func (Store) Generate(repository string, proposal domain.Proposal) error {
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("repository must be a directory")
	}
	harnessDir, err := securityboundary.PrepareDirectory(root, ".harness")
	if err != nil {
		return err
	}
	harnessPath := filepath.Join(harnessDir, "harness.yaml")
	h, err := (infrastructure.YAMLLoader{}).Load(harnessPath)
	if err != nil {
		return err
	}
	for _, skill := range h.Skills {
		if skill.ID == proposal.ID {
			if skill.Path == ".harness/skills/"+proposal.ID+"/SKILL.md" {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(skill.Path))); err == nil {
					return nil
				}
			}
			return fmt.Errorf("skill reference already exists but is incomplete")
		}
	}
	skillsRoot := filepath.Join(harnessDir, "skills")
	if parent, err := os.Lstat(skillsRoot); err == nil && parent.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("skills directory cannot be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Join(skillsRoot, proposal.ID)
	if err := ensureWithin(root, dir); err != nil {
		return err
	}
	if info, err := os.Lstat(dir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill directory cannot be a symlink")
		}
		return fmt.Errorf("skill directory already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "SKILL.md")
	content := render(proposal)
	tmp, err := os.CreateTemp(dir, "skill-*.md")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := appendReference(harnessPath, proposal); err != nil {
		os.Remove(path)
		os.Remove(dir)
		return err
	}
	return nil
}

func appendReference(path string, proposal domain.Proposal) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytesContainsSkill(original) {
		return fmt.Errorf("manual skills section requires a manual reference")
	}
	entry := "\nskills:\n  - id: " + strconv.Quote(proposal.ID) + "\n    description: " + strconv.Quote(proposal.Description) + "\n    path: " + strconv.Quote(".harness/skills/"+proposal.ID+"/SKILL.md") + "\n    status: approved\n    evidence:\n"
	for _, evidence := range proposal.Examples {
		entry += "      - file: " + strconv.Quote(evidence.File) + "\n        symbol: " + strconv.Quote(evidence.Symbol) + "\n"
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "harness-*.yaml")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(append([]byte{}, original...), []byte(entry)...)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, err := (infrastructure.YAMLLoader{}).Load(name); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != string(original) {
		return fmt.Errorf("harness changed during skill generation")
	}
	return os.Rename(name, path)
}

func bytesContainsSkill(data []byte) bool { return strings.Contains("\n"+string(data), "\nskills:") }
func ensureWithin(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe skill path")
	}
	return nil
}
func render(proposal domain.Proposal) string {
	text := "# " + proposal.ID + "\n\n" + proposal.Description + "\n\n## Evidence\n"
	for _, evidence := range proposal.Examples {
		text += "- " + evidence.File + ": " + evidence.Symbol + "\n"
	}
	text += "\n## Limitations\n"
	for _, limitation := range proposal.Limitations {
		text += "- " + limitation + "\n"
	}
	return text
}
