package application

import (
	"fmt"
	"harnessforge/internal/config/domain"
	"strings"
)

type Store interface {
	Load() (domain.Preferences, error)
	Save(domain.Preferences) error
}

type ProviderCatalog interface{ Supports(string) bool }

type Preferences struct {
	store   Store
	catalog ProviderCatalog
}

func NewPreferences(store Store, catalog ProviderCatalog) *Preferences {
	return &Preferences{store: store, catalog: catalog}
}
func (p *Preferences) Provider() (string, error) {
	saved, err := p.store.Load()
	if err != nil {
		return "", err
	}
	return p.validate(saved.Provider)
}
func (p *Preferences) SetProvider(value string) error {
	name, err := p.validate(value)
	if err != nil {
		return err
	}
	return p.store.Save(domain.Preferences{Provider: name})
}
func (p *Preferences) validate(value string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	if !p.catalog.Supports(name) {
		return "", fmt.Errorf("unsupported provider %q", name)
	}
	return name, nil
}
