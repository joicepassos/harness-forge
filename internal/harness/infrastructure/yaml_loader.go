package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v3"
	"harnessforge/internal/harness/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/schemas"
	"io"
)

// YAMLLoader only reads: manual formatting and comments remain byte-for-byte intact.
type YAMLLoader struct{}

func (YAMLLoader) Load(path string) (domain.Harness, error) {
	var h domain.Harness
	data, err := inputlimits.ReadFile(path, inputlimits.HarnessYAMLBytes, "Harness YAML")
	if err != nil {
		return h, fmt.Errorf("%s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return h, fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return h, fmt.Errorf("%s: expected one YAML document", path)
	}
	if err := rejectNull(value, "$"); err != nil {
		return h, err
	}
	// JSON conversion enforces string types rather than YAML's scalar-to-string coercion.
	data, err = json.Marshal(value)
	if err != nil {
		return h, fmt.Errorf("%s: YAML must use string keys: %w", path, err)
	}
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&h); err != nil {
		return h, fmt.Errorf("%s: %w", path, err)
	}
	if err := h.Validate(); err != nil {
		return h, err
	}
	if err := schemas.Validate("harness-v1.schema.json", data); err != nil {
		return h, err
	}
	return h, nil
}

func rejectNull(value any, field string) error {
	switch item := value.(type) {
	case nil:
		return fmt.Errorf("%s: null is not allowed; omit optional fields instead", field)
	case map[string]any:
		for key, child := range item {
			if err := rejectNull(child, field+"."+key); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range item {
			if err := rejectNull(child, fmt.Sprintf("%s[%d]", field, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
