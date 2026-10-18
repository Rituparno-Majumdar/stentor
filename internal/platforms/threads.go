package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type threads struct{}

func NewThreads() Platform { return threads{} }

func (threads) Name() string { return "threads" }
func (threads) Tier() Tier   { return TierPlatformReview }
func (threads) FrictionNote() string {
	return "Its own Meta app product (\"Threads API\", separate from the Facebook/Instagram app) plus Meta's " +
		"own review for threads_content_publish beyond your own test account."
}

func (threads) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("threads", "access_token") != "" && cfg.Get("threads", "user_id") != ""
}

func (threads) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	clientID, err := prompt(in, "Threads App ID: ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "Threads App secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URI (must match your app's exactly): ")
	if err != nil {
		return err
	}

	authURL := "https://threads.net/oauth/authorize?" + url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"threads_basic,threads_content_publish"},
		"response_type": {"code"},
	}.Encode()
	fmt.Println("\nOpen this URL, log in, click Allow:")
	fmt.Println(authURL)
	code, err := prompt(in, "\nPaste the code from the redirected URL: ")
	if err != nil {
		return err
	}

	var shortTok struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
		"code":          {code},
	}
	if err := doForm(ctx, http.MethodPost, "https://graph.threads.net/oauth/access_token", nil, form, &shortTok); err != nil {
		return fmt.Errorf("code exchange failed: %w", err)
	}

	var longTok struct {
		AccessToken string `json:"access_token"`
	}
	longURL := "https://graph.threads.net/access_token?" + url.Values{
		"grant_type":    {"th_exchange_token"},
		"client_secret": {clientSecret},
		"access_token":  {shortTok.AccessToken},
	}.Encode()
	if err := doJSON(ctx, http.MethodGet, longURL, nil, nil, &longTok); err != nil {
		return fmt.Errorf("long-lived token exchange failed: %w", err)
	}

	cfg.Set("threads", "access_token", longTok.AccessToken)
	cfg.Set("threads", "user_id", shortTok.UserID)
	fmt.Println("Verified.")
	return nil
}

func (threads) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	userID := cfg.Get("threads", "user_id")
	token := cfg.Get("threads", "access_token")
	if userID == "" || token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth threads")
	}

	text := c.Text
	if c.Link != "" {
		text = text + "\n\n" + c.Link
	}

	form := url.Values{"access_token": {token}}
	if c.Image != "" {
		if !strings.HasPrefix(c.Image, "http://") && !strings.HasPrefix(c.Image, "https://") {
			return content.Result{}, fmt.Errorf("threads requires --image to be a public http(s) URL, not a local path")
		}
		form.Set("media_type", "IMAGE")
		form.Set("image_url", c.Image)
		form.Set("text", text)
	} else {
		form.Set("media_type", "TEXT")
		form.Set("text", text)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := doForm(ctx, http.MethodPost, "https://graph.threads.net/v1.0/"+userID+"/threads", nil, form, &created); err != nil {
		return content.Result{}, fmt.Errorf("creating container: %w", err)
	}

	time.Sleep(2 * time.Second)

	var published struct {
		ID string `json:"id"`
	}
	pubForm := url.Values{"creation_id": {created.ID}, "access_token": {token}}
	if err := doForm(ctx, http.MethodPost, "https://graph.threads.net/v1.0/"+userID+"/threads_publish", nil, pubForm, &published); err != nil {
		return content.Result{}, fmt.Errorf("publishing: %w", err)
	}
	return content.Result{Platform: "threads", ID: published.ID}, nil
}
