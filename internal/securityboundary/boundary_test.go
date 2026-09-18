package securityboundary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSensitiveContentRecognizesQuotedAndStructuredValues(t *testing.T) {
	for _, value := range []string{
		`password = "fictional-password-for-test"`,
		`{"api_key":"fictional-credential-for-test"}`,
		`access_token: 'fictional-token-for-test'`,
	} {
		if !ContainsSensitiveContent([]byte(value)) {
			t.Fatalf("sensitive content was accepted: %s", value)
		}
	}
	if !SensitivePath("docs/credentials.md") {
		t.Fatal("credential-bearing path was accepted")
	}
}

func TestGitIgnoreAppliesDirectoryRulesAndNestedFiles(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore", "private/\n")
	write("sub/.gitignore", "notes.md\n")
	ignored, err := LoadGitIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"private/notes.md", "docs/private/notes.md", "sub/notes.md"} {
		if !ignored.Match(path) {
			t.Fatalf("ignored path was accepted: %s", path)
		}
	}
	if ignored.Match("sub/guide.md") {
		t.Fatal("safe path was ignored")
	}
}
