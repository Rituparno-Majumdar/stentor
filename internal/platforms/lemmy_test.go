package platforms

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

func TestLemmyAuthenticateAndPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v3/user/login"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["username_or_email"] != "tester" || body["password"] != "hunter2" {
				t.Errorf("unexpected login payload: %+v", body)
			}
			json.NewEncoder(w).Encode(map[string]string{"jwt": "jwt123"})
		case strings.HasSuffix(r.URL.Path, "/api/v3/community"):
			if r.Header.Get("Authorization") != "Bearer jwt123" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			if got := r.URL.Query().Get("name"); got != "selfhosted" {
				t.Errorf("community name = %q, want %q", got, "selfhosted")
			}
			json.NewEncoder(w).Encode(map[string]any{
				"community_view": map[string]any{
					"community": map[string]any{"id": 7},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/api/v3/post"):
			if r.Header.Get("Authorization") != "Bearer jwt123" {
				t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "hello world" {
				t.Errorf("name = %v, want %q", body["name"], "hello world")
			}
			if int64(body["community_id"].(float64)) != 7 {
				t.Errorf("community_id = %v, want 7", body["community_id"])
			}
			json.NewEncoder(w).Encode(map[string]any{
				"post_view": map[string]any{
					"post": map[string]any{"id": 99},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := newTestStore(t)
	in := bufio.NewReader(strings.NewReader(srv.URL + "\ntester\nhunter2\nselfhosted\n"))
	l := NewLemmy()

	if err := l.Authenticate(context.Background(), cfg, in); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if !l.IsAuthenticated(cfg) {
		t.Fatal("IsAuthenticated = false after a successful Authenticate")
	}

	res, err := l.Post(context.Background(), cfg, content.Content{Text: "hello world"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if res.ID != "99" {
		t.Errorf("result.ID = %q, want %q", res.ID, "99")
	}
	if res.URL != srv.URL+"/post/99" {
		t.Errorf("result.URL = %q", res.URL)
	}
}
