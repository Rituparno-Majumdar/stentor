package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type telegram struct{}

func NewTelegram() Platform { return telegram{} }

func (telegram) Name() string { return "telegram" }
func (telegram) Tier() Tier   { return TierFrictionless }
func (telegram) FrictionNote() string {
	return "None -- create a bot with @BotFather, no review."
}

func (telegram) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("telegram", "bot_token") != "" && cfg.Get("telegram", "chat_id") != ""
}

func (telegram) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	token, err := prompt(in, "Bot token (from @BotFather): ")
	if err != nil {
		return err
	}
	chatID, err := prompt(in, "Chat ID to post into (your own channel/group/DM id): ")
	if err != nil {
		return err
	}

	var me struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := doJSON(ctx, http.MethodGet, "https://api.telegram.org/bot"+token+"/getMe", nil, nil, &me); err != nil {
		return fmt.Errorf("token did not verify: %w", err)
	}
	if !me.OK {
		return fmt.Errorf("telegram rejected the token")
	}

	cfg.Set("telegram", "bot_token", token)
	cfg.Set("telegram", "chat_id", chatID)
	fmt.Printf("Verified bot @%s\n", me.Result.Username)
	return nil
}

func (telegram) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	token := cfg.Get("telegram", "bot_token")
	chatID := cfg.Get("telegram", "chat_id")
	if token == "" || chatID == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth telegram")
	}

	text := c.Text
	if c.Link != "" {
		text = text + "\n\n" + c.Link
	}

	base := "https://api.telegram.org/bot" + token
	var res struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}

	if c.Image != "" {
		fields := map[string]string{"chat_id": chatID, "caption": text}
		data, status, err := doMultipart(ctx, base+"/sendPhoto", nil, fields, "photo", c.Image)
		if err != nil {
			return content.Result{}, err
		}
		if status >= 300 {
			return content.Result{}, fmt.Errorf("sendPhoto http %d: %s", status, truncate(string(data), 300))
		}
		if err := jsonUnmarshal(data, &res); err != nil {
			return content.Result{}, err
		}
	} else {
		payload := map[string]string{"chat_id": chatID, "text": text}
		if err := doJSON(ctx, http.MethodPost, base+"/sendMessage", nil, payload, &res); err != nil {
			return content.Result{}, err
		}
	}
	if !res.OK {
		return content.Result{}, fmt.Errorf("telegram reported failure")
	}
	return content.Result{Platform: "telegram", ID: fmt.Sprintf("%d", res.Result.MessageID)}, nil
}
