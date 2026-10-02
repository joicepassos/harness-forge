package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAnalyzeDetectsGoProject(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "go.mod", "module example\n")
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "main_test.go", "package main\n")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	assertFinding(t, analysis.Languages, "Go")
	assertFinding(t, analysis.Build, "Go modules")
	assertFinding(t, analysis.Tests, "Go tests")
	if len(analysis.QualityGates) != 1 || analysis.QualityGates[0].Command != "go test ./..." {
		t.Fatalf("quality gates = %#v", analysis.QualityGates)
	}
}

func TestAnalyzeDetectsCommonRepositorySignals(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "pom.xml", "<dependency>spring-boot-starter-web</dependency><dependency>postgresql</dependency><dependency>flyway-core</dependency>")
	writeFile(t, dir, "Dockerfile", "FROM eclipse-temurin:21")
	writeFile(t, dir, ".github/workflows/ci.yml", "name: CI")
	writeFile(t, dir, "src/main/java/App.java", "class App {}")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	assertFinding(t, analysis.Languages, "Java")
	assertFinding(t, analysis.Build, "Maven")
	assertFinding(t, analysis.Frameworks, "Spring Boot")
	assertFinding(t, analysis.Infrastructure, "Docker")
	assertFinding(t, analysis.Infrastructure, "GitHub Actions")
	assertFinding(t, analysis.Database, "PostgreSQL")
	assertFinding(t, analysis.Database, "Flyway")
}

func TestFindingStrengthReflectsEvidenceKind(t *testing.T) {
	finding, found := (fileRule{value: "manifest", paths: []string{"go.mod"}}).Apply(Repository{Files: []string{"go.mod"}})
	if !found || finding.Strength != "strong" {
		t.Fatalf("manifest strength = %+v, found=%v", finding, found)
	}
	if _, found := (contentRule{value: "text", path: "README.md", text: "convention"}).Apply(Repository{Files: []string{"README.md"}}); found {
		t.Fatal("content rule matched without repository content")
	}
	if finding, found := (extensionRule{value: "go", extension: ".go"}).Apply(Repository{Files: []string{"main.go"}}); !found || finding.Strength != "medium" {
		t.Fatalf("extension strength = %+v, found=%v", finding, found)
	}
}

func TestAnalyzeDetectsNestedProjectSignals(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "backend/core/build.gradle", "implementation 'org.springframework.boot:spring-boot-starter-web'\nimplementation 'org.postgresql:postgresql'\n")
	writeFile(t, dir, "backend/core/src/test/java/com/example/AppTest.java", "class AppTest {}")
	writeFile(t, dir, "frontend/package.json", `{"dependencies":{"next":"latest","react":"latest"}}`)

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	assertFinding(t, analysis.Build, "Gradle")
	assertFinding(t, analysis.Build, "npm")
	assertFinding(t, analysis.Frameworks, "Spring Boot")
	assertFinding(t, analysis.Frameworks, "React")
	assertFinding(t, analysis.Frameworks, "Next.js")
	assertFinding(t, analysis.Database, "PostgreSQL")
	assertFinding(t, analysis.Tests, "Java tests")
}

func TestAnalyzeSuggestsEcosystemQualityGates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", "<project/>")
	writeFile(t, dir, "frontend/package.json", "{}")
	writeFile(t, dir, "services/api/package.json", "{}")
	writeFile(t, dir, "service/pyproject.toml", "[tool.pytest.ini_options]\n")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	commands := map[string]bool{}
	npmGates := 0
	for _, gate := range analysis.QualityGates {
		commands[gate.Command] = true
	}
	for _, command := range []string{"mvn test", "npm test", "pytest"} {
		if !commands[command] {
			t.Fatalf("missing %q in %#v", command, analysis.QualityGates)
		}
	}
	for _, gate := range analysis.QualityGates {
		if gate.Workspace != "" && (len(gate.Workspaces) != 1 || gate.Workspaces[0] != gate.Workspace) {
			t.Fatalf("quality gate missing workspace metadata: %#v", gate)
		}
		if gate.Command == "npm test" && gate.Workspace != "frontend" {
			if gate.Workspace != "services/api" {
				t.Fatalf("npm gate workspace = %q", gate.Workspace)
			}
		}
		if gate.Command == "npm test" {
			npmGates++
		}
		if gate.Command == "pytest" && gate.Workspace != "service" {
			t.Fatalf("pytest gate workspace = %q", gate.Workspace)
		}
	}
	if npmGates != 2 {
		t.Fatalf("npm gates = %d, want 2", npmGates)
	}
}

func TestAnalyzeAnnotatesFindingWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "packages/auth/go.mod", "module example/auth\n")
	writeFile(t, dir, "packages/auth/main.go", "package auth\n")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range analysis.Build {
		if finding.Value == "Go modules" {
			if finding.Workspace != "packages/auth" {
				t.Fatalf("workspace = %q, want packages/auth", finding.Workspace)
			}
			if len(finding.EvidenceItems) != 1 || finding.EvidenceItems[0].Path != "packages/auth/go.mod" || len(finding.EvidenceItems[0].SHA256) != 64 {
				t.Fatalf("structured evidence = %#v", finding.EvidenceItems)
			}
			if finding.EvidenceItems[0].StartLine != 1 || finding.EvidenceItems[0].EndLine != 1 {
				t.Fatalf("evidence lines = %#v", finding.EvidenceItems[0])
			}
			return
		}
	}
	t.Fatalf("Go modules finding not found: %#v", analysis.Build)
}

func TestAnalyzeCreatesOneGoGatePerModuleWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "services/auth/go.mod", "module example/auth\n")
	writeFile(t, dir, "services/billing/go.mod", "module example/billing\n")
	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	workspaces := map[string]bool{}
	for _, gate := range analysis.QualityGates {
		if gate.Command == "go test ./..." {
			workspaces[gate.Workspace] = true
		}
	}
	if !workspaces["services/auth"] || !workspaces["services/billing"] || len(workspaces) != 2 {
		t.Fatalf("go gate workspaces = %#v", workspaces)
	}
}

func TestAnalyzeExtractsGoModulePathsWithWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "frontend/go.mod", "module example.com/frontend\n\ngo 1.26\n")
	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.GoModules) != 1 || analysis.GoModules[0].Module != "example.com/frontend" || analysis.GoModules[0].Workspace != "frontend" {
		t.Fatalf("go modules = %#v", analysis.GoModules)
	}
}

func TestAnalyzeCollectsAllFindingWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "packages/auth/main.go", "package auth\n")
	writeFile(t, dir, "apps/web/main.go", "package web\n")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range analysis.Languages {
		if finding.Value == "Go" {
			if len(finding.Workspaces) != 2 || finding.Workspaces[0] != "apps/web" || finding.Workspaces[1] != "packages/auth" {
				t.Fatalf("workspaces = %#v", finding.Workspaces)
			}
			return
		}
	}
	t.Fatalf("Go finding not found: %#v", analysis.Languages)
}

func TestAnalyzeSkipsSymlinksAndSpecialFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires developer mode or elevated privileges")
	}
	dir, outside := t.TempDir(), t.TempDir()
	writeFile(t, outside, "pom.xml", "spring-boot postgresql")
	if err := os.Symlink(filepath.Join(outside, "pom.xml"), filepath.Join(dir, "pom.xml")); err != nil {
		t.Fatal(err)
	}
	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, findings := range [][]Finding{analysis.Build, analysis.Frameworks, analysis.Database} {
		for _, finding := range findings {
			if finding.Value == "Maven" || finding.Value == "Spring Boot" || finding.Value == "PostgreSQL" {
				t.Fatalf("symlink influenced analysis: %#v", analysis)
			}
		}
	}
}

func TestAnalyzeHonorsGitIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "generated/\n")
	writeFile(t, dir, "generated/go.mod", "module ignored\n")
	writeFile(t, dir, "main.go", "package main\n")

	analysis, err := Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Files != 2 {
		t.Fatalf("files = %d, want 2 (main.go and .gitignore)", analysis.Files)
	}
	for _, finding := range analysis.Build {
		if finding.Value == "Go modules" {
			t.Fatalf("ignored manifest influenced analysis: %#v", analysis.Build)
		}
	}
}

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func assertFinding(t *testing.T, findings []Finding, value string) {
	t.Helper()

	for _, finding := range findings {
		if finding.Value == value {
			return
		}
	}

	t.Fatalf("finding %q not found in %#v", value, findings)
}
