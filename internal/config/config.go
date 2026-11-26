// Package config persists the small amount of session state Bento restores on
// the next launch. Writes are atomic; nothing here runs inside Update.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const schemaVersion = 1

// State is the persisted session snapshot.
type State struct {
	Version   int      `json:"version"`
	LastDir   string   `json:"last_dir"`
	LastFile  string   `json:"last_file"`
	Focus     string   `json:"focus"`
	ShowTree  bool     `json:"show_tree"`
	ShowAgent bool     `json:"show_agent"`
	Line      int      `json:"line"`
	Col       int      `json:"col"`
	Recents   []string `json:"recents,omitempty"`

	Valid bool `json:"-"`
}

// Path returns the per-user config file location.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bento", "config.json"), nil
}

// Load reads the config file. A missing file is not an error.
func Load() (State, error) {
	path, err := Path()
	if err != nil {
		return State{}, err
	}
	return loadFrom(path)
}

func loadFrom(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, err
	}
	st.Valid = true
	return st, nil
}

// Save writes the config atomically.
func Save(st State) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return saveTo(path, st)
}

func saveTo(path string, st State) error {
	st.Version = schemaVersion
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
