package infrastructure

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type YAMLRuleStore struct{ YAMLLoader }

// SaveStatus edits only the scalar's byte span; comments, whitespace and CRLF remain untouched.
func (s YAMLRuleStore) SaveStatus(path, id, previous, status string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("harness must be a regular file for editing")
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("lock harness: %w", err)
	}
	defer os.Remove(path + ".lock")
	defer lock.Close()
	h, err := s.Load(path)
	if err != nil {
		return err
	}
	found := false
	for _, r := range h.Rules {
		if r.ID == id {
			found = true
			if r.Status != previous {
				return fmt.Errorf("rule changed during review; reload and retry")
			}
		}
	}
	if !found {
		return fmt.Errorf("rule not found")
	}
	if err := h.ReviewRule(id, status); err != nil {
		return err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(original, &document); err != nil {
		return err
	}
	if len(document.Content) != 1 {
		return fmt.Errorf("expected one YAML document")
	}
	rules := mappingValue(document.Content[0], "rules")
	if rules == nil || rules.Kind != yaml.SequenceNode {
		return fmt.Errorf("rules must be an explicit YAML sequence for editing")
	}
	var target *yaml.Node
	for _, rule := range rules.Content {
		key := mappingValue(rule, "id")
		if key != nil && key.Value == id {
			target = mappingValue(rule, "status")
			break
		}
	}
	if target == nil || target.Kind != yaml.ScalarNode || target.Anchor != "" || target.Style&(yaml.LiteralStyle|yaml.FoldedStyle|yaml.TaggedStyle) != 0 {
		return fmt.Errorf("status must be a plain or quoted scalar without anchors for editing")
	}
	if target.Value != previous {
		return fmt.Errorf("rule changed during review")
	}
	start, err := scalarOffset(original, target.Line, target.Column)
	if err != nil {
		return err
	}
	old := previous
	replacement := status
	switch target.Style {
	case yaml.SingleQuotedStyle:
		old = "'" + previous + "'"
		replacement = "'" + status + "'"
	case yaml.DoubleQuotedStyle:
		old = "\"" + previous + "\""
		replacement = "\"" + status + "\""
	}
	if !bytes.HasPrefix(original[start:], []byte(old)) {
		return fmt.Errorf("status uses escaped or unsupported syntax; edit it manually")
	}
	updated := append(append(append([]byte{}, original[:start]...), []byte(replacement)...), original[start+len(old):]...)
	info, err = os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "harness-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(updated); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	parsed, err := s.Load(tmp.Name())
	if err != nil {
		return err
	}
	if err := parsed.Validate(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("harness changed during review; reload and retry")
	}
	return os.Rename(tmp.Name(), path)
}
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func scalarOffset(data []byte, line, column int) (int, error) {
	offset := 0
	for i := 1; i < line; i++ {
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 {
			return 0, fmt.Errorf("invalid YAML location")
		}
		offset += end + 1
	}
	for i := 1; i < column; i++ {
		_, size := utf8.DecodeRune(data[offset:])
		offset += size
	}
	if offset >= len(data) || strings.ContainsAny(string(data[offset:offset+1]), "\r\n") {
		return 0, fmt.Errorf("invalid scalar location")
	}
	return offset, nil
}
