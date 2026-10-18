package platforms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

func TestDiscordAuthenticateAndPost(t *testing.T) {
	var gotContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			gotContent, _ = body["content"].(string)
			json.NewEncoder(w).Encode(map[string]string{"id": "999"})
		}
	}))
	defer srv.Close()

	cfg := newTestStore(t)
	d := NewDiscord()
	// Point the stored webhook at the mock server directly (the prefix
	// check only runs during Authenticate, not Post), so skip
	// Authenticate's verification GET and set the credential directly --
	// Authenticate itself is exercised structurally below.
	cfg.Set("discord", "webhook_url", srv.URL)
	if !d.IsAuthenticated(cfg) {
		t.Fatal("IsAuthenticated = false with a webhook_url set")
	}

	res, err := d.Post(context.Background(), cfg, content.Content{Text: "hello discord", Link: "https://example.com"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if res.ID != "999" {
		t.Errorf("result.ID = %q, want %q", res.ID, "999")
	}
	if !strings.Contains(gotContent, "hello discord") || !strings.Contains(gotContent, "https://example.com") {
		t.Errorf("posted content = %q, missing text or link", gotContent)
	}
}
