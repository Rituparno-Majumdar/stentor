package platforms

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type reddit struct{}

func NewReddit() Platform { return reddit{} }

func (reddit) Name() string { return "reddit" }
func (reddit) Tier() Tier   { return TierSelfServe }
func (reddit) FrictionNote() string {
	return "Blocked as of late 2025: Reddit closed self-service app creation and now requires a manual, unpredictable " +
		"\"Responsible Builder Policy\" review before issuing new OAuth credentials. This tool's code works; getting " +
		"credentials approved is entirely Reddit's own process, with no timeline and no guarantee."
}

func (reddit) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("reddit", "access_token") != "" && cfg.Get("reddit", "default_subreddit") != ""
}

func (reddit) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	fmt.Println("This requires an approved Reddit app -- see this platform's note in `stentor platforms`.")
	clientID, err := prompt(in, "Client ID: ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "Client secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URI (must match your app's exactly, e.g. http://localhost:8080): ")
	if err != nil {
		return err
	}
	username, err := prompt(in, "Your Reddit username (used in the required User-Agent header): ")
	if err != nil {
		return err
	}
	defaultSub, err := prompt(in, "Default subreddit to post to (without r/, e.g. selfhosted): ")
	if err != nil {
		return err
	}

	authURL := "https://www.reddit.com/api/v1/authorize?" + url.Values{
		"client_id":     {clientID},
		"response_type": {"code"},
		"state":         {"stentor"},
		"redirect_uri":  {redirectURI},
		"duration":      {"permanent"},
		"scope":         {"submit identity"},
	}.Encode()
	fmt.Println("\nOpen this URL, log in, click Allow:")
	fmt.Println(authURL)
	fmt.Println("\nYou'll land on your redirect URI with ?code=... (and &state=stentor) in the address bar.")
	fmt.Println("Copy just the code value -- it's single-use and expires in about 10 minutes.")

	code, err := prompt(in, "\nPaste the code: ")
	if err != nil {
		return err
	}

	basicAuth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	userAgent := fmt.Sprintf("stentor-cli/0.1 (by /u/%s)", username)

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	if err := doForm(ctx, http.MethodPost, "https://www.reddit.com/api/v1/access_token",
		map[string]string{"Authorization": "Basic " + basicAuth, "User-Agent": userAgent}, form, &tok); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	cfg.Set("reddit", "access_token", tok.AccessToken)
	cfg.Set("reddit", "refresh_token", tok.RefreshToken)
	cfg.Set("reddit", "user_agent", userAgent)
	cfg.Set("reddit", "default_subreddit", defaultSub)
	fmt.Println("Verified.")
	return nil
}

func (reddit) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	token := cfg.Get("reddit", "access_token")
	sub := cfg.Get("reddit", "default_subreddit")
	ua := cfg.Get("reddit", "user_agent")
	if token == "" || sub == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth reddit")
	}
	if c.Image != "" {
		return content.Result{}, fmt.Errorf("reddit: image/link posts aren't implemented yet, only self (text) posts")
	}

	title := c.Text
	if idx := strings.IndexByte(title, '\n'); idx != -1 {
		title = title[:idx]
	}
	if len(title) > 300 {
		title = title[:297] + "..."
	}
	body := c.Text
	if c.Link != "" {
		body = body + "\n\n" + c.Link
	}

	form := url.Values{
		"sr":       {sub},
		"title":    {title},
		"kind":     {"self"},
		"text":     {body},
		"api_type": {"json"},
	}
	var res struct {
		JSON struct {
			Errors [][]string `json:"errors"`
			Data   struct {
				URL string `json:"url"`
				ID  string `json:"id"`
			} `json:"data"`
		} `json:"json"`
	}
	headers := map[string]string{"Authorization": "Bearer " + token, "User-Agent": ua}
	if err := doForm(ctx, http.MethodPost, "https://oauth.reddit.com/api/submit", headers, form, &res); err != nil {
		return content.Result{}, err
	}
	if len(res.JSON.Errors) > 0 {
		return content.Result{}, fmt.Errorf("reddit: %v", res.JSON.Errors[0])
	}
	return content.Result{Platform: "reddit", URL: res.JSON.Data.URL, ID: res.JSON.Data.ID}, nil
}
