package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"harnessforge/internal/github/domain"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"
const maxPages = 10
const maxBodyBytes = 16 << 10

var repositoryName = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type Client struct {
	baseURL string
	token   string
	client  *http.Client
	pages   int
}

func NewClient(token string) *Client {
	return NewClientWithOptions(defaultBaseURL, token, http.DefaultClient, maxPages)
}
func NewClientWithOptions(baseURL, token string, client *http.Client, pages int) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	if pages < 1 {
		pages = 1
	}
	if pages > maxPages {
		pages = maxPages
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: client, pages: pages}
}

func (c *Client) Read(ctx context.Context, repository string) ([]domain.Discussion, error) {
	if !repositoryName.MatchString(repository) {
		return nil, fmt.Errorf("repository must be owner/name using safe characters")
	}
	var all []domain.Discussion
	issues, err := c.list(ctx, "/repos/"+repository+"/issues?state=all")
	if err != nil {
		return nil, err
	}
	all = append(all, issues...)
	pulls, err := c.list(ctx, "/repos/"+repository+"/pulls?state=all")
	if err != nil {
		return nil, err
	}
	all = append(all, pulls...)
	for _, pull := range pulls {
		reviews, err := c.list(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", repository, pull.Number))
		if err != nil {
			return nil, err
		}
		for i := range reviews {
			reviews[i].Kind = "review"
			reviews[i].Revision = pull.Revision
		}
		all = append(all, reviews...)
	}
	commits, err := c.list(ctx, "/repos/"+repository+"/commits")
	if err != nil {
		return nil, err
	}
	all = append(all, commits...)
	return all, ctx.Err()
}

type apiItem struct {
	Number  int    `json:"number"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
	CommitID string `json:"commit_id"`
	Head     struct {
		SHA string `json:"sha"`
	} `json:"head"`
	PullRequest *struct{} `json:"pull_request"`
	SHA         string    `json:"sha"`
	Commit      struct {
		Message string `json:"message"`
	} `json:"commit"`
}

func (c *Client) list(ctx context.Context, endpoint string) ([]domain.Discussion, error) {
	var result []domain.Discussion
	for page := 1; page <= c.pages; page++ {
		u, err := url.Parse(c.baseURL + endpoint)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		response, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GitHub request %s: %w", endpoint, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes+1))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("GitHub request %s: status %d", endpoint, response.StatusCode)
		}
		if len(body) > maxBodyBytes {
			return nil, fmt.Errorf("GitHub response %s exceeds %d bytes", endpoint, maxBodyBytes)
		}
		var items []apiItem
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, fmt.Errorf("decode GitHub response %s: %w", endpoint, err)
		}
		for _, item := range items {
			if strings.Contains(endpoint, "/issues") && item.PullRequest != nil {
				continue
			}
			if item.HTMLURL == "" {
				return nil, fmt.Errorf("GitHub response %s contains an item without a URL", endpoint)
			}
			revision := item.CommitID
			if item.Head.SHA != "" {
				revision = item.Head.SHA
			}
			bodyText := item.Body
			if strings.Contains(endpoint, "/commits") {
				revision = item.SHA
				bodyText = item.Commit.Message
			}
			result = append(result, domain.Discussion{Kind: kind(endpoint), Number: item.Number, Revision: revision, URL: item.HTMLURL, Author: item.User.Login, Body: bodyText})
		}
		if len(items) < 100 {
			break
		}
	}
	return result, nil
}
func kind(endpoint string) string {
	if strings.Contains(endpoint, "/issues") {
		return "issue"
	}
	if strings.Contains(endpoint, "/commits") {
		return "commit"
	}
	return "pull_request"
}
func NewHTTPClient() *http.Client { return &http.Client{Timeout: 20 * time.Second} }
