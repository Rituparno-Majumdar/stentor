package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: filepath.Join(dir, "credentials.json"), Entries: map[string]map[string]string{}}

	s.Set("mastodon", "access_token", "tok123")
	s.Set("mastodon", "instance", "https://example.social")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("credentials file mode = %v, want 0600", info.Mode().Perm())
	}

	s2 := &Store{path: s.path, Entries: map[string]map[string]string{}}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := json.Unmarshal(data, &s2.Entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := s2.Get("mastodon", "access_token"); got != "tok123" {
		t.Errorf("access_token = %q, want %q", got, "tok123")
	}
	if !s2.Has("mastodon") {
		t.Error("Has(mastodon) = false, want true")
	}
	if s2.Has("nonexistent") {
		t.Error("Has(nonexistent) = true, want false")
	}
	if got := s2.Get("nonexistent", "key"); got != "" {
		t.Errorf("Get on missing platform = %q, want empty", got)
	}
}
