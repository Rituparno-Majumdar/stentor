package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

const twitterPostURL = "https://api.twitter.com/2/tweets"

type twitter struct{}

func NewTwitter() Platform { return twitter{} }

func (twitter) Name() string { return "twitter" }
func (twitter) Tier() Tier   { return TierSelfServe }
func (twitter) FrictionNote() string {
	return "Self-serve app registration, but as of 2026 reports conflict on whether free write access still exists " +
		"(either a 1,500 posts/month free cap, or pay-per-post around $0.01 each) -- confirm current pricing on " +
		"the X developer portal before relying on this."
}

func (twitter) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("twitter", "access_token") != "" && cfg.Get("twitter", "access_token_secret") != ""
}

func (twitter) Authenticate(_ context.Context, cfg *config.Store, in *bufio.Reader) error {
	fmt.Println("Uses the classic OAuth 1.0a User Context credentials (consumer key/secret + access token/secret),")
	fmt.Println("all four generated in the X Developer Portal under your app's \"Keys and tokens\" tab.")
	consumerKey, err := prompt(in, "Consumer (API) key: ")
	if err != nil {
		return err
	}
	consumerSecret, err := prompt(in, "Consumer (API) secret: ")
	if err != nil {
		return err
	}
	accessToken, err := prompt(in, "Access token: ")
	if err != nil {
		return err
	}
	accessTokenSecret, err := prompt(in, "Access token secret: ")
	if err != nil {
		return err
	}
	cfg.Set("twitter", "consumer_key", consumerKey)
	cfg.Set("twitter", "consumer_secret", consumerSecret)
	cfg.Set("twitter", "access_token", accessToken)
	cfg.Set("twitter", "access_token_secret", accessTokenSecret)
	fmt.Println("Saved. (Not verified against a live call here -- X has no cheap read-only endpoint to confirm " +
		"against without burning quota; the first `post` will tell you if something's wrong.)")
	return nil
}

func (twitter) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	consumerKey := cfg.Get("twitter", "consumer_key")
	consumerSecret := cfg.Get("twitter", "consumer_secret")
	accessToken := cfg.Get("twitter", "access_token")
	accessTokenSecret := cfg.Get("twitter", "access_token_secret")
	if consumerKey == "" || accessToken == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth twitter")
	}
	if c.Image != "" {
		return content.Result{}, fmt.Errorf("twitter: image upload isn't implemented yet, text/link only")
	}

	text := c.Text
	if c.Link != "" {
		text = text + " " + c.Link
	}

	authHeader, err := oauth1Sign(http.MethodPost, twitterPostURL, consumerKey, consumerSecret, accessToken, accessTokenSecret)
	if err != nil {
		return content.Result{}, err
	}

	var res struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := doJSON(ctx, http.MethodPost, twitterPostURL,
		map[string]string{"Authorization": authHeader}, map[string]string{"text": text}, &res); err != nil {
		return content.Result{}, err
	}
	if len(res.Errors) > 0 {
		return content.Result{}, fmt.Errorf("twitter: %s", res.Errors[0].Message)
	}
	return content.Result{Platform: "twitter", ID: res.Data.ID}, nil
}
