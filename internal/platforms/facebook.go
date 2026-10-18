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

const metaGraphVersion = "v21.0"

type facebook struct{}

func NewFacebook() Platform { return facebook{} }

func (facebook) Name() string { return "facebook" }
func (facebook) Tier() Tier   { return TierPlatformReview }
func (facebook) FrictionNote() string {
	return "Create a Meta app, then it needs Meta's own App Review for pages_manage_posts before you can post " +
		"to a real Page (your own test Pages work without review while in Development mode)."
}

func (facebook) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("facebook", "page_access_token") != "" && cfg.Get("facebook", "page_id") != ""
}

func (facebook) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	clientID, err := prompt(in, "App ID: ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "App secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URI (must exactly match your app's Valid OAuth Redirect URIs): ")
	if err != nil {
		return err
	}

	authURL := "https://www.facebook.com/" + metaGraphVersion + "/dialog/oauth?" + url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"pages_manage_posts,pages_read_engagement"},
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
	}
	exchangeURL := "https://graph.facebook.com/" + metaGraphVersion + "/oauth/access_token?" + url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"client_secret": {clientSecret},
		"code":          {code},
	}.Encode()
	if err := doJSON(ctx, http.MethodGet, exchangeURL, nil, nil, &shortTok); err != nil {
		return fmt.Errorf("code exchange failed: %w", err)
	}

	var longTok struct {
		AccessToken string `json:"access_token"`
	}
	longURL := "https://graph.facebook.com/" + metaGraphVersion + "/oauth/access_token?" + url.Values{
		"grant_type":        {"fb_exchange_token"},
		"client_id":         {clientID},
		"client_secret":     {clientSecret},
		"fb_exchange_token": {shortTok.AccessToken},
	}.Encode()
	if err := doJSON(ctx, http.MethodGet, longURL, nil, nil, &longTok); err != nil {
		return fmt.Errorf("long-lived token exchange failed: %w", err)
	}

	var pages struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	pagesURL := "https://graph.facebook.com/" + metaGraphVersion + "/me/accounts?access_token=" + url.QueryEscape(longTok.AccessToken)
	if err := doJSON(ctx, http.MethodGet, pagesURL, nil, nil, &pages); err != nil {
		return fmt.Errorf("listing pages failed: %w", err)
	}
	if len(pages.Data) == 0 {
		return fmt.Errorf("no Facebook Pages found for this account -- you need to admin at least one Page")
	}

	chosen := pages.Data[0]
	if len(pages.Data) > 1 {
		fmt.Println("Pages found:")
		for _, p := range pages.Data {
			fmt.Printf("  %s  %s\n", p.ID, p.Name)
		}
		pageID, err := prompt(in, "Page ID to post as (blank = first listed): ")
		if err != nil {
			return err
		}
		if pageID != "" {
			for _, p := range pages.Data {
				if p.ID == pageID {
					chosen = p
				}
			}
		}
	}

	cfg.Set("facebook", "user_access_token", longTok.AccessToken)
	cfg.Set("facebook", "page_id", chosen.ID)
	cfg.Set("facebook", "page_access_token", chosen.AccessToken)
	fmt.Printf("Verified. Posting as Page %q (%s)\n", chosen.Name, chosen.ID)
	return nil
}

func (facebook) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	pageID := cfg.Get("facebook", "page_id")
	token := cfg.Get("facebook", "page_access_token")
	if pageID == "" || token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth facebook")
	}

	if c.Image != "" {
		fields := map[string]string{"caption": c.Text, "access_token": token}
		data, status, err := doMultipart(ctx, "https://graph.facebook.com/"+metaGraphVersion+"/"+pageID+"/photos", nil, fields, "source", c.Image)
		if err != nil {
			return content.Result{}, err
		}
		if status >= 300 {
			return content.Result{}, fmt.Errorf("facebook http %d: %s", status, truncate(string(data), 400))
		}
		var res struct {
			ID string `json:"post_id"`
		}
		_ = jsonUnmarshal(data, &res)
		return content.Result{Platform: "facebook", ID: res.ID}, nil
	}

	form := url.Values{"message": {c.Text}, "access_token": {token}}
	if c.Link != "" {
		form.Set("link", c.Link)
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := doForm(ctx, http.MethodPost, "https://graph.facebook.com/"+metaGraphVersion+"/"+pageID+"/feed", nil, form, &res); err != nil {
		return content.Result{}, err
	}
	return content.Result{Platform: "facebook", ID: res.ID}, nil
}
