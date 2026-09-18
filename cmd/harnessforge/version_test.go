package main

import (
	"harnessforge/internal/config"
	"strings"
	"testing"
)

func TestVersionCommandPrintsBuildMetadata(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := config.Version, config.Commit, config.BuildDate
	t.Cleanup(func() {
		config.Version, config.Commit, config.BuildDate = originalVersion, originalCommit, originalBuildDate
	})
	config.Version, config.Commit, config.BuildDate = "v1.2.3", "abc123", "2026-09-13T00:00:00Z"

	output, err := executeRoot("version")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"harnessforge version v1.2.3", "commit abc123", "build date 2026-09-13T00:00:00Z"} {
		if !strings.Contains(output, want) {
			t.Fatalf("version output missing %q: %q", want, output)
		}
	}
}

func TestBuildMetadataHasDevelopmentDefaults(t *testing.T) {
	if config.Version != "dev" || config.Commit != "none" || config.BuildDate != "unknown" {
		t.Fatalf("unexpected development defaults: %q %q %q", config.Version, config.Commit, config.BuildDate)
	}
}
