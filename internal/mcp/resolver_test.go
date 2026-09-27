package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolverRequiresBudgetAndUsesForgeKnowledge(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".forge"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".forge", "forge.yaml"), []byte("layout_version: 1\nir_version: 2\nproject: {name: sample}\ntargets: [codex]\nreferences: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{}
	if _, err := resolver.ResolveContext(context.Background(), root, "build", "", 0); err == nil {
		t.Fatal("expected explicit positive budget")
	}
	data, err := resolver.ReadResource(context.Background(), root, "build", "", 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty resource")
	}
}
