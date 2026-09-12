package application

import (
	"context"
	"harnessforge/internal/skills/domain"
	"testing"
)

type fixedFiles []File

func (f fixedFiles) Files(context.Context, string) ([]File, error) { return f, nil }

func TestDiscoverRequiresMultiLayerEvidence(t *testing.T) {
	files := fixedFiles{
		{Path: "api/OrderController.java", Text: "@RestController class OrderController {}"},
		{Path: "api/InvoiceController.java", Text: "@RestController class InvoiceController {}"},
		{Path: "service/OrderService.java", Text: "@Service class OrderService {}"},
		{Path: "service/InvoiceService.java", Text: "@Service class InvoiceService {}"},
		{Path: "data/OrderRepository.java", Text: "@Repository class OrderRepository {}"},
		{Path: "db/V1__orders.sql", Text: "CREATE TABLE orders(id bigint)"},
		{Path: "OrderTest.java", Text: "@SpringBootTest class OrderTest {}"},
	}
	proposals, err := NewDiscover(files).Execute(context.Background(), "repository")
	if err != nil {
		t.Fatal(err)
	}
	if len(proposals) != 1 || len(proposals[0].Examples) != 5 {
		t.Fatalf("unexpected proposals: %+v", proposals)
	}
	proposals, err = NewDiscover(files[:3]).Execute(context.Background(), "repository")
	if err != nil {
		t.Fatal(err)
	}
	if len(proposals) != 0 {
		t.Fatalf("weak evidence proposed a skill: %+v", proposals)
	}
}

type recordingWriter struct{ called bool }

func (w *recordingWriter) Generate(string, domain.Proposal) error { w.called = true; return nil }

func TestGenerateRequiresApprovalAndSafeID(t *testing.T) {
	writer := &recordingWriter{}
	proposal := domain.Proposal{ID: "add-backed-feature", Description: "Description", Examples: []domain.Evidence{{File: "a.java", Symbol: "A"}}}
	if err := NewGenerate(writer).Execute("repository", proposal, false); err == nil || writer.called {
		t.Fatal("unapproved proposal generated a skill")
	}
	proposal.ID = "../escape"
	if err := NewGenerate(writer).Execute("repository", proposal, true); err == nil || writer.called {
		t.Fatal("unsafe skill id generated a skill")
	}
	proposal.ID = "add-backed-feature"
	proposal.Examples[0].File = "../outside"
	if err := NewGenerate(writer).Execute("repository", proposal, true); err == nil || writer.called {
		t.Fatal("unsafe evidence generated a skill")
	}
	proposal.Examples[0].File = "a.java"
	if err := NewGenerate(writer).Execute("repository", proposal, true); err != nil || !writer.called {
		t.Fatalf("approved proposal not generated: %v", err)
	}
}
