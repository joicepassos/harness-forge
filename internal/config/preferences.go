package config

import (
	"harnessforge/internal/config/application"
	"harnessforge/internal/config/infrastructure"
	"harnessforge/internal/llm/infrastructure/chatcompat"
	"os"
	"path/filepath"
)

func UserPreferences() (*application.Preferences, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	store := infrastructure.FileStore{Path: filepath.Join(directory, "harnessforge", "preferences.json")}
	return application.NewPreferences(store, chatcompat.NewRegistry(os.Getenv)), nil
}
