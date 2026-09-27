package infrastructure

import (
	"bytes"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/safefile"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type YAMLRuleStore struct{ YAMLLoader }

// SaveStatus edits only the scalar's byte span; comments, whitespace and CRLF remain untouched.
func (s YAMLRuleStore) SaveStatus(path, id, previous, status string, record *domain.ReviewRecord) error {
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
			if status == "approved" {
				if record == nil {
					return fmt.Errorf("approval requires review hashes")
				}
				contentHash, err := domain.RuleContentHash(r)
				if err != nil {
					return err
				}
				if contentHash != record.ContentSHA256 {
					return fmt.Errorf("rule or evidence changed during review; reload and retry")
				}
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
			if status == "approved" {
				if record == nil {
					return fmt.Errorf("approval requires review hashes")
				}
				review := mappingValue(rule, "review")
				if review == nil {
					review = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
					rule.Content = append(rule.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "review"}, review)
				}
				if review.Kind != yaml.MappingNode {
					return fmt.Errorf("review must be a mapping for editing")
				}
				setScalar(review, "content_sha256", record.ContentSHA256)
				setScalar(review, "evidence_sha256", record.EvidenceSHA256)
			}
			if status == "candidate" {
				removeMappingValue(rule, "review")
			}
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
	if status == "approved" {
		updated, err = replaceRuleWithReview(updated, id, record)
		if err != nil {
			return err
		}
	}
	if status == "candidate" {
		updated, err = removeRuleReview(updated, id)
		if err != nil {
			return err
		}
	}
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
	return safefile.Replace(tmp.Name(), path)
}
func replaceRuleWithReview(data []byte, id string, record *domain.ReviewRecord) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	for _, rule := range mappingValue(doc.Content[0], "rules").Content {
		key := mappingValue(rule, "id")
		if key == nil || key.Value != id {
			continue
		}
		if record == nil {
			return nil, fmt.Errorf("approval hashes missing")
		}
		status := mappingValue(rule, "status")
		if status == nil {
			return nil, fmt.Errorf("rule status missing")
		}
		lineStart := 0
		for n := 1; n < status.Line; n++ {
			nl := bytes.IndexByte(data[lineStart:], '\n')
			if nl < 0 {
				return nil, fmt.Errorf("invalid status line")
			}
			lineStart += nl + 1
		}
		indent := 0
		for lineStart+indent < len(data) && data[lineStart+indent] == ' ' {
			indent++
		}
		insertAt := len(data)
		rules := mappingValue(doc.Content[0], "rules")
		// Keep the new block alongside the rule by inserting before the next sequence item.
		for _, sibling := range rules.Content {
			key := mappingValue(sibling, "id")
			if key != nil && key.Value != id && sibling.Line > rule.Line {
				insertAt = 0
				for n := 1; n < sibling.Line; n++ {
					nl := bytes.IndexByte(data[insertAt:], '\n')
					if nl < 0 {
						return nil, fmt.Errorf("invalid rule line")
					}
					insertAt += nl + 1
				}
				break
			}
		}
		prefix := ""
		if insertAt > 0 && data[insertAt-1] != '\n' {
			prefix = "\n"
		}
		block := fmt.Sprintf("%s%*sreview:\n%*s  content_sha256: %s\n%*s  evidence_sha256: %s\n", prefix, indent, "", indent, "", record.ContentSHA256, indent, "", record.EvidenceSHA256)
		return append(append(append([]byte(nil), data[:insertAt]...), []byte(block)...), data[insertAt:]...), nil
	}
	return nil, fmt.Errorf("rule not found")
}
func removeRuleReview(data []byte, id string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	rules := mappingValue(doc.Content[0], "rules")
	for _, rule := range rules.Content {
		key := mappingValue(rule, "id")
		if key == nil || key.Value != id {
			continue
		}
		review := mappingValue(rule, "review")
		if review == nil {
			return data, nil
		}
		line := review.Line
		for i := 0; i+1 < len(rule.Content); i += 2 {
			if rule.Content[i].Value == "review" {
				line = rule.Content[i].Line
				break
			}
		}
		start := 0
		for n := 1; n < line; n++ {
			nl := bytes.IndexByte(data[start:], '\n')
			if nl < 0 {
				return nil, fmt.Errorf("invalid review location")
			}
			start += nl + 1
		}
		lastLine := yamlNodeLastLine(review)
		end := start
		for n := line; n <= lastLine; n++ {
			nl := bytes.IndexByte(data[end:], '\n')
			if nl < 0 {
				end = len(data)
				break
			}
			end += nl + 1
		}
		return append(append([]byte(nil), data[:start]...), data[end:]...), nil
	}
	return data, nil
}

func yamlNodeLastLine(node *yaml.Node) int {
	last := node.Line
	for _, child := range node.Content {
		if childLine := yamlNodeLastLine(child); childLine > last {
			last = childLine
		}
	}
	return last
}
func setScalar(mapping *yaml.Node, key, value string) {
	if node := mappingValue(mapping, key); node != nil {
		node.Value = value
		node.Tag = "!!str"
		return
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}
func removeMappingValue(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
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
