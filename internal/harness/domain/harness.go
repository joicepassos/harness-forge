package domain

import (
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
	File     string `json:"file"`
	Symbol   string `json:"symbol,omitempty"`
	Revision string `json:"revision,omitempty"`
}
type Rule struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Scope       Scope      `json:"scope,omitempty"`
	Origin      string     `json:"origin"`
	Status      string     `json:"status"`
	Evidence    []Evidence `json:"evidence,omitempty"`
}
type Skill struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}
type QualityGate struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

func (h Harness) Validate() error {
	if h.Version != 1 {
		return fmt.Errorf("version: expected 1")
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
	}
	return nil
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
