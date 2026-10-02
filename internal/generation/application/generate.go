package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
type EvidenceFingerprinter interface {
	Fingerprint(path, id string, evidence []harnessdomain.Evidence) (string, error)
}
type Generate struct {
	loader   Loader
	adapter  Adapter
	writer   Writer
	evidence EvidenceFingerprinter
}

func NewGenerate(l Loader, a Adapter, w Writer, checkers ...EvidenceFingerprinter) *Generate {
	g := &Generate{loader: l, adapter: a, writer: w}
	if len(checkers) > 0 {
		g.evidence = checkers[0]
	}
	return g
}
func (g *Generate) Execute(ctx context.Context, harnessPath, repository string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h, err := g.loader.Load(harnessPath)
	if err != nil {
		return err
	}
	input := domain.Input{
		Project:      h.Project.Name,
		Summary:      h.Context.Summary,
		Notes:        h.Context.Notes,
		Documents:    append([]string(nil), h.Context.Documents...),
		Architecture: append([]string(nil), h.Architecture.Styles...),
	}
	for _, r := range h.Rules {
		if r.Status == "approved" {
			if r.Review != nil {
				contentHash, err := harnessdomain.RuleContentHash(r)
				if err != nil {
					return err
				}
				if contentHash != r.Review.ContentSHA256 {
					return fmt.Errorf("approved rule %q changed after review; review it again", r.ID)
				}
				evidenceHash := sha256.Sum256([]byte("[]"))
				if len(r.Evidence) > 0 {
					if g.evidence == nil {
						return fmt.Errorf("cannot publish reviewed rule %q without revalidating evidence", r.ID)
					}
					fingerprint, err := g.evidence.Fingerprint(harnessPath, r.ID, r.Evidence)
					if err != nil {
						return fmt.Errorf("approved rule %q evidence is stale; review it again: %w", r.ID, err)
					}
					decoded, err := hex.DecodeString(fingerprint)
					if err != nil || len(decoded) != sha256.Size {
						return fmt.Errorf("approved rule %q has an invalid evidence fingerprint", r.ID)
					}
					if fingerprint != r.Review.EvidenceSHA256 {
						return fmt.Errorf("approved rule %q evidence changed after review; review it again", r.ID)
					}
				} else if hex.EncodeToString(evidenceHash[:]) != r.Review.EvidenceSHA256 {
					return fmt.Errorf("approved rule %q evidence changed after review; review it again", r.ID)
				}
			}
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
