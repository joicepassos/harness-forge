package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// MigrateV1ToV2 reads a validated v1 harness and writes an equivalent v2
// document to output. The source is never modified.
func MigrateV1ToV2(loader Loader, source, output string) error {
	h, err := loader.Load(source)
	if err != nil {
		return err
	}
	if h.Version != 1 {
		return fmt.Errorf("migration requires a version 1 harness")
	}
	h.Version = 2
	if err := h.Validate(); err != nil {
		return err
	}
	jsonData, err := json.Marshal(h)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(jsonData, &document); err != nil {
		return err
	}
	data, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	if filepath.Clean(source) == filepath.Clean(output) {
		return fmt.Errorf("migration output must differ from source")
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("migration output already exists: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(output)
	temporary, err := os.CreateTemp(dir, ".harness-migrate-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Chmod(0600)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err := loader.Load(name); err != nil {
		return fmt.Errorf("validate migrated harness: %w", err)
	}
	return os.Rename(name, output)
}
