package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Harness struct {
	Version      int           `json:"version"`
	Project      Project       `json:"project"`
	Architecture Architecture  `json:"architecture,omitempty"`
	Rules        []Rule        `json:"rules,omitempty"`
	Skills       []Skill       `json:"skills,omitempty"`
	QualityGates []QualityGate `json:"quality_gates,omitempty"`
}
type Project struct {
	Name      string   `json:"name"`
	Languages []string `json:"languages,omitempty"`
}
type Architecture struct {
	Styles []string `json:"styles,omitempty"`
}
type Scope struct {
	Paths []string `json:"paths,omitempty"`
}
type Evidence struct {
	File      string `json:"file" yaml:"file"`
	Kind      string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Symbol    string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Quote     string `json:"quote,omitempty" yaml:"quote,omitempty"`
	StartLine int    `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty" yaml:"end_line,omitempty"`
	SHA256    string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	Revision  string `json:"revision,omitempty" yaml:"revision"`
}
type Rule struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Scope       Scope         `json:"scope,omitempty"`
	Origin      string        `json:"origin"`
	Status      string        `json:"status"`
	Evidence    []Evidence    `json:"evidence,omitempty"`
	Review      *ReviewRecord `json:"review,omitempty" yaml:"review,omitempty"`
}

// ReviewRecord identifies the exact rule and evidence examined by the reviewer.
type ReviewRecord struct {
	ContentSHA256  string `json:"content_sha256" yaml:"content_sha256"`
	EvidenceSHA256 string `json:"evidence_sha256" yaml:"evidence_sha256"`
}
type Skill struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Path        string     `json:"path,omitempty"`
	Status      string     `json:"status,omitempty"`
	Evidence    []Evidence `json:"evidence,omitempty"`
}
type QualityGate struct {
	ID         string   `json:"id" yaml:"id"`
	Command    string   `json:"command" yaml:"command"`
	Workspace  string   `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Workspaces []string `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
}

func (h Harness) Validate() error {
	if h.Version != 1 && h.Version != 2 {
		return fmt.Errorf("version: expected 1 or 2")
	}
	if strings.TrimSpace(h.Project.Name) == "" {
		return fmt.Errorf("project.name: must not be empty")
	}
	if err := nonemptyList("project.languages", h.Project.Languages); err != nil {
		return err
	}
	if err := nonemptyList("architecture.styles", h.Architecture.Styles); err != nil {
		return err
	}
	ids := map[string]bool{}
	for i, r := range h.Rules {
		field := fmt.Sprintf("rules[%d]", i)
		if err := uniqueID(field, r.ID, ids); err != nil {
			return err
		}
		if strings.TrimSpace(r.Description) == "" {
			return fmt.Errorf("%s.description: must not be empty", field)
		}
		if r.Origin != "human" && r.Origin != "ai" {
			return fmt.Errorf("%s.origin: expected human or ai", field)
		}
		if r.Status != "candidate" && r.Status != "approved" && r.Status != "rejected" {
			return fmt.Errorf("%s.status: expected candidate, approved or rejected", field)
		}
		if r.Origin == "ai" && len(r.Evidence) == 0 {
			return fmt.Errorf("%s.evidence: AI rules require evidence", field)
		}
		if err := nonemptyList(field+".scope.paths", r.Scope.Paths); err != nil {
			return err
		}
		for j, e := range r.Evidence {
			if strings.TrimSpace(e.File) == "" {
				return fmt.Errorf("%s.evidence[%d].file: must not be empty", field, j)
			}
			if e.Workspace != "" && strings.TrimSpace(e.Workspace) == "" {
				return fmt.Errorf("%s.evidence[%d].workspace: must not be blank", field, j)
			}
			if e.StartLine < 0 || e.EndLine < 0 || (e.EndLine > 0 && e.StartLine > e.EndLine) {
				return fmt.Errorf("%s.evidence[%d]: invalid line range", field, j)
			}
			if e.SHA256 != "" && (len(e.SHA256) != 64 || !isHex(e.SHA256)) {
				return fmt.Errorf("%s.evidence[%d].sha256: expected 64 hexadecimal characters", field, j)
			}
		}
	}
	ids = map[string]bool{}
	for i, s := range h.Skills {
		field := fmt.Sprintf("skills[%d]", i)
		if err := uniqueID(field, s.ID, ids); err != nil {
			return err
		}
		if strings.TrimSpace(s.Description) == "" {
			return fmt.Errorf("%s.description: must not be empty", field)
		}
		if s.Status != "" && s.Status != "approved" {
			return fmt.Errorf("%s.status: expected approved when present", field)
		}
		for j, e := range s.Evidence {
			if strings.TrimSpace(e.File) == "" {
				return fmt.Errorf("%s.evidence[%d].file: must not be empty", field, j)
			}
			if e.Workspace != "" && strings.TrimSpace(e.Workspace) == "" {
				return fmt.Errorf("%s.evidence[%d].workspace: must not be blank", field, j)
			}
			if e.StartLine < 0 || e.EndLine < 0 || (e.EndLine > 0 && e.StartLine > e.EndLine) {
				return fmt.Errorf("%s.evidence[%d]: invalid line range", field, j)
			}
			if e.SHA256 != "" && (len(e.SHA256) != 64 || !isHex(e.SHA256)) {
				return fmt.Errorf("%s.evidence[%d].sha256: expected 64 hexadecimal characters", field, j)
			}
		}
	}
	ids = map[string]bool{}
	for i, g := range h.QualityGates {
		field := fmt.Sprintf("quality_gates[%d]", i)
		if err := uniqueID(field, g.ID, ids); err != nil {
			return err
		}
		if strings.TrimSpace(g.Command) == "" {
			return fmt.Errorf("%s.command: must not be empty", field)
		}
		if g.Workspace != "" && strings.TrimSpace(g.Workspace) == "" {
			return fmt.Errorf("%s.workspace: must not be blank", field)
		}
		if err := nonemptyList(field+".workspaces", g.Workspaces); err != nil {
			return err
		}
		if g.Workspace != "" {
			if len(g.Workspaces) == 0 || g.Workspaces[0] != g.Workspace {
				return fmt.Errorf("%s: workspace must be represented in workspaces", field)
			}
		}
	}
	return nil
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}
func uniqueID(field, id string, ids map[string]bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s.id: must not be empty", field)
	}
	if ids[id] {
		return fmt.Errorf("%s.id: duplicate %q", field, id)
	}
	ids[id] = true
	return nil
}
func nonemptyList(field string, values []string) error {
	for i, v := range values {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s[%d]: must not be empty", field, i)
		}
	}
	return nil
}

// ReviewRule controls explicit review transitions. Reopening is required before reversing a decision.
func (h *Harness) ReviewRule(id, status string) error {
	transitions := map[string]map[string]bool{
		"candidate": {"approved": true, "rejected": true},
		"approved":  {"candidate": true}, "rejected": {"candidate": true},
	}
	for i := range h.Rules {
		rule := &h.Rules[i]
		if rule.ID != id {
			continue
		}
		if rule.Status == status {
			return nil
		}
		if !transitions[rule.Status][status] {
			return fmt.Errorf("rule %s: invalid transition %s -> %s; reopen as candidate first", id, rule.Status, status)
		}
		rule.Status = status
		return nil
	}
	return fmt.Errorf("rule %q not found", id)
}

// RuleContentHash hashes the reviewable rule content, excluding its decision record.
func RuleContentHash(rule Rule) (string, error) {
	rule.Review = nil
	rule.Status = ""
	data, err := json.Marshal(rule)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
