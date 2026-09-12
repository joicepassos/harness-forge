package main

import (
	"bytes"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isolatePreferences(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	variable := "XDG_CONFIG_HOME"
	if runtime.GOOS == "windows" {
		variable = "AppData"
	}
	if runtime.GOOS == "darwin" {
		variable = "HOME"
	}
	t.Setenv(variable, directory)
	root, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "harnessforge", "preferences.json")
}
func executeConfig(args ...string) (string, error) {
	command := newConfigCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}
func TestPreferencesPersistAndRejectSecrets(t *testing.T) {
	path := isolatePreferences(t)
	output, err := executeConfig("get", "provider")
	if err != nil || strings.TrimSpace(output) != "openai" {
		t.Fatalf("default %q %v", output, err)
	}
	for _, name := range []string{"deepseek", "gemini"} {
		if _, err := executeConfig("set", "provider", name); err != nil {
			t.Fatal(err)
		}
		output, err = executeConfig("get", "provider")
		if err != nil || strings.TrimSpace(output) != name {
			t.Fatalf("saved %q %v", output, err)
		}
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"set", "api_key", "secret"}, {"set", "provider", "invalid"}} {
		if _, err := executeConfig(args...); err == nil {
			t.Fatal("invalid setting accepted")
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("invalid setting changed preferences")
	}
}
func TestExplicitProviderOverridesSavedPreference(t *testing.T) {
	path := isolatePreferences(t)
	if _, err := executeConfig("set", "provider", "deepseek"); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("provider", "openai", "")
	name, err := selectedProvider(cmd)
	if err != nil || name != "deepseek" {
		t.Fatalf("saved preference %s %v", name, err)
	}
	if err := cmd.Flags().Set("provider", "groq"); err != nil {
		t.Fatal(err)
	}
	name, err = selectedProvider(cmd)
	if err != nil || name != "groq" {
		t.Fatalf("flag override %s %v", name, err)
	}
	if err := os.WriteFile(path, []byte(`{"provider":"deepseek","api_key":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := executeConfig("get", "provider"); err == nil {
		t.Fatal("unknown field accepted")
	}
}
