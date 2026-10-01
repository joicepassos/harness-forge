package main

import (
	"bufio"
	"bytes"
	"context"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/harness"
	harnessinfra "harnessforge/internal/harness/infrastructure"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGuidedInitLocalPlanRequiresConfirmation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("A small Go service.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/service\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	noAI := func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		t.Fatal("unexpected AI call")
		return setupAIProposal{}, nil
	}
	if err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\nn\n"), &output, root, noAI); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Proposed setup") || !strings.Contains(output.String(), "Go modules") {
		t.Fatalf("missing analysis or preview: %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".harness")); !os.IsNotExist(err) {
		t.Fatalf("files changed before confirmation: %v", err)
	}
	output.Reset()
	if err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\ny\n"), &output, root, noAI); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte(filepath.Base(root))) {
		t.Fatalf("missing project name: %s", content)
	}
}

func TestGuidedInitUsesDocumentsNotesAndAIProposal(t *testing.T) {
	root := t.TempDir()
	readme := "The service uses a controller and a repository layer.\n"
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "test-key-not-to-save")
	called := false
	propose := func(_ context.Context, config setupProvider, _ *analyzer.Analysis, documents []setupDocument, notes string) (setupAIProposal, error) {
		called = true
		if config.Name != "openai" || config.Model == "" || notes != "Team uses ports and adapters" || len(documents) != 1 {
			t.Fatalf("incomplete context: %#v %#v %q", config, documents, notes)
		}
		citation := setupCitation{Source: "repository-file:README.md", Quote: "controller and a repository layer"}
		return setupAIProposal{Summary: "A layered service.", Architecture: []string{"layered"}, Rules: []setupRule{{ID: "keep-layers", Description: "Keep controller and repository responsibilities separate.", Evidence: []setupCitation{citation}}}, Skills: []setupSkill{{ID: "add-feature", Description: "Add a feature across the service layers.", Steps: []string{"Update the controller.", "Update the repository."}, Evidence: []setupCitation{citation}}}}, nil
	}
	var output bytes.Buffer
	input := "\n\ny\nopenai\n\n\nTeam uses ports and adapters\n\ny\n3\ny\n"
	if err := runGuidedInit(context.Background(), strings.NewReader(input), &output, root, propose); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("AI proposer was not called")
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", ".harness/harness.yaml", ".harness/skills/add-feature/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("test-key-not-to-save")) || !bytes.Contains(data, []byte("keep-layers")) {
		t.Fatalf("invalid harness: %s", data)
	}
	if _, err := (harnessinfra.YAMLLoader{}).Load(filepath.Join(root, ".harness", "harness.yaml")); err != nil {
		t.Fatalf("generated harness failed schema validation: %v", err)
	}
	configured, err := projectAIConfig(root)
	if err != nil || configured.Provider != "openai" || configured.Model == "" {
		t.Fatalf("project AI configuration not available to later commands: %#v %v", configured, err)
	}
	agent, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(agent, []byte("Team uses ports and adapters")) || !bytes.Contains(agent, []byte("add-feature")) {
		t.Fatalf("missing generated context: %s", agent)
	}
}

