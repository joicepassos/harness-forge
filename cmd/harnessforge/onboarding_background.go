package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"harnessforge/internal/analyzer"
	"harnessforge/internal/llm/infrastructure/chatcompat"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

type setupRunDocument struct{ Path, Source, Hash string }
type setupRunEvent struct {
	Stage string
	At    time.Time
}
type setupBackgroundRun struct {
	AppliedFiles                                                      []string
	ID, Root, Provider, Model, ArtifactLanguage, Status, Stage, Error string
	StartedAt, UpdatedAt                                              time.Time
	Documents                                                         []setupRunDocument
	Languages                                                         []string
	Events                                                            []setupRunEvent
	Calls                                                             []chatcompat.CallEvent
	Proposal                                                          *setupAIProposal
	DiscardedRules, DiscardedSkills                                   int
	AnalysisHash                                                      string
}
type setupWorkerInput struct {
	ID        string
	Config    setupProvider
	Analysis  *analyzer.Analysis
	Documents []setupDocument
	Notes     string
}

var setupRunIDPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

func setupRunsDirectory() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "harnessforge", "setup-runs"), nil
}
func setupRunPath(id string) (string, error) {
	if !setupRunIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid setup run ID")
	}
	dir, err := setupRunsDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".json"), nil
}
func loadSetupBackgroundRun(id string) (setupBackgroundRun, error) {
	var run setupBackgroundRun
	path, err := setupRunPath(id)
	if err != nil {
		return run, err
	}
	f, err := os.Open(path)
	if err != nil {
		return run, err
	}
	defer f.Close()
	err = json.NewDecoder(io.LimitReader(f, 2<<20)).Decode(&run)
	if err == nil && run.ID != id {
		err = fmt.Errorf("setup run identity mismatch")
	}
	if err == nil && (run.Status == "queued" || run.Status == "running") && time.Since(run.StartedAt) > 6*time.Minute {
		run.Status = "failed"
		run.Stage = "failed"
		run.Error = "The worker stopped or exceeded its time limit. No project files were written."
	}
	return run, err
}
func saveSetupBackgroundRun(run setupBackgroundRun) error {
	path, err := setupRunPath(run.ID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".run-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
func setupAnalysisHash(analysis *analyzer.Analysis) string {
	snapshot := *analysis
	snapshot.Languages = nil
	data, _ := json.Marshal(snapshot)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func setupDocumentHash(document setupDocument) string {
	sum := sha256.Sum256([]byte(document.Text))
	return hex.EncodeToString(sum[:])
}
func startSetupBackground(root string, config setupProvider, analysis *analyzer.Analysis, documents []setupDocument, notes string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	id := hex.EncodeToString(random)
	now := time.Now().UTC()
	run := setupBackgroundRun{ID: id, Root: root, Provider: config.Name, Model: config.Model, ArtifactLanguage: config.ArtifactLanguage, Status: "queued", Stage: "preparing_context", StartedAt: now, UpdatedAt: now, AnalysisHash: setupAnalysisHash(analysis)}
	for _, f := range analysis.Languages {
		run.Languages = append(run.Languages, f.Value)
	}
	for _, d := range documents {
		run.Documents = append(run.Documents, setupRunDocument{Path: d.Path, Source: d.Source, Hash: setupDocumentHash(d)})
	}
	if err := saveSetupBackgroundRun(run); err != nil {
		return "", err
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(executable, "internal-setup-worker", id)
	configureSetupWorkerProcess(cmd)
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	if err = cmd.Start(); err != nil {
		pipe.Close()
		run.Status = "failed"
		run.Error = "Could not start the background worker."
		_ = saveSetupBackgroundRun(run)
		return "", err
	}
	// The authorized context and any manually entered key travel only through this anonymous pipe.
	err = json.NewEncoder(pipe).Encode(setupWorkerInput{ID: id, Config: config, Analysis: analysis, Documents: documents, Notes: notes})
	pipe.Close()
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		run.Status = "failed"
		run.Stage = "failed"
		run.Error = "Could not deliver the authorized context to the background worker."
		run.UpdatedAt = time.Now().UTC()
		_ = saveSetupBackgroundRun(run)
		return "", err
	}
	_ = cmd.Process.Release()
	return id, nil
}
func cancelSetupBackgroundRun(id string) error {
	run, err := loadSetupBackgroundRun(id)
	if err != nil {
		return err
	}
	if run.Status != "queued" && run.Status != "running" {
		return nil
	}
	path, _ := setupRunPath(id)
	return os.WriteFile(path+".cancel", []byte("cancel"), 0600)
}
func runSetupBackgroundWorker(ctx context.Context, input io.Reader, id string, proposer setupProposer) (workerErr error) {
	defer func() {
		if workerErr != nil {
			if run, err := loadSetupBackgroundRun(id); err == nil && (run.Status == "queued" || run.Status == "running") {
				run.Status = "failed"
				run.Stage = "failed"
				run.Error = "The background worker could not complete safely. No project files were written."
				run.UpdatedAt = time.Now().UTC()
				_ = saveSetupBackgroundRun(run)
			}
		}
	}()
	var payload setupWorkerInput
	if err := json.NewDecoder(io.LimitReader(input, 4<<20)).Decode(&payload); err != nil {
		return fmt.Errorf("could not read worker input")
	}
	if payload.ID != id || payload.Analysis == nil {
		return fmt.Errorf("invalid worker input")
	}
	run, err := loadSetupBackgroundRun(id)
	if err != nil {
		return err
	}
	if run.Provider != payload.Config.Name || run.Model != payload.Config.Model || run.AnalysisHash != setupAnalysisHash(payload.Analysis) || len(run.Documents) != len(payload.Documents) {
		return fmt.Errorf("worker context does not match the authorized run")
	}
	for i, d := range payload.Documents {
		if d.Source != run.Documents[i].Source || d.Path != run.Documents[i].Path || setupDocumentHash(d) != run.Documents[i].Hash {
			return fmt.Errorf("worker document does not match the authorized run")
		}
	}
	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	path, _ := setupRunPath(id)
	if _, err := os.Stat(path + ".cancel"); err == nil {
		run.Status = "cancelled"
		run.Stage = "cancelled"
		run.UpdatedAt = time.Now().UTC()
		return saveSetupBackgroundRun(run)
	}
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if _, err := os.Stat(path + ".cancel"); err == nil {
					cancel()
					return
				}
			}
		}
	}()
	run.Status = "running"
	run.UpdatedAt = time.Now().UTC()
	if err = saveSetupBackgroundRun(run); err != nil {
		return err
	}
	var persistErr error
	persist := func() {
		if persistErr == nil {
			persistErr = saveSetupBackgroundRun(run)
			if persistErr != nil {
				cancel()
			}
		}
	}
	requestCtx = context.WithValue(requestCtx, setupStageKey{}, func(stage setupStage) {
		run.Stage = string(stage)
		run.UpdatedAt = time.Now().UTC()
		run.Events = append(run.Events, setupRunEvent{Stage: string(stage), At: run.UpdatedAt})
		persist()
	})
	requestCtx = withSetupCallReporter(requestCtx, func(event chatcompat.CallEvent) {
		run.Calls = append(run.Calls, event)
		run.UpdatedAt = time.Now().UTC()
		persist()
	})
	proposal, proposalErr := proposer(requestCtx, payload.Config, payload.Analysis, payload.Documents, payload.Notes)
	if _, err := os.Stat(path + ".cancel"); err == nil {
		cancel()
	}
	run.UpdatedAt = time.Now().UTC()
	if persistErr != nil {
		run.Status = "failed"
		run.Stage = "failed"
		run.Error = "Could not save the worker state; the proposal was not accepted."
	} else if requestCtx.Err() == context.DeadlineExceeded {
		run.Status = "failed"
		run.Stage = "failed"
		run.Error = "The provider exceeded the background time limit. No project files were written."
	} else if requestCtx.Err() != nil {
		run.Status = "cancelled"
		run.Stage = "cancelled"
	} else if proposalErr != nil {
		run.Status = "failed"
		run.Stage = "failed"
		run.Error = setupWorkerSafeError(proposalErr, payload)
	} else {
		run.Status = "ready"
		run.Stage = "ready"
		run.Proposal = &proposal
		run.DiscardedRules = proposal.DiscardedRules
		run.DiscardedSkills = proposal.DiscardedSkills
	}
	run.Events = append(run.Events, setupRunEvent{Stage: run.Stage, At: run.UpdatedAt})
	return saveSetupBackgroundRun(run)
}

