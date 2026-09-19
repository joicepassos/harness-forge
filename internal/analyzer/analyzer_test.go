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
