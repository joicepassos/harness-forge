package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLayoutReportsEvidenceBackedArchitecture(t *testing.T) {
	structure, architecture := detectLayout([]string{
		"internal/domain/order.go",
		"internal/application/create_order.go",
		"internal/infrastructure/store.go",
		"docs/architecture.md",
	})
	if len(structure) == 0 || len(architecture) != 1 || architecture[0].Value != "Layered domain/application/infrastructure directories" || len(architecture[0].Evidence) != 3 {
		t.Fatalf("structure=%v architecture=%v", structure, architecture)
	}
}

func TestAdditionalStackAndConventionDetection(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "Service.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk.Web"><PackageReference Include="Microsoft.AspNetCore.OpenApi" /></Project>`), 0644); err != nil {
		t.Fatal(err)
	}
	repository := Repository{Path: root, Files: []string{"backend/Service.csproj", "backend/Program.cs", "docs/adr/001.md", ".editorconfig"}}
	build := (buildDetector{}).Detect(context.Background(), repository)
	frameworks := (frameworkDetector{}).Detect(context.Background(), repository)
	conventions := detectConventions(repository.Files)
	if !containsFinding(build, ".NET") || !containsFinding(frameworks, "ASP.NET Core") || !containsFinding(conventions, "Architecture decision records") || !containsFinding(conventions, "EditorConfig conventions") {
		t.Fatalf("build=%v frameworks=%v conventions=%v", build, frameworks, conventions)
	}
}

func containsFinding(findings []Finding, value string) bool {
	for _, item := range findings {
		if item.Value == value {
			return true
		}
	}
	return false
}
