package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

type lemmy struct{}

func NewLemmy() Platform { return lemmy{} }

func (lemmy) Name() string { return "lemmy" }
func (lemmy) Tier() Tier   { return TierFrictionless }
func (lemmy) FrictionNote() string {
	return "None -- any instance's registration is open and self-serve, no app review by anyone."
}

func (lemmy) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("lemmy", "jwt") != "" && cfg.Get("lemmy", "instance") != "" &&
		cfg.Get("lemmy", "default_community") != ""
}

func (lemmy) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	instance, err := prompt(in, "Lemmy instance URL (e.g. https://lemmy.world): ")
	if err != nil {
		return err
	}
	instance = strings.TrimRight(instance, "/")
	username, err := prompt(in, "Username or email: ")
	if err != nil {
		return err
	}
	password, err := prompt(in, "Password: ")
	if err != nil {
		return err
	}
	defaultCommunity, err := prompt(in, "Default community to post to (e.g. selfhosted, or name@remote.instance for a federated one): ")
	if err != nil {
		return err
	}

	var login struct {
		JWT string `json:"jwt"`
	}
	payload := map[string]string{"username_or_email": username, "password": password}
	if err := doJSON(ctx, http.MethodPost, instance+"/api/v3/user/login", nil, payload, &login); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if login.JWT == "" {
		return fmt.Errorf("login succeeded but no JWT was returned (2FA accounts aren't supported yet)")
	}

	cfg.Set("lemmy", "instance", instance)
	cfg.Set("lemmy", "jwt", login.JWT)
	cfg.Set("lemmy", "username", username)
	cfg.Set("lemmy", "default_community", defaultCommunity)
	fmt.Printf("Verified on %s\n", instance)
	return nil
}

func (lemmy) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	instance := cfg.Get("lemmy", "instance")
	jwt := cfg.Get("lemmy", "jwt")
	community := cfg.Get("lemmy", "default_community")
	if instance == "" || jwt == "" || community == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth lemmy")
	}
	if c.Image != "" {
		return content.Result{}, fmt.Errorf("lemmy: image posts aren't implemented yet, only text/link posts")
	}

	authHeader := map[string]string{"Authorization": "Bearer " + jwt}

	var communityLookup struct {
		CommunityView struct {
			Community struct {
				ID int64 `json:"id"`
			} `json:"community"`
		} `json:"community_view"`
	}
	lookupURL := instance + "/api/v3/community?" + url.Values{"name": {community}}.Encode()
	if err := doJSON(ctx, http.MethodGet, lookupURL, authHeader, nil, &communityLookup); err != nil {
		return content.Result{}, fmt.Errorf("looking up community %q: %w", community, err)
	}
	communityID := communityLookup.CommunityView.Community.ID
	if communityID == 0 {
		return content.Result{}, fmt.Errorf("community %q not found on %s", community, instance)
	}

	title := c.Text
	if idx := strings.IndexByte(title, '\n'); idx != -1 {
		title = title[:idx]
	}
	if len(title) > 200 {
		title = title[:197] + "..."
	}

	payload := map[string]any{
		"community_id": communityID,
		"name":         title,
		"body":         c.Text,
	}
	if c.Link != "" {
		payload["url"] = c.Link
	}

	var res struct {
		PostView struct {
			Post struct {
				ID int64 `json:"id"`
			} `json:"post"`
		} `json:"post_view"`
	}
	if err := doJSON(ctx, http.MethodPost, instance+"/api/v3/post", authHeader, payload, &res); err != nil {
		return content.Result{}, err
	}
	if res.PostView.Post.ID == 0 {
		return content.Result{}, fmt.Errorf("lemmy: post created but no id was returned")
	}

	return content.Result{
		Platform: "lemmy",
		URL:      fmt.Sprintf("%s/post/%d", instance, res.PostView.Post.ID),
		ID:       fmt.Sprintf("%d", res.PostView.Post.ID),
	}, nil
}
