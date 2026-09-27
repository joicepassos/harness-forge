package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"harnessforge/internal/drift/application"
	"harnessforge/internal/drift/domain"
	driftinfra "harnessforge/internal/drift/infrastructure"
	harnessdomain "harnessforge/internal/harness/domain"
	"harnessforge/internal/harness/infrastructure"
)

func newDriftCommand() *cobra.Command {
	var repository string
	var layout string
	command := &cobra.Command{Use: "drift [file]", Short: "Report approved Harness rule and Forge knowledge evidence drift", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) == 1 {
			path = args[0]
		}
		if path == "" {
			project, err := infrastructure.LoadProject(repository, layout)
			if err != nil {
				return err
			}
			if project.Manifest != nil {
				reader, err := driftinfra.NewFileReader(repository)
				if err != nil {
					return err
				}
				defer reader.Close()
				report, err := application.NewDetect(staticHarnessLoader{project.Harness}, reader).Execute(cmd.Context(), project.Layout.ManifestPath)
				if err != nil {
					return err
				}
				if err := appendForgeKnowledgeDrift(repository, project.Layout, *project.Manifest, &report); err != nil {
					return err
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			}
			path = project.Layout.HarnessPath
		}
		reader, err := driftinfra.NewFileReader(repository)
		if err != nil {
			return err
		}
		report, err := application.NewDetect(infrastructure.YAMLLoader{}, reader).Execute(cmd.Context(), path)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}}
	command.Flags().StringVar(&repository, "repository", ".", "Repository to inspect without modifying it")
	command.Flags().StringVar(&layout, "layout", "", "Project layout to inspect (harness or forge); inferred when unambiguous")
	return command
}

type staticHarnessLoader struct{ harness harnessdomain.Harness }

func (l staticHarnessLoader) Load(string) (harnessdomain.Harness, error) { return l.harness, nil }

func appendForgeKnowledgeDrift(root string, layout infrastructure.ProjectLayout, manifest harnessdomain.Manifest, report *domain.Report) error {
	handle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer handle.Close()
	for _, ref := range manifest.References.Knowledge {
		absolute, err := layout.ResolveReference(ref.Path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(layout.Root, absolute)
		if err != nil {
			return err
		}
		data, readErr := infrastructure.ReadProjectFile(handle, filepath.ToSlash(relative), 2<<20)
		occurrence := domain.Occurrence{SubjectType: "knowledge", RuleID: "knowledge:" + ref.ID, Status: domain.StatusNotEvaluated, EvidenceStatus: domain.StatusNotEvaluated, Conformance: domain.StatusNotEvaluated, Locations: []string{ref.Path}}
		if readErr != nil {
			occurrence.Explanations = []string{"Knowledge item could not be evaluated: " + readErr.Error()}
			report.Occurrences = append(report.Occurrences, occurrence)
			report.Coverage.Total++
			report.Coverage.NotEvaluated++
			continue
		}
		item, parseErr := parseForgeKnowledgeDocument(data)
		if parseErr != nil || item.ID != ref.ID {
			if parseErr == nil {
				parseErr = fmt.Errorf("manifest ID %q does not match document ID %q", ref.ID, item.ID)
			}
			occurrence.Explanations = []string{"Knowledge item could not be evaluated: " + parseErr.Error()}
			report.Occurrences = append(report.Occurrences, occurrence)
			report.Coverage.Total++
			report.Coverage.NotEvaluated++
			continue
		}
		if item.Review != harnessdomain.KnowledgeApproved {
			continue
		}
		if len(item.Evidence) > 0 {
			occurrence.EvidenceStatus = domain.EvidencePresent
		}
		if item.Health == harnessdomain.KnowledgeStale || item.Health == harnessdomain.KnowledgeMissing || item.ContentSHA256 != harnessdomain.HashKnowledgeContent(item.Content) {
			occurrence.Status = domain.StatusDifference
			occurrence.Explanations = append(occurrence.Explanations, "Approved knowledge content or recorded health differs from its reviewed state.")
		}
		fingerprint, fingerprintErr := infrastructure.KnowledgeFingerprint(root, ref.Path, ref.ID, item.Evidence)
		if fingerprintErr != nil && errors.Is(fingerprintErr, os.ErrNotExist) {
			occurrence.Status = domain.StatusDifference
			occurrence.EvidenceStatus = domain.EvidenceMissing
			occurrence.Explanations = append(occurrence.Explanations, "Reviewed knowledge evidence file is missing.")
		} else if fingerprintErr != nil && (strings.Contains(fingerprintErr.Error(), "not found") || strings.Contains(fingerprintErr.Error(), "hash mismatch")) {
			occurrence.Status = domain.StatusDifference
			occurrence.EvidenceStatus = domain.EvidenceChanged
			occurrence.Explanations = append(occurrence.Explanations, "Reviewed knowledge evidence content changed: "+fingerprintErr.Error())
		} else if fingerprintErr != nil {
			if occurrence.Status != domain.StatusDifference {
				occurrence.Status = domain.StatusNotEvaluated
			}
			occurrence.EvidenceStatus = domain.StatusNotEvaluated
			occurrence.Explanations = append(occurrence.Explanations, "Reviewed knowledge evidence could not be evaluated: "+fingerprintErr.Error())
		} else if !strings.EqualFold(fingerprint, item.EvidenceSHA256) {
			occurrence.Status = domain.StatusDifference
			occurrence.EvidenceStatus = domain.EvidenceChanged
			occurrence.Explanations = append(occurrence.Explanations, "Reviewed knowledge evidence fingerprint changed.")
		} else if occurrence.Status != domain.StatusDifference && len(item.Evidence) > 0 {
			occurrence.Status = domain.StatusAligned
			occurrence.Explanations = append(occurrence.Explanations, "Reviewed knowledge and evidence fingerprints are unchanged; this does not establish that the knowledge conforms to the codebase.")
		} else if occurrence.Status != domain.StatusDifference {
			occurrence.Status = domain.StatusNotEvaluated
			occurrence.Explanations = append(occurrence.Explanations, "This approved knowledge item has no evidence to compare.")
		}
		occurrence.Explanations = append(occurrence.Explanations, "Evidence availability or unchanged fingerprints are not proof of rule conformance; conformance remains not_evaluated.")
		report.Occurrences = append(report.Occurrences, occurrence)
		report.Coverage.Total++
		if occurrence.Status == domain.StatusNotEvaluated {
			report.Coverage.NotEvaluated++
		} else {
			report.Coverage.Evaluated++
		}
	}
	report.Limitations = append(report.Limitations, "Forge knowledge drift checks only approved item content and recorded evidence fingerprints; they do not evaluate semantic conformance.")
	return nil
}
