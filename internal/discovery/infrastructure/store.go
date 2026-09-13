package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/discovery/domain"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
	"strings"
)

type Store struct{}

func (Store) Apply(repository, path string, proposal domain.Proposal) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("harness must be a regular file")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	h, err := (harnessinfra.YAMLLoader{}).Load(path)
	if err != nil {
		return err
	}
	for _, rule := range h.Rules {
		if rule.ID == proposal.ID {
			return nil
		}
	}
	evidence := make([]harnessdomain.Evidence, len(proposal.Evidence))
	for i, item := range proposal.Evidence {
		evidence[i] = harnessdomain.Evidence{File: item.File, Symbol: item.Symbol, Revision: item.Revision}
	}
	candidate := harnessdomain.Rule{ID: proposal.ID, Description: proposal.Description, Origin: "ai", Status: "approved", Evidence: evidence}
	check := harnessdomain.Harness{Version: 1, Project: harnessdomain.Project{Name: "validation"}, Rules: []harnessdomain.Rule{candidate}}
	if err := check.Validate(); err != nil {
		return err
	}
	if err := harnessinfra.CheckEvidence(context.Background(), repository, check); err != nil {
		return err
	}
	encoded, err := yaml.Marshal([]harnessdomain.Rule{candidate})
	if err != nil {
		return err
	}
	addition := []byte{}
	if len(h.Rules) == 0 {
		addition = []byte("\nrules:\n" + indentYAML(encoded))
	}
	return appendRule(path, original, addition, encoded, info.Mode().Perm(), len(h.Rules) == 0)
}
func appendRule(path string, original, addition, encoded []byte, mode os.FileMode, newSection bool) error {
	if !newSection {
		addition = []byte(indentYAML(encoded))
	}
	updated := append(append([]byte{}, original...), addition...)
	if len(updated) == 0 || updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "harness-discovery-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(updated); err == nil {
		err = temporary.Chmod(mode)
	}
	if err == nil {
		err = temporary.Close()
	}
	if err != nil {
		return err
	}
	if _, err := (harnessinfra.YAMLLoader{}).Load(name); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("harness changed during discovery")
	}
	return os.Rename(name, path)
}

func indentYAML(data []byte) string {
	return "  " + strings.TrimSuffix(strings.ReplaceAll(string(data), "\n", "\n  "), "  ")
}
