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
const maxBodyBytes = 1 << 20

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
	copyClient := *client
	copyClient.Timeout = 20 * time.Second
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if pages < 1 {
		pages = 1
	}
	if pages > maxPages {
		pages = maxPages
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: &copyClient, pages: pages}
}

func (c *Client) Read(ctx context.Context, repository string) ([]domain.Discussion, error) {
	if !repositoryName.MatchString(repository) || strings.Contains(repository, "..") {
		return nil, fmt.Errorf("repository must be owner/name using safe characters")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
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
	if len(issues)+len(pulls) > 100 {
		return nil, fmt.Errorf("GitHub discussion limit exceeds 100 parent items; narrow the source repository or increase limits in a reviewed adapter")
	}
	parents := append(append([]domain.Discussion{}, issues...), pulls...)
	for _, parent := range parents {
		comments, err := c.list(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", repository, parent.Number))
		if err != nil {
			return nil, err
		}
		for i := range comments {
			comments[i].Kind = "comment"
			comments[i].ParentURL = parent.URL
			comments[i].Number = parent.Number
		}
		all = append(all, comments...)
	}
	for _, pull := range pulls {
		reviews, err := c.list(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", repository, pull.Number))
		if err != nil {
			return nil, err
		}
		for i := range reviews {
			reviews[i].Kind = "review"
			if reviews[i].Revision == "" {
				reviews[i].Revision = pull.Revision
			}
			reviews[i].ParentURL = pull.URL
			reviews[i].Number = pull.Number
		}
		all = append(all, reviews...)
		comments, err := c.list(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", repository, pull.Number))
		if err != nil {
			return nil, err
		}
		for i := range comments {
			comments[i].Kind = "review_comment"
			comments[i].ParentURL = pull.URL
			comments[i].Number = pull.Number
		}
		all = append(all, comments...)
	}
	commits, err := c.list(ctx, "/repos/"+repository+"/commits")
	if err != nil {
		return nil, err
	}
	all = append(all, commits...)
	return all, ctx.Err()
}

type apiItem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
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
			if response.StatusCode == 429 || (response.StatusCode == 403 && response.Header.Get("X-RateLimit-Remaining") == "0") {
				return nil, fmt.Errorf("GitHub rate limit reached; retry after the server's reset window")
			}
			return nil, fmt.Errorf("GitHub request %s: status %d", endpoint, response.StatusCode)
		}
		if len(body) > maxBodyBytes {
			return nil, fmt.Errorf("GitHub response %s exceeds %d bytes", endpoint, maxBodyBytes)
		}
		if c.token != "" && strings.Contains(string(body), c.token) {
			return nil, fmt.Errorf("GitHub response contains the active credential; refusing to expose it")
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
			source, err := url.Parse(item.HTMLURL)
			if err != nil || source.Scheme != "https" || source.Host == "" || source.User != nil {
				return nil, fmt.Errorf("GitHub source URL must be HTTPS without credentials")
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
			result = append(result, domain.Discussion{Kind: kind(endpoint), Number: item.Number, Revision: revision, URL: item.HTMLURL, Author: item.User.Login, Body: bodyText, Path: item.Path, Line: item.Line})
		}
		if len(items) < 100 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("GitHub pagination limit reached before exhaustion; results would be incomplete")
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
