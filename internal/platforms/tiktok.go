package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type tiktok struct{}

func NewTikTok() Platform { return tiktok{} }

func (tiktok) Name() string { return "tiktok" }
func (tiktok) Tier() Tier   { return TierPlatformReview }
func (tiktok) FrictionNote() string {
	return "Needs your own TikTok Developer app approved for the Content Posting API. TikTok is fundamentally " +
		"video/photo-first -- there's no plain-text post type -- so `stentor post` here requires --image and " +
		"posts it as a single-photo post with your text as the caption."
}

func (tiktok) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("tiktok", "access_token") != "" && cfg.Get("tiktok", "open_id") != ""
}

func (tiktok) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	clientKey, err := prompt(in, "Client key: ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "Client secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URI (must match your app's exactly): ")
	if err != nil {
		return err
	}

	authURL := "https://www.tiktok.com/v2/auth/authorize?" + url.Values{
		"client_key":    {clientKey},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"user.info.basic,video.publish"},
		"state":         {"stentor"},
	}.Encode()
	fmt.Println("\nOpen this URL, log in, click Allow:")
	fmt.Println(authURL)
	code, err := prompt(in, "\nPaste the code from the redirected URL: ")
	if err != nil {
		return err
	}

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		OpenID       string `json:"open_id"`
	}
	form := url.Values{
		"client_key":    {clientKey},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
	if err := doForm(ctx, http.MethodPost, "https://open.tiktokapis.com/v2/oauth/token/", nil, form, &tok); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	cfg.Set("tiktok", "access_token", tok.AccessToken)
	cfg.Set("tiktok", "refresh_token", tok.RefreshToken)
	cfg.Set("tiktok", "open_id", tok.OpenID)
	fmt.Println("Verified.")
	return nil
}

// Post implements TikTok's Content Posting API photo mode. This is the
// least-certain integration in this tool -- TikTok's photo/carousel
// endpoint is documented less stably than its video endpoint. Verify
// against TikTok's current docs before relying on this in production.
func (tiktok) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	token := cfg.Get("tiktok", "access_token")
	if token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth tiktok")
	}
	if c.Image == "" {
		return content.Result{}, fmt.Errorf("tiktok has no plain-text post type -- pass --image for a photo post")
	}

	headers := map[string]string{"Authorization": "Bearer " + token}

	data, err := readFile(c.Image)
	if err != nil {
		return content.Result{}, err
	}

	initPayload := map[string]any{
		"post_info": map[string]any{
			"title":           c.Text,
			"privacy_level":   "SELF_ONLY",
			"disable_comment": false,
		},
		"source_info": map[string]any{
			"source":            "FILE_UPLOAD",
			"photo_cover_index": 0,
			"photo_images":      []string{},
		},
		"post_mode":  "DIRECT_POST",
		"media_type": "PHOTO",
	}
	var init struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := doJSON(ctx, http.MethodPost, "https://open.tiktokapis.com/v2/post/publish/content/init/", headers, initPayload, &init); err != nil {
		return content.Result{}, fmt.Errorf("init failed: %w", err)
	}
	if init.Error.Code != "" && init.Error.Code != "ok" {
		return content.Result{}, fmt.Errorf("tiktok init error: %s", init.Error.Message)
	}

	if init.Data.UploadURL != "" {
		_, status, err := doRequest(ctx, http.MethodPut, init.Data.UploadURL, map[string]string{"Content-Type": "image/jpeg"}, byteReader(data))
		if err != nil {
			return content.Result{}, fmt.Errorf("uploading photo: %w", err)
		}
		if status >= 300 {
			return content.Result{}, fmt.Errorf("photo upload http %d", status)
		}
	}

	return content.Result{Platform: "tiktok", ID: init.Data.PublishID}, nil
}
