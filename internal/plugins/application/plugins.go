package application

import (
	"context"
	"fmt"
	"harnessforge/internal/plugins/domain"
	"slices"
	"strings"
	"time"
)

type Catalog interface {
	Discover(context.Context, string) ([]domain.Manifest, error)
}

type Executor interface {
	Execute(context.Context, string, domain.Manifest, domain.Request) (domain.Response, error)
}

type Plugins struct {
	catalog  Catalog
	executor Executor
}

func NewPlugins(c Catalog, e Executor) *Plugins { return &Plugins{catalog: c, executor: e} }

func (p *Plugins) Discover(ctx context.Context, directory string) ([]domain.Manifest, error) {
	return p.catalog.Discover(ctx, directory)
}

func (p *Plugins) Execute(ctx context.Context, directory, name, capability string, input map[string]any, authorized bool, timeout time.Duration) (domain.Response, error) {
	if !authorized {
		return domain.Response{}, fmt.Errorf("plugin execution requires explicit authorization")
	}
	if timeout <= 0 || timeout > 5*time.Minute {
		return domain.Response{}, fmt.Errorf("plugin timeout must be between 1ns and 5m")
	}
	manifests, err := p.catalog.Discover(ctx, directory)
	if err != nil {
		return domain.Response{}, err
	}
	var selected *domain.Manifest
	for i := range manifests {
		if manifests[i].Name == name {
			selected = &manifests[i]
			break
		}
	}
	if selected == nil {
		return domain.Response{}, fmt.Errorf("plugin %q was not discovered", name)
	}
	if selected.APIVersion != domain.APIVersion {
		return domain.Response{}, fmt.Errorf("plugin %q uses incompatible API version %q; expected %q", name, selected.APIVersion, domain.APIVersion)
	}
	if strings.TrimSpace(capability) == "" || !slices.Contains(selected.Capabilities, capability) {
		return domain.Response{}, fmt.Errorf("plugin %q does not declare capability %q", name, capability)
	}
	request := domain.Request{APIVersion: domain.APIVersion, Operation: "execute", Capability: capability, Input: input}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := p.executor.Execute(runCtx, directory, *selected, request)
	if err != nil {
		return domain.Response{}, fmt.Errorf("plugin %q failed: %w", name, err)
	}
	if response.APIVersion != domain.APIVersion {
		return domain.Response{}, fmt.Errorf("plugin %q returned incompatible API version %q", name, response.APIVersion)
	}
	if response.Error != "" {
		return domain.Response{}, fmt.Errorf("plugin %q failed: %s", name, response.Error)
	}
	return response, nil
}
