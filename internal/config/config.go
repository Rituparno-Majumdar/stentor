// Package config stores per-platform credentials in a single local file.
// No database, no daemon -- stentor reads this, posts, exits.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store is the on-disk credential file: one map of key/value fields per
// platform (e.g. linkedin -> {access_token, refresh_token, person_urn}).
type Store struct {
	path     string
	Entries  map[string]map[string]string `json:"entries"`
}

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".stentor"), nil
}

// Load reads ~/.stentor/credentials.json, returning an empty Store if it
// doesn't exist yet (first run).
func Load() (*Store, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(d, "credentials.json")
	s := &Store{path: path, Entries: map[string]map[string]string{}}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.Entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return s, nil
}

// Save writes the store back to disk at mode 0600, creating ~/.stentor
// (mode 0700) if needed.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.Entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// Get returns one credential field for a platform ("" if unset).
func (s *Store) Get(platform, key string) string {
	if s.Entries[platform] == nil {
		return ""
	}
	return s.Entries[platform][key]
}

// Set stores one credential field for a platform. Call Save() after.
func (s *Store) Set(platform, key, value string) {
	if s.Entries[platform] == nil {
		s.Entries[platform] = map[string]string{}
	}
	s.Entries[platform][key] = value
}

// Has reports whether a platform has any stored credentials at all.
func (s *Store) Has(platform string) bool {
	return len(s.Entries[platform]) > 0
}
