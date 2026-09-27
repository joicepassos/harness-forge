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

type ManifestLoader struct{}

func (ManifestLoader) Load(path string) (domain.Manifest, error) {
	var manifest domain.Manifest
	data, err := inputlimits.ReadFile(path, inputlimits.HarnessYAMLBytes, "Forge manifest")
	if err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return manifest, fmt.Errorf("%s: expected one YAML document", path)
	}
	if err := rejectNull(value, "$"); err != nil {
		return manifest, err
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return manifest, fmt.Errorf("%s: YAML must use string keys: %w", path, err)
	}
	strict := json.NewDecoder(bytes.NewReader(jsonData))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("%s: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	if err := schemas.Validate("forge-layout-v1.schema.json", jsonData); err != nil {
		return manifest, err
	}
	return manifest, nil
}

type ProjectConfig struct {
	Layout   ProjectLayout
	Harness  domain.Harness
	Manifest *domain.Manifest
}

// ProjectLoader adapts the discovered layout to the existing Harness loader
// interface used by application services. A non-empty path is an explicit
// legacy Harness file; an empty path performs layout discovery.
type ProjectLoader struct {
	Root      string
	Selection string
}

func (l ProjectLoader) Load(path string) (domain.Harness, error) {
	if path != "" {
		return (YAMLLoader{}).Load(path)
	}
	project, err := LoadProject(l.Root, l.Selection)
	if err != nil {
		return domain.Harness{}, err
	}
	if project.Manifest != nil {
		return domain.Harness{}, fmt.Errorf("legacy Harness consumers cannot load the Forge manifest; use the Forge-native command")
	}
	return project.Harness, nil
}

// LoadProject resolves and loads either supported layout into one canonical
// Harness value while retaining the Forge manifest's references and targets.
func LoadProject(root, selection string) (ProjectConfig, error) {
	layout, err := ResolveLayout(root, selection)
	if err != nil {
		return ProjectConfig{}, err
	}
	if layout.Kind == LayoutHarness {
		h, err := (YAMLLoader{}).Load(layout.HarnessPath)
		if err != nil {
			return ProjectConfig{}, err
		}
		return ProjectConfig{Layout: layout, Harness: h}, nil
	}
	manifest, err := (ManifestLoader{}).Load(layout.ManifestPath)
	if err != nil {
		return ProjectConfig{}, err
	}
	h := domain.Harness{Version: manifest.IRVersion, Project: manifest.Project, QualityGates: manifest.QualityGates}
	return ProjectConfig{Layout: layout, Harness: h, Manifest: &manifest}, nil
}
