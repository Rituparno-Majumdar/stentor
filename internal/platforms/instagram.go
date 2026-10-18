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

type instagram struct{}

func NewInstagram() Platform { return instagram{} }

func (instagram) Name() string { return "instagram" }
func (instagram) Tier() Tier   { return TierPlatformReview }
func (instagram) FrictionNote() string {
	return "Rides on the same Meta app as facebook (run `stentor auth facebook` first) plus an Instagram " +
		"Business account linked to that Page, plus Meta App Review for instagram_content_publish. The Graph " +
		"API also requires images be a public image_url, not a local file -- pass --image as an http(s) URL."
}

func (instagram) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("instagram", "ig_user_id") != ""
}

func (instagram) Authenticate(ctx context.Context, cfg *config.Store, _ *bufio.Reader) error {
	pageID := cfg.Get("facebook", "page_id")
	token := cfg.Get("facebook", "page_access_token")
	if pageID == "" || token == "" {
		return fmt.Errorf("run `stentor auth facebook` first -- instagram reuses that Page's token")
	}

	var res struct {
		InstagramBusinessAccount struct {
			ID string `json:"id"`
		} `json:"instagram_business_account"`
	}
	fetchURL := "https://graph.facebook.com/" + metaGraphVersion + "/" + pageID +
		"?fields=instagram_business_account&access_token=" + url.QueryEscape(token)
	if err := doJSON(ctx, http.MethodGet, fetchURL, nil, nil, &res); err != nil {
		return fmt.Errorf("fetching linked IG account: %w", err)
	}
	if res.InstagramBusinessAccount.ID == "" {
		return fmt.Errorf("that Page has no linked Instagram Business account")
	}

	cfg.Set("instagram", "ig_user_id", res.InstagramBusinessAccount.ID)
	fmt.Printf("Verified. Instagram Business account id: %s\n", res.InstagramBusinessAccount.ID)
	return nil
}

func (instagram) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	igUserID := cfg.Get("instagram", "ig_user_id")
	token := cfg.Get("facebook", "page_access_token")
	if igUserID == "" || token == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth instagram")
	}
	if c.Image == "" {
		return content.Result{}, fmt.Errorf("instagram requires an image -- it has no plain-text post type")
	}
	if !strings.HasPrefix(c.Image, "http://") && !strings.HasPrefix(c.Image, "https://") {
		return content.Result{}, fmt.Errorf("instagram requires --image to be a public http(s) URL, not a local path " +
			"(the Graph API fetches it server-side)")
	}

	var created struct {
		ID string `json:"id"`
	}
	form := url.Values{"image_url": {c.Image}, "caption": {c.Text}, "access_token": {token}}
	if err := doForm(ctx, http.MethodPost, "https://graph.facebook.com/"+metaGraphVersion+"/"+igUserID+"/media", nil, form, &created); err != nil {
		return content.Result{}, fmt.Errorf("creating media container: %w", err)
	}

	// The container needs a moment to finish fetching the image server-side.
	time.Sleep(2 * time.Second)

	var published struct {
		ID string `json:"id"`
	}
	pubForm := url.Values{"creation_id": {created.ID}, "access_token": {token}}
	if err := doForm(ctx, http.MethodPost, "https://graph.facebook.com/"+metaGraphVersion+"/"+igUserID+"/media_publish", nil, pubForm, &published); err != nil {
		return content.Result{}, fmt.Errorf("publishing media: %w", err)
	}
	return content.Result{Platform: "instagram", ID: published.ID}, nil
}
