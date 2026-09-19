package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"harnessforge/internal/config/domain"
	"harnessforge/internal/inputlimits"
	"harnessforge/internal/safefile"
	"io"
	"os"
	"path/filepath"
)

type FileStore struct{ Path string }

func (s FileStore) Load() (domain.Preferences, error) {
	info, err := os.Lstat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.DefaultPreferences(), nil
	}
	if err != nil {
		return domain.Preferences{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return domain.Preferences{}, fmt.Errorf("preferences must be a regular file")
	}
	data, err := inputlimits.ReadFile(s.Path, inputlimits.PreferencesBytes, "preferences")
	if err != nil {
		return domain.Preferences{}, err
	}
	var saved domain.Preferences
	decoder := json.NewDecoder(bytes.NewReader(data))
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
	if info, err := os.Lstat(s.Path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("preferences must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.Path), "preferences-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return safefile.Replace(file.Name(), s.Path)
}
