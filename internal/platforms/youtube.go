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

type youtube struct{}

func NewYouTube() Platform { return youtube{} }

func (youtube) Name() string { return "youtube" }
func (youtube) Tier() Tier   { return TierPlatformReview }
func (youtube) FrictionNote() string {
	return "Google Cloud OAuth consent screen verification required for anyone other than you. Also: YouTube's " +
		"API has no endpoint for plain-text or Community posts at all -- only video upload -- so `stentor post` " +
		"to this platform will always fail by design; this tool doesn't do video."
}

func (youtube) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("youtube", "refresh_token") != ""
}

func (youtube) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	clientID, err := prompt(in, "OAuth client ID (Google Cloud Console): ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "OAuth client secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URI (must match your OAuth client's exactly): ")
	if err != nil {
		return err
	}

	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/youtube.upload"},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}.Encode()
	fmt.Println("\nOpen this URL, log in, click Allow:")
	fmt.Println(authURL)
	fmt.Println("\nNote up front: this only proves auth works. `stentor post --platform youtube` will still " +
		"fail afterward -- YouTube has no text-post API, see `stentor platforms`.")
	code, err := prompt(in, "\nPaste the code from the redirected URL: ")
	if err != nil {
		return err
	}

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	if err := doForm(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", nil, form, &tok); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	if tok.RefreshToken == "" {
		return fmt.Errorf("no refresh token returned -- revoke this app's prior access at " +
			"myaccount.google.com/permissions and try again")
	}

	cfg.Set("youtube", "client_id", clientID)
	cfg.Set("youtube", "client_secret", clientSecret)
	cfg.Set("youtube", "refresh_token", tok.RefreshToken)
	fmt.Println("Verified (auth only -- see the note above about posting).")
	return nil
}

func (youtube) Post(_ context.Context, _ *config.Store, _ content.Content) (content.Result, error) {
	return content.Result{}, fmt.Errorf("youtube: not supported -- the YouTube Data API has no endpoint for " +
		"plain-text or Community posts, only video uploads, and this tool only posts text/images/links")
}
