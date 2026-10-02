package onboarding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	harnessinfra "harnessforge/internal/harness/infrastructure"
)

// Step is a safe, copyable action in the guided onboarding flow.
type Step struct {
	Title       string `json:"title"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description"`
}

// Guide describes how to start maintaining an already-existing project. It
// never changes project files; it only diagnoses layout selection and proposes
// the next explicit CLI actions.
type Guide struct {
	Repository string `json:"repository"`
	Status     string `json:"status"`
	Layout     string `json:"layout,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`
	Message    string `json:"message"`
	Steps      []Step `json:"steps"`
}

// BuildGuide inspects a project and returns a practical path through the
// existing check, review, and sync commands. A dual-layout project is a
// deliberate conflict unless the caller explicitly selects one.
func BuildGuide(repository, selection string) (Guide, error) {
	if selection != "" && selection != "harness" && selection != "forge" {
		return Guide{}, fmt.Errorf("unknown layout %q; choose harness or forge", selection)
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return Guide{}, err
	}
	guide := Guide{Repository: root, Status: "ready", Steps: []Step{}}
	harnessPath := filepath.Join(root, ".harness", "harness.yaml")
	forgePath := filepath.Join(root, ".forge", "forge.yaml")
	harnessExists, err := regularConfigExists(harnessPath)
	if err != nil {
		return Guide{}, err
	}
	forgeExists, err := regularConfigExists(forgePath)
	if err != nil {
		return Guide{}, err
	}
	if harnessExists && forgeExists && selection == "" {
		guide.Status = "conflict"
		guide.Message = "Both .harness/harness.yaml and .forge/forge.yaml exist. HarnessForge will not choose a source silently; select the layout you intend to maintain. No files were changed."
		guide.Steps = []Step{
			{Title: "Choose the Harness source", Command: "harnessforge onboard <repository> --layout harness", Description: "Select this only if .harness/harness.yaml is the source you intend to maintain."},
			{Title: "Choose the Forge source", Command: "harnessforge onboard <repository> --layout forge", Description: "Select this only if .forge/forge.yaml is the source you intend to maintain."},
		}
		return guide, nil
	}
	if !harnessExists && !forgeExists {
		if selection != "" {
			guide.Status = "conflict"
			guide.Message = fmt.Sprintf("The selected %s layout does not exist. No files were changed.", selection)
			guide.Steps = []Step{{Title: "Initialize or select an existing layout", Command: "harnessforge init", Description: "Create a Harness configuration, or rerun onboarding without --layout after an existing Forge configuration is available."}}
			return guide, nil
		}
		guide.Status = "not_initialized"
		guide.Message = "No HarnessForge configuration was found. The repository is unchanged; initialize only if you want to start managing it with HarnessForge."
		guide.Steps = []Step{{Title: "Create the initial configuration", Command: "harnessforge init", Description: "Creates .harness/harness.yaml without importing or approving repository content."}, {Title: "Continue onboarding", Command: "harnessforge onboard .", Description: "Re-run this guide after initialization to inspect the project and get the review and generation steps."}}
		return guide, nil
	}
	if selection == "" {
		if harnessExists {
			selection = "harness"
		} else {
			selection = "forge"
		}
	}
	project, err := harnessinfra.LoadProject(root, selection)
	if err != nil {
		guide.Status = "invalid"
		guide.Layout = selection
		guide.Message = "The selected project configuration could not be loaded. Fix the reported schema or reference issue before onboarding continues. No files were changed."
		guide.Steps = []Step{{Title: "Repair the selected configuration", Description: err.Error()}}
		return guide, nil
	}
	guide.Layout = selection
	if project.Layout.Kind == harnessinfra.LayoutHarness {
		guide.ConfigPath = filepath.Join(".harness", "harness.yaml")
		guide.Message = "Harness configuration found. Start with a read-only check, then inspect or propose rules and generate the current outputs. Imported or discovered material still needs human review."
		guide.Steps = []Step{
			{Title: "Check the existing project", Command: "harnessforge check --repository <repository> --layout harness", Description: "Validates the selected Harness source and reports generated-output drift without writing."},
			{Title: "Inspect existing rules", Command: "harnessforge validate --repository <repository> --layout harness", Description: "Review current rules, evidence, skills, and gates before adding more."},
			{Title: "Optionally propose evidence-backed rules", Command: "harnessforge discover propose <repository> <goal>", Description: "Requires a configured model provider. Inspect the JSON proposal and cited repository evidence; this command does not write."},
			{Title: "Store one proposal as a candidate", Command: "harnessforge discover apply <single-proposal-object> --repository <repository> --file <repository>/.harness/harness.yaml --approve", Description: "propose returns an array: select one object from that array and pass the object itself, not the whole array. Apply stores the rule as candidate; it does not approve it for export."},
			{Title: "Review the candidate", Command: "harnessforge review <rule-id> approved --repository <repository> --layout harness", Description: "Inspect the rule and cited evidence first. Use rejected to keep it out of generated instructions."},
			{Title: "Generate Codex instructions", Command: "harnessforge generate codex --repository <repository> --layout harness", Description: "Run only after review; generated files may report a conflict if they were edited manually."},
			{Title: "Generate Claude Code instructions", Command: "harnessforge generate claude --repository <repository> --layout harness", Description: "Use when Claude Code is a project target. Resolve any file conflict explicitly before retrying."},
		}
	} else {
		guide.ConfigPath = filepath.Join(".forge", "forge.yaml")
		guide.Message = "Forge configuration found. Check it first, inspect local observations, review candidates explicitly, then preview sync before applying generated files. Candidate knowledge is never published by onboarding."
		guide.Steps = []Step{
			{Title: "Check configuration, evidence, and generated files", Command: "harnessforge check --repository <repository> --layout forge", Description: "Read-only validation. Resolve any reported conflict before continuing."},
			{Title: "Import an existing project document", Command: "harnessforge onboard import <path> --kind <fact|convention|business_rule|decision|constraint> --repository <repository>", Description: "Imports a selected UTF-8 text file as a candidate and reports its stable ID. Import does not execute or approve its contents."},
			{Title: "Review imported knowledge", Command: "harnessforge review <candidate-id> approved --reviewer <name> --repository <repository> --layout forge", Description: "Use the ID printed by import. Inspect the document and evidence first; use rejected to keep it out of generated instructions."},
			{Title: "Review available local observations", Command: "harnessforge memory list --repository <repository>", Description: "Observations are local and are not approved knowledge. If needed, capture a fact explicitly with memory capture --source manual --evidence <path> --repository <repository>."},
			{Title: "Capture a fact as a local observation", Command: "harnessforge memory capture <fact> --source manual --evidence <path> --repository <repository>", Description: "Use repository-relative evidence paths. Capture stores a local observation only; it does not modify Forge knowledge."},
			{Title: "Review the observation", Command: "harnessforge memory review <observation-id> approved --reviewer <name> --repository <repository>", Description: "Check the captured content and evidence first. Use rejected instead when it should not be published."},
			{Title: "Publish as a candidate", Command: "harnessforge memory publish <observation-id> --kind <fact|convention|business_rule|decision|constraint> --repository <repository>", Description: "This creates a Forge candidate, not an approved rule."},
			{Title: "Approve the Forge candidate", Command: "harnessforge review observation-<observation-id> approved --reviewer <name> --repository <repository> --layout forge", Description: "Approve only after checking the candidate and cited evidence. Candidate IDs are prefixed with observation-."},
			{Title: "Preview generated changes", Command: "harnessforge sync --dry-run --repository <repository>", Description: "Inspect target files and conflicts; this step does not write."},
			{Title: "Apply reviewed changes", Command: "harnessforge sync --apply --repository <repository>", Description: "Apply only after the preview is acceptable. Human-edited generated files remain protected and produce a conflict."},
			{Title: "Verify the result", Command: "harnessforge check --repository <repository> --layout forge", Description: "Confirm knowledge evidence and generated outputs still match the source."},
		}
	}
	return guide, nil
}

func regularConfigExists(path string) (bool, error) {
	parent := filepath.Dir(path)
	parentInfo, parentErr := os.Lstat(parent)
	if os.IsNotExist(parentErr) {
		return false, nil
	}
	if parentErr != nil {
		return false, parentErr
	}
	if parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return false, fmt.Errorf("configuration directory must be a regular non-symlink directory: %s", parent)
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("configuration must be a regular non-symlink file: %s", path)
	}
	return true, nil
}

// FormatGuide renders a stable plain-text guide for terminal use.
func FormatGuide(guide Guide) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Onboarding: %s\nRepository: %s\n", guide.Status, guide.Repository)
	if guide.Layout != "" {
		fmt.Fprintf(&out, "Layout: %s (%s)\n", guide.Layout, guide.ConfigPath)
	}
	fmt.Fprintln(&out, guide.Message)
	for index, step := range guide.Steps {
		fmt.Fprintf(&out, "\n%d. %s\n", index+1, step.Title)
		if step.Command != "" {
			fmt.Fprintf(&out, "   $ %s\n", step.Command)
		}
		fmt.Fprintf(&out, "   %s\n", step.Description)
	}
	return out.String()
}
