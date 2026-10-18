package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

const blueskyHost = "https://bsky.social"

type bluesky struct{}

func NewBluesky() Platform { return bluesky{} }

func (bluesky) Name() string { return "bluesky" }
func (bluesky) Tier() Tier   { return TierFrictionless }
func (bluesky) FrictionNote() string {
	return "None -- generate an app password yourself in Settings -> App Passwords, no developer app or review of any kind."
}

func (bluesky) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("bluesky", "access_jwt") != "" && cfg.Get("bluesky", "did") != ""
}

func (bluesky) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	handle, err := prompt(in, "Bluesky handle (e.g. you.bsky.social): ")
	if err != nil {
		return err
	}
	appPassword, err := prompt(in, "App password (Settings -> App Passwords, NOT your real password): ")
	if err != nil {
		return err
	}

	var session struct {
		AccessJWT  string `json:"accessJwt"`
		RefreshJWT string `json:"refreshJwt"`
		DID        string `json:"did"`
		Handle     string `json:"handle"`
	}
	payload := map[string]string{"identifier": handle, "password": appPassword}
	if err := doJSON(ctx, http.MethodPost, blueskyHost+"/xrpc/com.atproto.server.createSession", nil, payload, &session); err != nil {
		return fmt.Errorf("createSession failed: %w", err)
	}

	cfg.Set("bluesky", "handle", session.Handle)
	cfg.Set("bluesky", "did", session.DID)
	cfg.Set("bluesky", "access_jwt", session.AccessJWT)
	cfg.Set("bluesky", "refresh_jwt", session.RefreshJWT)
	fmt.Printf("Verified as %s (%s)\n", session.Handle, session.DID)
	return nil
}

func (bluesky) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	did := cfg.Get("bluesky", "did")
	token := cfg.Get("bluesky", "access_jwt")
	if did == "" || token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth bluesky")
	}
	headers := map[string]string{"Authorization": "Bearer " + token}

	text := c.Text
	if c.Link != "" {
		text = text + "\n\n" + c.Link
	}
	if len(text) > 300 {
		text = text[:297] + "..."
	}

	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}

	if c.Image != "" {
		data, err := readFile(c.Image)
		if err != nil {
			return content.Result{}, fmt.Errorf("reading image: %w", err)
		}
		var blob struct {
			Blob any `json:"blob"`
		}
		imgHeaders := map[string]string{"Authorization": "Bearer " + token, "Content-Type": "image/jpeg"}
		respData, status, err := doRequest(ctx, http.MethodPost, blueskyHost+"/xrpc/com.atproto.repo.uploadBlob", imgHeaders, byteReader(data))
		if err != nil {
			return content.Result{}, fmt.Errorf("uploading image: %w", err)
		}
		if status >= 300 {
			return content.Result{}, fmt.Errorf("uploadBlob http %d: %s", status, truncate(string(respData), 300))
		}
		if err := jsonUnmarshal(respData, &blob); err != nil {
			return content.Result{}, err
		}
		record["embed"] = map[string]any{
			"$type": "app.bsky.embed.images",
			"images": []map[string]any{
				{"alt": "", "image": blob.Blob},
			},
		}
	}

	payload := map[string]any{
		"repo":       did,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}
	var res struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := doJSON(ctx, http.MethodPost, blueskyHost+"/xrpc/com.atproto.repo.createRecord", headers, payload, &res); err != nil {
		return content.Result{}, err
	}
	return content.Result{Platform: "bluesky", ID: res.URI}, nil
}
