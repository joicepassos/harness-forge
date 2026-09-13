package infrastructure

import (
	"context"
	"encoding/json"
	"harnessforge/internal/plugins/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverIsDeterministicAndDoesNotExecute(t *testing.T) {
	directory := t.TempDir()
	marker := filepath.Join(directory, "executed")
	manifest, err := json.Marshal(domain.Manifest{APIVersion: domain.APIVersion, Name: "example", Description: "Example", Command: []string{"tool", marker}, Capabilities: []string{"analyzer"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "example.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	plugins, err := (Local{}).Discover(context.Background(), directory)
	if err != nil || len(plugins) != 1 {
		t.Fatal(plugins, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("discovery executed plugin")
	}
}

func TestDiscoverRejectsUnknownManifestFields(t *testing.T) {
	directory := t.TempDir()
	manifest := `{"api_version":"harnessforge.plugin/v1","name":"example","command":["tool"],"capabilities":["analyzer"],"credential":"secret"}`
	if err := os.WriteFile(filepath.Join(directory, "example.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Local{}).Discover(context.Background(), directory)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatal(err)
	}
}

func TestSafeEnvironmentDoesNotForwardCredentials(t *testing.T) {
	t.Setenv("HARNESSFORGE_TEST_TOKEN", "must-not-leak")
	for _, value := range safeEnvironment() {
		if strings.Contains(value, "must-not-leak") {
			t.Fatal("credential was forwarded")
		}
	}
}
