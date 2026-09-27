package application

import (
	"context"
	"harnessforge/internal/generation/domain"
	harnessdomain "harnessforge/internal/harness/domain"
)

type Loader interface {
	Load(string) (harnessdomain.Harness, error)
}
type Adapter interface {
	Render(domain.Input) (domain.Document, error)
}
type Writer interface {
	Write(context.Context, string, domain.Document) error
}
type Generate struct {
	loader  Loader
	adapter Adapter
	writer  Writer
}

func NewGenerate(l Loader, a Adapter, w Writer) *Generate { return &Generate{l, a, w} }
func (g *Generate) Execute(ctx context.Context, harnessPath, repository string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h, err := g.loader.Load(harnessPath)
	if err != nil {
		return err
	}
	input := domain.Input{Project: h.Project.Name}
	for _, r := range h.Rules {
		if r.Status == "approved" {
			input.Rules = append(input.Rules, domain.Rule{ID: r.ID, Description: r.Description, Paths: r.Scope.Paths})
		}
	}
	for _, skill := range h.Skills {
		if skill.Status == "" || skill.Status == "approved" {
			input.Skills = append(input.Skills, domain.Skill{ID: skill.ID, Description: skill.Description, Path: skill.Path})
		}
	}
	for _, gate := range h.QualityGates {
		input.Gates = append(input.Gates, domain.QualityGate{ID: gate.ID, Command: gate.Command, Workspace: gate.Workspace, Workspaces: append([]string(nil), gate.Workspaces...)})
	}
	document, err := g.adapter.Render(input)
	if err != nil {
		return err
	}
	return g.writer.Write(ctx, repository, document)
}
