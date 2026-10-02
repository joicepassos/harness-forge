package application_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"harnessforge/internal/harness/application"
	"harnessforge/internal/harness/infrastructure"
)

func TestMigrateV1ToV2PreservesReviewedFieldsAndSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "harness.yaml")
	output := filepath.Join(dir, "harness-v2.yaml")
	original := []byte(`version: 1
project:
  name: sample
  languages: [Go]
architecture:
  styles: [hexagonal]
rules:
  - id: ports
    description: Depend on ports
    scope:
      paths: [internal/**]
    origin: human
    status: approved
    evidence:
      - file: internal/port.go
        symbol: Port
skills:
  - id: backend
    description: Backend conventions
    path: skills/backend/SKILL.md
quality_gates:
  - id: unit
    command: go test ./...
`)
	if err := os.WriteFile(source, original, 0600); err != nil {
		t.Fatal(err)
	}
	loader := infrastructure.YAMLLoader{}
	before, err := loader.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.MigrateV1ToV2(loader, source, output); err != nil {
		t.Fatal(err)
	}
	after, err := loader.Load(output)
	if err != nil {
		t.Fatal(err)
	}
	before.Version = 2
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("migration lost fields:\nbefore=%#v\nafter=%#v", before, after)
	}
	unchanged, err := os.ReadFile(source)
	if err != nil || !reflect.DeepEqual(unchanged, original) {
		t.Fatalf("source was not preserved: %v", err)
	}
}
