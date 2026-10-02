package main

import (
	"bytes"
	"context"
	"harnessforge/internal/analyzer"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupReviewFixture(t *testing.T) setupBackgroundRun {
	t.Helper()
	setupBackgroundTestCache(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("The project requires human review.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	analysis, err := analyzer.AnalyzeWithOptions(context.Background(), root, analyzer.Options{})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := readSetupPath(context.Background(), root, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	run := setupBackgroundRun{ID: "0123456789abcdef", Root: root, Provider: "deepseek", Model: "deepseek-flash", Status: "ready", StartedAt: time.Now(), AnalysisHash: setupAnalysisHash(analysis), Documents: []setupRunDocument{{Path: docs[0].Path, Source: docs[0].Source, Hash: setupDocumentHash(docs[0])}}, Proposal: &setupAIProposal{Rules: []setupRule{{ID: "human-review", Description: "Require human review.", Evidence: []setupCitation{{Source: docs[0].Source, Quote: "requires human review"}}}}}}
	if err := saveSetupBackgroundRun(run); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestBackgroundResumeRejectsChangedContextAndInventedCitation(t *testing.T) {
	for _, scenario := range []string{"changed-document", "invented-citation"} {
		t.Run(scenario, func(t *testing.T) {
			run := setupReviewFixture(t)
			if scenario == "changed-document" {
				if err := os.WriteFile(filepath.Join(run.Root, "README.md"), []byte("Different context.\n"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				run.Proposal.Rules[0].Evidence[0].Quote = "not present in any authorized document"
				if err := saveSetupBackgroundRun(run); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if err := resumeSetupBackgroundRun(context.Background(), strings.NewReader("1\n1\ny\n"), &output, run.ID, true); err == nil {
				t.Fatal("unsafe saved proposal accepted")
			}
			if _, err := os.Stat(filepath.Join(run.Root, ".harness")); !os.IsNotExist(err) {
				t.Fatal("unsafe resume wrote project files")
			}
		})
	}
}

func TestBackgroundResumeAppliesOnceAndKeepsLocalFallback(t *testing.T) {
	for _, status := range []string{"ready", "failed"} {
		t.Run(status, func(t *testing.T) {
			run := setupReviewFixture(t)
			run.Status = status
			if err := saveSetupBackgroundRun(run); err != nil {
				t.Fatal(err)
			}
			answers := "1\n1\ny\n"
			if status == "failed" {
				answers = "1\ny\n1\ny\n"
			}
			var output bytes.Buffer
			if err := resumeSetupBackgroundRun(context.Background(), strings.NewReader(answers), &output, run.ID, true); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(run.Root, ".harness", "harness.yaml")); err != nil {
				t.Fatal(err)
			}
			if err := resumeSetupBackgroundRun(context.Background(), strings.NewReader(answers), &output, run.ID, true); err == nil {
				t.Fatal("already applied run accepted again")
			}
		})
	}
}

func TestBackgroundCancellationStopsActiveProvider(t *testing.T) {
	payload := setupBackgroundTestPayload(t)
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- runSetupBackgroundWorker(context.Background(), setupBackgroundPayloadReader(t, payload), payload.ID, func(ctx context.Context, _ setupProvider, _ *analyzer.Analysis, _ []setupDocument, _ string) (setupAIProposal, error) {
			close(started)
			<-ctx.Done()
			return setupAIProposal{}, ctx.Err()
		})
	}()
	<-started
	if err := cancelSetupBackgroundRun(payload.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop active provider")
	}
	run, err := loadSetupBackgroundRun(payload.ID)
	if err != nil || run.Status != "cancelled" || run.Proposal != nil {
		t.Fatal("cancelled provider produced an accepted proposal")
	}
}
