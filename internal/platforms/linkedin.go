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

const linkedinAPIVersion = "202504"

type linkedin struct{}

func NewLinkedIn() Platform { return linkedin{} }

func (linkedin) Name() string { return "linkedin" }
func (linkedin) Tier() Tier   { return TierSelfServe }
func (linkedin) FrictionNote() string {
	return "Self-serve: create a LinkedIn app linked to a Page you admin, request the \"Share on LinkedIn\" product, no further review."
}

func (linkedin) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("linkedin", "access_token") != "" && cfg.Get("linkedin", "person_urn") != ""
}

func (linkedin) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	clientID, err := prompt(in, "Client ID (from your LinkedIn app's Auth tab): ")
	if err != nil {
		return err
	}
	clientSecret, err := prompt(in, "Client secret: ")
	if err != nil {
		return err
	}
	redirectURI, err := prompt(in, "Redirect URL (must exactly match one on your app's Auth tab, e.g. http://localhost:8080/callback): ")
	if err != nil {
		return err
	}

	authURL := "https://www.linkedin.com/oauth/v2/authorization?" + url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {"openid profile w_member_social"},
	}.Encode()
	fmt.Println("\nOpen this URL, log in, click Allow:")
	fmt.Println(authURL)
	fmt.Println("\nYou'll land on your redirect URL with ?code=... in the address bar")
	fmt.Println("(the page itself won't load -- that's expected). Copy just the code value.")
	fmt.Println("It expires in under a minute, so do this step right now.")

	code, err := prompt(in, "\nPaste the code: ")
	if err != nil {
		return err
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	if err := doForm(ctx, http.MethodPost, "https://www.linkedin.com/oauth/v2/accessToken", nil, form, &tok); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	var userinfo struct {
		Sub string `json:"sub"`
	}
	if err := doJSON(ctx, http.MethodGet, "https://api.linkedin.com/v2/userinfo",
		map[string]string{"Authorization": "Bearer " + tok.AccessToken}, nil, &userinfo); err != nil {
		return fmt.Errorf("fetching userinfo: %w", err)
	}

	cfg.Set("linkedin", "access_token", tok.AccessToken)
	cfg.Set("linkedin", "person_urn", "urn:li:person:"+userinfo.Sub)
	fmt.Println("Verified. Note: LinkedIn access tokens last ~60 days; re-run this auth step when it expires.")
	return nil
}

func (linkedin) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	token := cfg.Get("linkedin", "access_token")
	author := cfg.Get("linkedin", "person_urn")
	if token == "" || author == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth linkedin")
	}

	headers := map[string]string{
		"Authorization":           "Bearer " + token,
		"LinkedIn-Version":        linkedinAPIVersion,
		"X-Restli-Protocol-Version": "2.0.0",
	}

	commentary := c.Text
	if c.Link != "" {
		commentary = commentary + "\n\n" + c.Link
	}

	payload := map[string]any{
		"author":      author,
		"commentary":  commentary,
		"visibility":  "PUBLIC",
		"distribution": map[string]any{
			"feedDistribution":                "MAIN_FEED",
			"targetEntities":                  []any{},
			"thirdPartyDistributionChannels":  []any{},
		},
		"lifecycleState":            "PUBLISHED",
		"isReshareDisabledByAuthor": false,
	}

	if c.Image != "" {
		imageURN, err := linkedinUploadImage(ctx, token, author, c.Image)
		if err != nil {
			return content.Result{}, fmt.Errorf("uploading image: %w", err)
		}
		payload["content"] = map[string]any{"media": map[string]any{"id": imageURN}}
	}

	h := map[string]string{"Content-Type": "application/json"}
	for k, v := range headers {
		h[k] = v
	}
	data, status, err := doRequestJSON(ctx, http.MethodPost, "https://api.linkedin.com/rest/posts", h, payload)
	if err != nil {
		return content.Result{}, err
	}
	if status < 200 || status >= 300 {
		return content.Result{}, fmt.Errorf("linkedin http %d: %s", status, truncate(string(data), 500))
	}
	return content.Result{Platform: "linkedin"}, nil
}

func linkedinUploadImage(ctx context.Context, token, owner, path string) (string, error) {
	var init struct {
		Value struct {
			UploadURL string `json:"uploadUrl"`
			Image     string `json:"image"`
		} `json:"value"`
	}
	initPayload := map[string]any{
		"initializeUploadRequest": map[string]any{"owner": owner},
	}
	if err := doJSON(ctx, http.MethodPost, "https://api.linkedin.com/rest/images?action=initializeUpload",
		map[string]string{"Authorization": "Bearer " + token, "LinkedIn-Version": linkedinAPIVersion},
		initPayload, &init); err != nil {
		return "", err
	}

	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	_, status, err := doRequest(ctx, http.MethodPut, init.Value.UploadURL,
		map[string]string{"Authorization": "Bearer " + token}, byteReader(data))
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("image upload http %d", status)
	}
	return init.Value.Image, nil
}
