package platforms

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

func newTestStore(t *testing.T) *config.Store {
	t.Helper()
	s, err := config.LoadFromPath(t.TempDir() + "/credentials.json")
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	return s
}

func TestMastodonAuthenticateAndPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v1/accounts/verify_credentials"):
			if r.Header.Get("Authorization") != "Bearer tok123" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			json.NewEncoder(w).Encode(map[string]string{"username": "tester"})
		case strings.HasSuffix(r.URL.Path, "/api/v1/statuses"):
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["status"] != "hello world" {
				t.Errorf("status = %v, want %q", body["status"], "hello world")
			}
			json.NewEncoder(w).Encode(map[string]string{"id": "42", "url": "https://example.social/@tester/42"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := newTestStore(t)
	in := bufio.NewReader(strings.NewReader(srv.URL + "\ntok123\n"))
	m := NewMastodon()

	if err := m.Authenticate(context.Background(), cfg, in); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !m.IsAuthenticated(cfg) {
		t.Fatal("IsAuthenticated = false after a successful Authenticate")
	}

	res, err := m.Post(context.Background(), cfg, content.Content{Text: "hello world"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if res.ID != "42" {
		t.Errorf("result.ID = %q, want %q", res.ID, "42")
	}
	if res.URL != "https://example.social/@tester/42" {
		t.Errorf("result.URL = %q", res.URL)
	}
}
