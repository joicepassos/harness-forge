package analyzer

import (
	"os"
	"path/filepath"
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
