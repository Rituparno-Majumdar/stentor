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

type mastodon struct{}

func NewMastodon() Platform { return mastodon{} }

func (mastodon) Name() string { return "mastodon" }
func (mastodon) Tier() Tier   { return TierFrictionless }
func (mastodon) FrictionNote() string {
	return "None -- generate a token yourself in your instance's Settings -> Development -> New Application, no review."
}

func (mastodon) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("mastodon", "access_token") != "" && cfg.Get("mastodon", "instance") != ""
}

func (mastodon) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	instance, err := prompt(in, "Mastodon instance URL (e.g. https://mastodon.social): ")
	if err != nil {
		return err
	}
	instance = strings.TrimRight(instance, "/")
	token, err := prompt(in, "Access token (Settings -> Development -> New Application on that instance): ")
	if err != nil {
		return err
	}

	var acct struct {
		Username string `json:"username"`
	}
	if err := doJSON(ctx, http.MethodGet, instance+"/api/v1/accounts/verify_credentials",
		map[string]string{"Authorization": "Bearer " + token}, nil, &acct); err != nil {
		return fmt.Errorf("token did not verify against %s: %w", instance, err)
	}

	cfg.Set("mastodon", "instance", instance)
	cfg.Set("mastodon", "access_token", token)
	cfg.Set("mastodon", "username", acct.Username)
	fmt.Printf("Verified as @%s on %s\n", acct.Username, instance)
	return nil
}

func (mastodon) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	instance := cfg.Get("mastodon", "instance")
	token := cfg.Get("mastodon", "access_token")
	if instance == "" || token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth mastodon")
	}

	status := c.Text
	if c.Link != "" {
		status = status + "\n\n" + c.Link
	}

	headers := map[string]string{"Authorization": "Bearer " + token}
	var mediaIDs []string
	if c.Image != "" {
		var media struct {
			ID string `json:"id"`
		}
		data, statusCode, err := doMultipart(ctx, instance+"/api/v2/media", headers, nil, "file", c.Image)
		if err != nil {
			return content.Result{}, fmt.Errorf("uploading image: %w", err)
		}
		if statusCode >= 300 {
			return content.Result{}, fmt.Errorf("media upload http %d: %s", statusCode, truncate(string(data), 300))
		}
		if err := jsonUnmarshal(data, &media); err != nil {
			return content.Result{}, err
		}
		mediaIDs = []string{media.ID}
	}

	payload := map[string]any{"status": status}
	if len(mediaIDs) > 0 {
		payload["media_ids"] = mediaIDs
	}
	var res struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := doJSON(ctx, http.MethodPost, instance+"/api/v1/statuses", headers, payload, &res); err != nil {
		return content.Result{}, err
	}
	return content.Result{Platform: "mastodon", URL: res.URL, ID: res.ID}, nil
}