func setupWorkerSafeText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
}
func setupWorkerSafeError(err error, payload setupWorkerInput) string {
	message := err.Error()
	for _, secret := range []string{payload.Config.Key, os.Getenv(setupKeyVariable(payload.Config.Name)), payload.Notes} {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	for _, doc := range payload.Documents {
		for _, line := range strings.Split(doc.Text, "\n") {
			if len(strings.TrimSpace(line)) >= 3 {
				message = strings.ReplaceAll(message, line, "[redacted]")
			}
		}
	}
	message = setupWorkerSafeText(message)
	if len(message) > 2048 {
		message = message[:2048]
	}
	return message
}

func newSetupWorkerCommand() *cobra.Command {
	return &cobra.Command{Use: "internal-setup-worker <id>", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return runSetupBackgroundWorker(cmd.Context(), cmd.InOrStdin(), args[0], requestSetupProposal)
	}}
}
func newSetupRunCommands() []*cobra.Command {
	runs := &cobra.Command{Use: "runs", Short: "List background proposal runs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		dir, err := setupRunsDirectory()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "No background runs.")
			return nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			id := entry.Name()[:len(entry.Name())-5]
			run, err := loadSetupBackgroundRun(id)
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-10s  %s / %s  %s\n", id, setupWorkerSafeText(run.Status), setupWorkerSafeText(run.Provider), setupWorkerSafeText(run.Model), setupWorkerSafeText(filepath.Base(run.Root)))
			}
		}
		return nil
	}}
	var inspectAccessible, resumeAccessible bool
	inspect := &cobra.Command{Use: "inspect <id>", Short: "Inspect a background proposal without writing project files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		output := cmd.OutOrStdout()
		if flag := cmd.Flags().Lookup("language"); flag != nil && (strings.EqualFold(flag.Value.String(), "pt-BR") || strings.EqualFold(flag.Value.String(), "pt")) {
			output = setupLocalizedWriter{output: output}
		}
		return inspectSetupBackgroundRun(cmd.Context(), cmd.InOrStdin(), output, args[0], inspectAccessible)
	}}
	inspect.Flags().BoolVar(&inspectAccessible, "accessible", false, "Use plain output")
	resume := &cobra.Command{Use: "resume <id>", Short: "Review a completed proposal and confirm project changes", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return resumeSetupBackgroundRun(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), args[0], resumeAccessible)
	}}
	resume.Flags().BoolVar(&resumeAccessible, "accessible", false, "Use plain prompts")
	cancel := &cobra.Command{Use: "cancel <id>", Short: "Cancel an active background proposal", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := cancelSetupBackgroundRun(args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Cancellation requested; no project files changed.")
		return nil
	}}
	forget := &cobra.Command{Use: "forget <id>", Short: "Delete a finished run and its privately cached proposal", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		run, err := loadSetupBackgroundRun(args[0])
		if err != nil {
			return err
		}
		if run.Status == "queued" || run.Status == "running" {
			return fmt.Errorf("cancel the run and wait for it to finish before forgetting it")
		}
		path, _ := setupRunPath(args[0])
		if err = os.Remove(path); err != nil {
			return err
		}
		if err = os.Remove(path + ".cancel"); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Run and cached proposal deleted.")
		return nil
	}}
	return []*cobra.Command{runs, inspect, resume, cancel, forget}
}

