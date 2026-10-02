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
	"slices"
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
	evidence := make([]harnessdomain.Evidence, len(proposal.Evidence))
	for i, item := range proposal.Evidence {
		evidence[i] = harnessdomain.Evidence{File: item.File, Kind: item.Kind, Workspace: item.Workspace, Symbol: item.Symbol, Quote: item.Quote, StartLine: item.StartLine, EndLine: item.EndLine, SHA256: item.SHA256, Revision: item.Revision}
	}
	// AI output is a proposal. Approval is a separate human decision and must
	// not be implied by persisting the proposal.
	candidate := harnessdomain.Rule{ID: proposal.ID, Description: proposal.Description, Origin: "ai", Status: "candidate", Evidence: evidence}
	for _, rule := range h.Rules {
		if rule.ID != proposal.ID {
			continue
		}
		if rule.Description == candidate.Description && slices.Equal(rule.Evidence, candidate.Evidence) {
			return nil
		}
		return fmt.Errorf("proposal ID %q already exists with different content", proposal.ID)
	}
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
	updated, err := appendRule(original, encoded)
	if err != nil {
		return err
	}
	return writeValidated(path, original, updated, info.Mode().Perm())
}
func appendRule(original, encoded []byte) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(original, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("harness must be a YAML mapping")
	}
	mapping := document.Content[0]
	for index := 0; index < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if key.Value != "rules" {
			continue
		}
		if value.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("rules must be a YAML sequence")
		}
		if len(value.Content) == 0 && value.Style&yaml.FlowStyle != 0 {
			scalarStart := offsetAt(original, value.Line, value.Column)
			start := scalarStart
			for start > 0 && (original[start-1] == ' ' || original[start-1] == '\t') {
				start--
			}
			end := scalarStart + 2
			if end < len(original) && original[end] == '\r' {
				end++
			}
			if end < len(original) && original[end] == '\n' {
				end++
			}
			return replace(original, start, end, []byte("\n"+indentYAML(encoded))), nil
		}
		insertAt := len(original)
		if index+2 < len(mapping.Content) {
			next := mapping.Content[index+2]
			insertAt = offsetAt(original, next.Line, next.Column)
		}
		return insert(original, insertAt, []byte(indentYAML(encoded))), nil
	}
	addition := []byte("\nrules:\n" + indentYAML(encoded))
	return append(append([]byte{}, original...), addition...), nil
}

func offsetAt(data []byte, line, column int) int {
	if line <= 1 {
		return max(column-1, 0)
	}
	offset := 0
	for current := 1; current < line && offset < len(data); current++ {
		next := bytes.IndexByte(data[offset:], '\n')
		if next < 0 {
			return len(data)
		}
		offset += next + 1
	}
	return min(offset+max(column-1, 0), len(data))
}

func insert(data []byte, at int, addition []byte) []byte {
	updated := make([]byte, 0, len(data)+len(addition))
	updated = append(updated, data[:at]...)
	updated = append(updated, addition...)
	return append(updated, data[at:]...)
}

func replace(data []byte, start, end int, value []byte) []byte {
	updated := make([]byte, 0, len(data)-end+start+len(value))
	updated = append(updated, data[:start]...)
	updated = append(updated, value...)
	return append(updated, data[end:]...)
}

func writeValidated(path string, original, updated []byte, mode os.FileMode) error {
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
