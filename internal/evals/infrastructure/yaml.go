package infrastructure

import (
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/evals/domain"
	"io"
	"os"
)

const maxInput = 1024 * 1024

func Dataset(path string) (domain.Dataset, error) {
	var value domain.Dataset
	return value, read(path, &value)
}
func Results(path string) (domain.Results, error) {
	var value domain.Results
	return value, read(path, &value)
}
func read(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(io.LimitReader(file, maxInput))
	decoder.KnownFields(true)
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s: expected one YAML document", path)
	}
	return nil
}