func resumeSetupBackgroundRun(ctx context.Context, input io.Reader, output io.Writer, id string, accessible bool) error {
	return reviewSetupBackgroundRun(ctx, input, output, id, accessible, false)
}

func reviewSetupBackgroundRun(ctx context.Context, input io.Reader, output io.Writer, id string, accessible, preserveLanguage bool) error {
	run, err := loadSetupBackgroundRun(id)
	if err != nil {
		return err
	}
	if run.Status == "running" || run.Status == "queued" {
		return fmt.Errorf("proposal is still running; use harnessforge init inspect %s", id)
	}
	if run.Status == "applied" {
		return fmt.Errorf("this proposal has already been applied")
	}
	session := setupSession{reader: bufio.NewReader(input), input: input, output: output, ctx: ctx, interactive: setupInteractive(input, (setupSession{output: output}).uiOutput(), accessible)}
	if !preserveLanguage {
		output, err = chooseSetupInterface(session.reader, output, session)
		if err != nil {
			return err
		}
	}
	session.output = output
	analysis, err := analyzer.AnalyzeWithOptions(ctx, run.Root, analyzer.Options{})
	if err != nil {
		return err
	}
	if setupAnalysisHash(analysis) != run.AnalysisHash {
		return fmt.Errorf("project analysis changed since this run; start a new setup before applying")
	}
	analysis.Languages = nil
	for _, language := range run.Languages {
		analysis.Languages = append(analysis.Languages, analyzer.Finding{Value: language})
	}
	var documents []setupDocument
	for _, saved := range run.Documents {
		docs, err := readSetupPath(ctx, run.Root, saved.Path)
		if err != nil {
			return err
		}
		matched := false
		for _, doc := range docs {
			if doc.Source == saved.Source && setupDocumentHash(doc) == saved.Hash {
				documents = append(documents, doc)
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("selected context changed since this run; start a new setup before applying")
		}
	}
	config := setupProvider{Name: run.Provider, Model: run.Model, ArtifactLanguage: run.ArtifactLanguage}
	proposal := setupAIProposal{}
	if run.Status != "ready" || run.Proposal == nil {
		if run.Error != "" {
			fmt.Fprintf(output, "AI proposal could not be validated: %s\n", setupWorkerSafeText(run.Error))
		}
		fmt.Fprintln(output, "The background AI proposal is unavailable. Your observations were not stored; a local proposal uses the current project and selected files.")
		allowed, err := session.confirm("Continue with a local proposal?")
		if err != nil {
			return err
		}
		if !allowed {
			return nil
		}
		config = setupProvider{ArtifactLanguage: run.ArtifactLanguage}
	} else {
		proposal = *run.Proposal
		proposal.DiscardedRules = run.DiscardedRules
		proposal.DiscardedSkills = run.DiscardedSkills
		if err := validateSetupProposal(proposal, setupSources(analysis, documents, "")); err != nil {
			return fmt.Errorf("saved AI proposal no longer has valid project-file citations; start a new setup")
		}
		if proposal.DiscardedRules+proposal.DiscardedSkills > 0 {
			fmt.Fprintf(output, "The AI generated %d rule(s) and %d skill(s) without valid project-file citations; these items were discarded. The remaining proposal has verified citations and still requires your review.\n", proposal.DiscardedRules, proposal.DiscardedSkills)
		}
	}
	agents, err := askSetupAgents(session)
	if err != nil {
		return err
	}
	plan, err := buildSetupPlan(run.Root, analysis, documents, "", config, agents, proposal)
	if err != nil {
		return err
	}
	showSetupPlan(output, plan)
	var approved bool
	if preserveLanguage && session.interactive {
		choice, choiceErr := session.formSelect(session.uiText("Review complete: choose the next action"), []string{session.uiText("Apply these files"), session.uiText("Back without applying")}, []string{"apply", "back"}, "back")
		err = choiceErr
		approved = choice == "apply"
	} else {
		approved, err = session.confirm("Create or update exactly these files?")
	}
	if err != nil {
		return err
	}
	if !approved {
		fmt.Fprintln(output, "Setup cancelled; no project files changed.")
		return nil
	}
	if err = writeSetupPlan(ctx, run.Root, plan.Files); err != nil {
		return err
	}
	run.Status = "applied"
	for _, file := range plan.Files {
		run.AppliedFiles = append(run.AppliedFiles, file.Path)
	}
	run.Stage = "applied"
	run.UpdatedAt = time.Now().UTC()
	run.Events = append(run.Events, setupRunEvent{Stage: "applied", At: run.UpdatedAt})
	if err = saveSetupBackgroundRun(run); err != nil {
		return err
	}
	fmt.Fprintln(output, "Setup complete. Review the generated files before committing them.")
	return nil
}
