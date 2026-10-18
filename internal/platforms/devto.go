package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type devto struct{}

func NewDevto() Platform { return devto{} }

func (devto) Name() string { return "devto" }
func (devto) Tier() Tier   { return TierFrictionless }
func (devto) FrictionNote() string {
	return "None -- generate a key yourself in Settings -> Account -> DEV API Keys, no review."
}

func (devto) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("devto", "api_key") != ""
}

func (devto) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	key, err := prompt(in, "API key (Settings -> Account -> DEV API Keys): ")
	if err != nil {
		return err
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := doJSON(ctx, http.MethodGet, "https://dev.to/api/users/me", map[string]string{"api-key": key}, nil, &me); err != nil {
		return fmt.Errorf("key did not verify: %w", err)
	}
	cfg.Set("devto", "api_key", key)
	fmt.Printf("Verified as @%s\n", me.Username)
	return nil
}

// dev.to articles need a title. Content has no dedicated title field, so
// one is derived from the first line of Text -- documented in the README.
func (devto) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	key := cfg.Get("devto", "api_key")
	if key == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth devto")
	}

	title := c.Text
	if idx := strings.IndexByte(title, '\n'); idx != -1 {
		title = title[:idx]
	}
	if len(title) > 100 {
		title = title[:97] + "..."
	}
	if strings.TrimSpace(title) == "" {
		title = "Untitled post"
	}

	body := c.Text
	if c.Link != "" {
		body = body + "\n\n" + c.Link
	}

	payload := map[string]any{
		"article": map[string]any{
			"title":         title,
			"body_markdown": body,
			"published":     true,
		},
	}
	var res struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := doJSON(ctx, http.MethodPost, "https://dev.to/api/articles", map[string]string{"api-key": key}, payload, &res); err != nil {
		return content.Result{}, err
	}
	return content.Result{Platform: "devto", URL: res.URL, ID: fmt.Sprintf("%d", res.ID)}, nil
}