func TestGuidedInitPreservesManualAgentFile(t *testing.T) {
	root := t.TempDir()
	if _, err := harness.Init(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte("Manual instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	var output bytes.Buffer
	err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\nn\n"), &output, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "append to existing instructions") {
		t.Fatalf("append was not previewed: %s", output.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("starter harness changed")
	}
	manual, _ := os.ReadFile(path)
	if string(manual) != "Manual instructions" {
		t.Fatalf("manual instructions changed before confirmation: %q", manual)
	}
	output.Reset()
	if err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\ny\n"), &output, root, nil); err != nil {
		t.Fatal(err)
	}
	manual, _ = os.ReadFile(path)
	if !bytes.HasPrefix(manual, []byte("Manual instructions")) || !bytes.Contains(manual, []byte(generatedSetupMarker)) {
		t.Fatalf("manual instructions lost: %q", manual)
	}
}

func TestGuidedInitReplacesOnlyStarterHarness(t *testing.T) {
	root := t.TempDir()
	if _, err := harness.Init(root); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\ny\n"), &output, root, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "replace starter/generated file") {
		t.Fatalf("starter replacement was not previewed: %s", output.String())
	}
	manual := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(manual, []byte("Manual instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	output.Reset()
	err := runGuidedInit(context.Background(), strings.NewReader("\n\n\n\n\n\ny\n"), &output, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "already has a configured harness") {
		t.Fatalf("missing existing setup guidance: %s", output.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, ".harness", "harness.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("configured harness changed")
	}
}

func TestSetupProposalRejectsUnsupportedEvidence(t *testing.T) {
	sources := map[string]string{"repository-file:README.md": "Use a controller for HTTP requests."}
	proposal := setupAIProposal{Rules: []setupRule{{ID: "controllers", Description: "Use controllers.", Evidence: []setupCitation{{Source: "repository-file:README.md", Quote: "invented text"}}}}}
	if err := validateSetupProposal(proposal, sources); err == nil {
		t.Fatal("unsupported quote accepted")
	}
}

func TestGuidedInitDoesNotAskForKeyWhenAIContextIsDeclined(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "")
	var output bytes.Buffer
	input := "\n\nn\n\n\n\nn\n"
	err := runGuidedInit(context.Background(), strings.NewReader(input), &output, root, func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		t.Fatal("AI called after context sharing was declined")
		return setupAIProposal{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "OPENAI_API_KEY") {
		t.Fatalf("key requested before consent: %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".harness")); !os.IsNotExist(err) {
		t.Fatalf("project changed after cancellation: %v", err)
	}
}

func TestSelectedContextAcceptsDirectoriesAndExternalFiles(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "team-docs")
	if err := os.Mkdir(docs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "architecture.md"), []byte("Keep the API separate from storage."), 0644); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "company.md")
	if err := os.WriteFile(external, []byte("The team reviews every API change."), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	session := setupSession{reader: bufio.NewReader(strings.NewReader("team-docs\n" + external + "\n\n")), output: &output}
	selected, err := session.selectedDocuments(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].Source != "repository-file:team-docs/architecture.md" || selected[1].Relative || !strings.Contains(selected[1].Text, "reviews every API") {
		t.Fatalf("unexpected selected context: %#v", selected)
	}
}

func TestChooseLanguagesSupportsMultipleSelectionsAndDefaults(t *testing.T) {
	var output bytes.Buffer
	session := setupSession{reader: bufio.NewReader(strings.NewReader("1,3,1\n")), output: &output}
	selected, err := session.chooseLanguages()
	if err != nil || !reflect.DeepEqual(selected, []string{"Go", "Python"}) {
		t.Fatalf("selected = %#v, err = %v", selected, err)
	}
	defaultSession := setupSession{reader: bufio.NewReader(strings.NewReader("\n")), output: &output}
	selected, err = defaultSession.chooseLanguages()
	if err != nil || !reflect.DeepEqual(selected, []string{"all detected"}) {
		t.Fatalf("default selected = %#v, err = %v", selected, err)
	}
}

func TestContextAllowsLinkedParentButRejectsProjectEscape(t *testing.T) {
	parent := t.TempDir()
	project := filepath.Join(parent, "project")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "README.md"), []byte("Project overview"), 0644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(parent, alias); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	linkedProject := filepath.Join(alias, "project")
	items, err := readSetupPath(context.Background(), linkedProject, "README.md")
	if err != nil || len(items) != 1 || items[0].Source != "repository-file:README.md" {
		t.Fatalf("linked parent was rejected: %v %#v", err, items)
	}
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "overview.md"), []byte("Outside document"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(project, "linked-docs")); err != nil {
		t.Skipf("directory links unavailable: %v", err)
	}
	if _, err := readSetupPath(context.Background(), project, filepath.Join("linked-docs", "overview.md")); err == nil {
		t.Fatal("linked directory escaped the project")
	}
}

func TestWriteSetupPlanRefusesChangesAfterPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(generatedSetupMarker+" original"), 0644); err != nil {
		t.Fatal(err)
	}
	files := []setupOutputFile{{Path: "AGENTS.md", Content: []byte("new")}}
	if err := classifySetupOutputs(root, files); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("edited while previewing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeSetupPlan(context.Background(), root, files); err == nil {
		t.Fatal("changed file was overwritten")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "edited while previewing" {
		t.Fatalf("manual change was lost: %q %v", content, err)
	}
}
