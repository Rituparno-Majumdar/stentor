package platforms

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

const hashnodeAPI = "https://gql.hashnode.com/"

type hashnode struct{}

func NewHashnode() Platform { return hashnode{} }

func (hashnode) Name() string { return "hashnode" }
func (hashnode) Tier() Tier   { return TierFrictionless }
func (hashnode) FrictionNote() string {
	return "None -- generate a personal access token yourself in your Hashnode account settings, no review."
}

func (hashnode) IsAuthenticated(cfg *config.Store) bool {
	return cfg.Get("hashnode", "token") != "" && cfg.Get("hashnode", "publication_id") != ""
}

func (hashnode) Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error {
	token, err := prompt(in, "Personal access token (Account Settings -> Developer): ")
	if err != nil {
		return err
	}

	query := `query { me { username publications(first: 1) { edges { node { id } } } } }`
	var resp struct {
		Data struct {
			Me struct {
				Username     string `json:"username"`
				Publications struct {
					Edges []struct {
						Node struct {
							ID string `json:"id"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"publications"`
			} `json:"me"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := doJSON(ctx, http.MethodPost, hashnodeAPI, map[string]string{"Authorization": token},
		map[string]string{"query": query}, &resp); err != nil {
		return fmt.Errorf("token did not verify: %w", err)
	}
	if len(resp.Errors) > 0 {
		return fmt.Errorf("hashnode: %s", resp.Errors[0].Message)
	}
	if len(resp.Data.Me.Publications.Edges) == 0 {
		return fmt.Errorf("no publication found on this account -- create one on hashnode.com first")
	}

	cfg.Set("hashnode", "token", token)
	cfg.Set("hashnode", "publication_id", resp.Data.Me.Publications.Edges[0].Node.ID)
	fmt.Printf("Verified as @%s, publication %s\n", resp.Data.Me.Username, resp.Data.Me.Publications.Edges[0].Node.ID)
	return nil
}

func (hashnode) Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error) {
	token := cfg.Get("hashnode", "token")
	pubID := cfg.Get("hashnode", "publication_id")
	if token == "" || pubID == "" {
		return content.Result{}, fmt.Errorf("not authenticated -- run: stentor auth hashnode")
	}

	title := c.Text
	if idx := strings.IndexByte(title, '\n'); idx != -1 {
		title = title[:idx]
	}
	if len(title) > 100 {
		title = title[:97] + "..."
	}
	if strings.TrimSpace(title) == "" {
		title = "Untitled post"
	}
	body := c.Text
	if c.Link != "" {
		body = body + "\n\n" + c.Link
	}

	mutation := `mutation PublishPost($input: PublishPostInput!) {
		publishPost(input: $input) { post { id url } }
	}`
	variables := map[string]any{
		"input": map[string]any{
			"title":         title,
			"contentMarkdown": body,
			"publicationId": pubID,
		},
	}
	var resp struct {
		Data struct {
			PublishPost struct {
				Post struct {
					ID  string `json:"id"`
					URL string `json:"url"`
				} `json:"post"`
			} `json:"publishPost"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := doJSON(ctx, http.MethodPost, hashnodeAPI, map[string]string{"Authorization": token},
		map[string]any{"query": mutation, "variables": variables}, &resp); err != nil {
		return content.Result{}, err
	}
	if len(resp.Errors) > 0 {
		return content.Result{}, fmt.Errorf("hashnode: %s", resp.Errors[0].Message)
	}
	return content.Result{Platform: "hashnode", URL: resp.Data.PublishPost.Post.URL, ID: resp.Data.PublishPost.Post.ID}, nil
}
