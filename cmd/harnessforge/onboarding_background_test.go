package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"harnessforge/internal/analyzer"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupBackgroundTestCache(t *testing.T) {
	t.Helper()
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
}
func setupBackgroundTestPayload(t *testing.T) setupWorkerInput {
	t.Helper()
	setupBackgroundTestCache(t)
	analysis := &analyzer.Analysis{Project: "fixture", Languages: []analyzer.Finding{{Value: "go"}}}
	payload := setupWorkerInput{ID: "0123456789abcdef", Config: setupProvider{Name: "deepseek", Model: "deepseek-flash", Key: "secret-key-fixture"}, Analysis: analysis, Documents: []setupDocument{{Source: "repository-file:README.md", Path: "README.md", Text: "private-document-fixture"}}, Notes: "private-notes-fixture"}
	now := time.Now().UTC()
	run := setupBackgroundRun{ID: payload.ID, Root: t.TempDir(), Provider: payload.Config.Name, Model: payload.Config.Model, Status: "queued", StartedAt: now, UpdatedAt: now, AnalysisHash: setupAnalysisHash(analysis), Documents: []setupRunDocument{{Path: "README.md", Source: payload.Documents[0].Source, Hash: setupDocumentHash(payload.Documents[0])}}}
	if err := saveSetupBackgroundRun(run); err != nil {
		t.Fatal(err)
	}
	return payload
}
func setupBackgroundPayloadReader(t *testing.T, payload setupWorkerInput) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}
func TestBackgroundWorkerStoresRealStagesWithoutPrivateInputs(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	proposer := func(ctx context.Context, _ setupProvider, _ *analyzer.Analysis, _ []setupDocument, _ string) (setupAIProposal, error) {
		reportSetupStage(ctx, "context_prepared")
		reportSetupStage(ctx, "waiting_provider")
		reportSetupStage(ctx, "response_received")
		reportSetupStage(ctx, "validating_proposal")
		return setupAIProposal{Summary: "safe-summary", DiscardedRules: 1}, nil
	}
	if err := runSetupBackgroundWorker(context.Background(), setupBackgroundPayloadReader(t, payload), payload.ID, proposer); err != nil {
		t.Fatal(err)
	}
	run, err := loadSetupBackgroundRun(payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "ready" || run.Proposal == nil || run.DiscardedRules != 1 || len(run.Events) != 5 {
		t.Fatalf("unexpected run: %+v", run)
	}
	path, _ := setupRunPath(payload.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{payload.Config.Key, payload.Notes, payload.Documents[0].Text} {
		if bytes.Contains(data, []byte(private)) {
			t.Fatalf("private input persisted: %s", private)
		}
	}
	entries, err := os.ReadDir(run.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("worker wrote project files")
	}
}
func TestBackgroundCancellationBeforeProviderCall(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	if err := cancelSetupBackgroundRun(payload.ID); err != nil {
		t.Fatal(err)
	}
	called := false
	proposer := func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		called = true
		return setupAIProposal{}, nil
	}
	if err := runSetupBackgroundWorker(context.Background(), setupBackgroundPayloadReader(t, payload), payload.ID, proposer); err != nil {
		t.Fatal(err)
	}
	run, err := loadSetupBackgroundRun(payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	if called || run.Status != "cancelled" {
		t.Fatalf("cancelled run sent a provider request: %v %s", called, run.Status)
	}
}
func TestBackgroundProviderErrorRedactsPrivateInputs(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	proposer := func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		return setupAIProposal{}, errors.New("HTTP 400 invalid model\x1b[31m " + payload.Config.Key + " " + payload.Notes + " " + payload.Documents[0].Text)
	}
	if err := runSetupBackgroundWorker(context.Background(), setupBackgroundPayloadReader(t, payload), payload.ID, proposer); err != nil {
		t.Fatal(err)
	}
	run, _ := loadSetupBackgroundRun(payload.ID)
	if run.Status != "failed" || !strings.Contains(run.Error, "HTTP 400 invalid model") {
		t.Fatalf("missing useful error: %s", run.Error)
	}
	for _, private := range []string{payload.Config.Key, payload.Notes, payload.Documents[0].Text, "\x1b"} {
		if strings.Contains(run.Error, private) {
			t.Fatalf("private error input persisted")
		}
	}
}
func TestBackgroundRejectsChangedWorkerPayload(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	payload.Documents[0].Text = "changed"
	called := false
	proposer := func(context.Context, setupProvider, *analyzer.Analysis, []setupDocument, string) (setupAIProposal, error) {
		called = true
		return setupAIProposal{}, nil
	}
	if err := runSetupBackgroundWorker(context.Background(), setupBackgroundPayloadReader(t, payload), payload.ID, proposer); err == nil || called {
		t.Fatal("changed authorized context accepted")
	}
	run, _ := loadSetupBackgroundRun(payload.ID)
	if run.Status != "failed" {
		t.Fatal("invalid worker input left pending")
	}
}
func TestBackgroundMalformedInputDoesNotLeaveQueuedRun(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	if err := runSetupBackgroundWorker(context.Background(), strings.NewReader("invalid"), payload.ID, nil); err == nil {
		t.Fatal("malformed input accepted")
	}
	run, _ := loadSetupBackgroundRun(payload.ID)
	if run.Status != "failed" {
		t.Fatal("malformed input left queued run")
	}
}
func TestBackgroundStaleRunAndConfinedIDs(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	run, _ := loadSetupBackgroundRun(payload.ID)
	run.StartedAt = time.Now().Add(-7 * time.Minute)
	if err := saveSetupBackgroundRun(run); err != nil {
		t.Fatal(err)
	}
	run, err := loadSetupBackgroundRun(payload.ID)
	if err != nil || run.Status != "failed" {
		t.Fatal("stale run remains active")
	}
	if _, err = setupRunPath("../outside"); err == nil {
		t.Fatal("path escape accepted")
	}
}
func TestBackgroundResumeRequiresConfirmation(t *testing.T) {
	setupBackgroundTestCache(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0600); err != nil {
		t.Fatal(err)
	}
	analysis, err := analyzer.AnalyzeWithOptions(context.Background(), root, analyzer.Options{})
	if err != nil {
		t.Fatal(err)
	}
	run := setupBackgroundRun{ID: "0123456789abcdef", Root: root, Provider: "deepseek", Model: "deepseek-flash", Status: "ready", StartedAt: time.Now(), AnalysisHash: setupAnalysisHash(analysis), Proposal: &setupAIProposal{}}
	for _, f := range analysis.Languages {
		run.Languages = append(run.Languages, f.Value)
	}
	if err = saveSetupBackgroundRun(run); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = resumeSetupBackgroundRun(context.Background(), strings.NewReader("1\n1\nn\n"), &output, run.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, ".harness")); !os.IsNotExist(err) {
		t.Fatal("declined review wrote project files")
	}
	if !strings.Contains(output.String(), "No project files have been changed.") {
		t.Fatal("no preview before confirmation")
	}
	output.Reset()
	if err = resumeSetupBackgroundRun(context.Background(), strings.NewReader("1\n1\ny\n"), &output, run.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, ".harness", "harness.yaml")); err != nil {
		t.Fatalf("approved proposal did not write harness: %v", err)
	}
	applied, err := loadSetupBackgroundRun(run.ID)
	if err != nil || applied.Status != "applied" {
		t.Fatal("approved run did not become applied")
	}
	if err = resumeSetupBackgroundRun(context.Background(), strings.NewReader(""), &output, run.ID, true); err == nil {
		t.Fatal("applied run accepted twice")
	}
}
