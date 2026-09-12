package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"harnessforge/internal/config/domain"
	"io"
	"os"
	"path/filepath"
)

type FileStore struct{ Path string }

func (s FileStore) Load() (domain.Preferences, error) {
	file, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.DefaultPreferences(), nil
	}
	if err != nil {
		return domain.Preferences{}, err
	}
	defer file.Close()
	var saved domain.Preferences
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil {
		return saved, fmt.Errorf("read preferences: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return saved, fmt.Errorf("preferences must contain one JSON object")
	}
	return saved, nil
}
func (s FileStore) Save(saved domain.Preferences) error {
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.Path), "preferences-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.Path)
}
