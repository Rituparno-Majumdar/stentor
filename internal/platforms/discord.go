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

type discord struct{}

func NewDiscord() Platform { return discord{} }

func (discord) Name() string { return "discord" }
func (discord) Tier() Tier   { return TierFrictionless }
func (discord) FrictionNote() string {
	return "None -- create a webhook yourself on a channel you admin (Channel Settings -> Integrations -> Webhooks), no review."
}

func (discord) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("discord", "webhook_url") != ""
}

func (discord) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	webhookURL, err := prompt(in, "Webhook URL (Channel Settings -> Integrations -> Webhooks): ")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(webhookURL, "https://discord.com/api/webhooks/") &&
		!strings.HasPrefix(webhookURL, "https://discordapp.com/api/webhooks/") {
		return fmt.Errorf("that doesn't look like a Discord webhook URL")
	}
	data, status, err := doRequest(ctx, http.MethodGet, webhookURL, nil, nil)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("webhook did not verify, http %d: %s", status, truncate(string(data), 300))
	}
	cfg.Set("discord", "webhook_url", webhookURL)
	fmt.Println("Webhook verified")
	return nil
}

func (discord) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	webhookURL := cfg.Get("discord", "webhook_url")
	if webhookURL == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth discord")
	}

	text := c.Text
	if c.Link != "" {
		text = text + "\n\n" + c.Link
	}

	if c.Image != "" {
		data, status, err := doMultipart(ctx, webhookURL+"?wait=true", nil, map[string]string{"content": text}, "file", c.Image)
		if err != nil {
			return content.Result{}, err
		}
		if status >= 300 {
			return content.Result{}, fmt.Errorf("discord http %d: %s", status, truncate(string(data), 300))
		}
		var res struct {
			ID string `json:"id"`
		}
		_ = jsonUnmarshal(data, &res)
		return content.Result{Platform: "discord", ID: res.ID}, nil
	}

	var res struct {
		ID string `json:"id"`
	}
	if err := doJSON(ctx, http.MethodPost, webhookURL+"?wait=true", nil, map[string]string{"content": text}, &res); err != nil {
		return content.Result{}, err
	}
	return content.Result{Platform: "discord", ID: res.ID}, nil
}
