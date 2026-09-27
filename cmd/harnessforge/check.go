package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/gates"
	generation "harnessforge/internal/generation/infrastructure"
	harnessdomain "harnessforge/internal/harness/domain"
	harnessinfra "harnessforge/internal/harness/infrastructure"
)

type projectCheckError struct{}

func (projectCheckError) Error() string     { return "project checks failed" }
func (projectCheckError) CheckFailed() bool { return true }

type checkDiagnostic struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Path       string `json:"path,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

func validateForgeKnowledgeHealth(ctx context.Context, root string, manifest harnessdomain.Manifest) error {
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()
	layout, err := harnessinfra.ResolveLayout(root, "forge")
	if err != nil {
		return err
	}
	for _, ref := range manifest.References.Knowledge {
		if _, err := layout.ResolveReference(ref.Path); err != nil {
			return err
		}
		data, err := readProjectFile(rootHandle, ref.Path, 2<<20)
		if err != nil {
			return fmt.Errorf("knowledge %q: %w", ref.ID, err)
		}
		item, err := parseForgeKnowledgeDocument(data)
		if err != nil {
			return fmt.Errorf("knowledge %q: %w", ref.ID, err)
		}
		if item.ID != ref.ID {
			return fmt.Errorf("knowledge reference %q points to item %q", ref.ID, item.ID)
		}
		if item.Review != harnessdomain.KnowledgeApproved {
			continue
		}
		if item.ContentSHA256 != harnessdomain.HashKnowledgeContent(item.Content) {
			return fmt.Errorf("knowledge %q content changed since review", ref.ID)
		}
		if item.Health == harnessdomain.KnowledgeStale || item.Health == harnessdomain.KnowledgeMissing {
			continue
		}
		fingerprint, err := harnessinfra.KnowledgeFingerprint(root, ref.Path, ref.ID, item.Evidence)
		if err != nil {
			return fmt.Errorf("knowledge %q evidence: %w", ref.ID, err)
		}
		if !strings.EqualFold(fingerprint, item.EvidenceSHA256) {
			return fmt.Errorf("knowledge %q evidence changed since review", ref.ID)
		}
	}
	return nil
}

func parseForgeKnowledgeDocument(data []byte) (harnessdomain.KnowledgeItem, error) {
	var item harnessdomain.KnowledgeItem
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "---") {
		return item, fmt.Errorf("expected YAML front matter")
	}
	parts := strings.SplitN(strings.TrimPrefix(text, "---"), "---", 2)
	if len(parts) != 2 {
		return item, fmt.Errorf("unterminated YAML front matter")
	}
	dec := yaml.NewDecoder(bytes.NewBufferString(strings.TrimSpace(parts[0])))
	dec.KnownFields(true)
	if err := dec.Decode(&item); err != nil {
		return item, err
	}
	if err := item.Validate(); err != nil {
		return item, err
	}
	return item, nil
}

func readProjectFile(root *os.Root, relative string, limit int64) ([]byte, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") || strings.Contains(relative, ":") {
		return nil, fmt.Errorf("expected a repository-relative path")
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("path escapes project root")
	}
	current := "."
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := root.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink path component is not allowed")
		}
		if current != clean && !info.IsDir() {
			return nil, fmt.Errorf("parent path component is not a directory")
		}
		if current == clean && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("expected regular file")
		}
	}
	f, err := root.Open(clean)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds size limit")
	}
	return data, nil
}

type checkEnvelope struct {
	Version     int               `json:"version"`
	OK          bool              `json:"ok"`
	Diagnostics []checkDiagnostic `json:"diagnostics"`
	GateResults []gates.Result    `json:"gate_results,omitempty"`
}

func newCheckCommand() *cobra.Command {
	var repository, layout, format string
	var runGates bool
	var gateTimeout time.Duration
	cmd := &cobra.Command{Use: "check", Short: "Validate project configuration, evidence and generated outputs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		root, err := filepath.Abs(repository)
		if err != nil {
			return err
		}
		project, err := harnessinfra.LoadProject(root, layout)
		env := checkEnvelope{Version: 1, OK: true, Diagnostics: []checkDiagnostic{}}
		if err != nil {
			env.OK = false
			env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "project.layout_invalid", Severity: "error", Message: err.Error()})
		} else {
			valid := true
			if project.Manifest != nil {
				if err := harnessinfra.ValidateManifestReferences(project.Layout, *project.Manifest); err != nil {
					env.OK = false
					valid = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "project.references_invalid", Severity: "error", Message: err.Error(), Path: project.Layout.ManifestPath})
				}
			}
			if project.Layout.Kind == harnessinfra.LayoutHarness {
				if err := harnessinfra.CheckEvidence(cmd.Context(), root, project.Harness); err != nil {
					env.OK = false
					valid = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "project.evidence_invalid", Severity: "error", Message: err.Error(), Path: project.Layout.HarnessPath})
				}
				if _, err := generation.CheckLegacyForgeOutputs(cmd.Context(), root); err != nil {
					env.OK = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "generated.drift", Severity: "error", Message: err.Error(), Suggestion: "Run harnessforge generate after reviewing the approved Harness rules."})
				}
			} else if err := validateForgeKnowledgeHealth(cmd.Context(), root, *project.Manifest); err != nil {
				env.OK = false
				env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "project.knowledge_invalid", Severity: "error", Message: err.Error(), Path: project.Layout.ManifestPath})
			}
			if project.Layout.Kind == harnessinfra.LayoutForge {
				if _, err := generation.CheckForge(cmd.Context(), root); err != nil {
					env.OK = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "generated.drift", Severity: "error", Message: err.Error(), Path: filepath.Join(root, ".forge", "generated-manifest.json"), Suggestion: "Run harnessforge sync --apply after reviewing the planned output."})
				}
			}
			for _, policy := range projectPolicies(project) {
				if policy.Capability == "enforced" {
					env.OK = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "policy.unsupported_enforcement", Severity: "error", Message: policy.ID + " is marked enforced, but its executor is not implemented."})
				}
			}
			if runGates && valid {
				var quality []harnessdomain.QualityGate
				if project.Manifest != nil {
					quality = project.Manifest.QualityGates
				} else {
					quality = project.Harness.QualityGates
				}
				items := make([]gates.Gate, 0, len(quality))
				for _, g := range quality {
					items = append(items, gates.Gate{ID: g.ID, Command: g.Command, Workspace: g.Workspace, Workspaces: g.Workspaces})
				}
				if len(items) == 0 {
					env.OK = false
					env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "gate.none_declared", Severity: "error", Message: "--run-gates was requested but no quality gates are declared."})
				}
				if hasUnsupportedEnforcement(project) {
					for _, policy := range projectPolicies(project) {
						if policy.Capability == "enforced" {
							env.OK = false
							env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "policy.unsupported_enforcement", Severity: "error", Message: policy.ID + " requests enforcement through an executor that check does not implement."})
						}
					}
				}
				if len(items) > 0 && !hasUnsupportedEnforcement(project) {
					results, err := gates.Run(cmd.Context(), root, items, gateTimeout)
					if err != nil {
						return err
					}
					env.GateResults = results
					for _, result := range results {
						if result.Status != "passed" {
							env.OK = false
							env.Diagnostics = append(env.Diagnostics, checkDiagnostic{Code: "gate.failed", Severity: "error", Message: result.ID + " failed in " + result.Workspace + ": " + result.Error, Suggestion: "Review the gate command, workspace, output, and timeout before retrying."})
						}
					}
				}
			}
		}
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(env); err != nil {
				return err
			}
		} else if format == "text" {
			if env.OK {
				fmt.Fprintln(cmd.OutOrStdout(), "check passed")
			} else {
				for _, d := range env.Diagnostics {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", d.Code, d.Message)
				}
			}
		} else {
			return fmt.Errorf("unsupported format %q", format)
		}
		if !env.OK {
			return projectCheckError{}
		}
		return nil
	}}
	cmd.Flags().StringVar(&repository, "repository", ".", "Repository to check without modifying it")
	cmd.Flags().StringVar(&layout, "layout", "", "Select harness or forge when both layouts exist")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&runGates, "run-gates", false, "Explicitly execute declared quality gates")
	cmd.Flags().DurationVar(&gateTimeout, "gate-timeout", gates.DefaultTimeout, "Maximum runtime per quality gate (requires --run-gates)")
	return cmd
}

func projectPolicies(project harnessinfra.ProjectConfig) []harnessdomain.Policy {
	if project.Manifest == nil {
		return nil
	}
	return project.Manifest.Policies
}

func hasUnsupportedEnforcement(project harnessinfra.ProjectConfig) bool {
	for _, p := range projectPolicies(project) {
		if p.Capability == "enforced" {
			return true
		}
	}
	return false
}
