package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

type User struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Type  string `json:"type,omitempty"`
}

type PullRequest struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	State     string     `json:"state"`
	MergedAt  *time.Time `json:"merged_at"`
	Draft     bool       `json:"draft"`
	User      User       `json:"user"`
}

type IssuePullRequest struct {
	URL string `json:"url"`
}

type Issue struct {
	Number      int               `json:"number"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	CreatedAt   time.Time         `json:"created_at"`
	User        User              `json:"user"`
	PullRequest *IssuePullRequest `json:"pull_request,omitempty"`
}

type TimelineItem struct {
	Raw json.RawMessage
}

type NotificationRepository struct {
	FullName string `json:"full_name"`
}

type NotificationSubject struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Notification struct {
	ID         string                 `json:"id"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Repository NotificationRepository `json:"repository"`
	Subject    NotificationSubject    `json:"subject"`
}

type RequestedReviewers struct {
	Users []User `json:"users"`
}

type ReviewComment struct {
	ID                int64      `json:"id"`
	NodeID            string     `json:"node_id"`
	InReplyToID       *int64     `json:"in_reply_to_id"`
	Path              string     `json:"path"`
	Body              string     `json:"body"`
	DiffHunk          string     `json:"diff_hunk"`
	HTMLURL           string     `json:"html_url"`
	PullRequestReview int64      `json:"pull_request_review_id"`
	Position          *int       `json:"position"`
	OriginalPosition  *int       `json:"original_position"`
	Line              *int       `json:"line"`
	StartLine         *int       `json:"start_line"`
	CreatedAt         *time.Time `json:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at"`
	CommitID          string     `json:"commit_id"`
	OriginalCommitID  string     `json:"original_commit_id"`
	User              User       `json:"user"`
}

type ForcePushInterdiff struct {
	BeforeSHA  string
	AfterSHA   string
	CompareURL string
	Diff       string
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(token string, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}

	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      strings.TrimSpace(token),
	}
}

func (c *Client) FetchPullRequest(ctx context.Context, owner, repo string, number int) (PullRequest, error) {
	var pr PullRequest
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	if err := c.getJSON(ctx, path, &pr); err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

func (c *Client) FetchIssue(ctx context.Context, owner, repo string, number int) (Issue, error) {
	var issue Issue
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	if err := c.getJSON(ctx, path, &issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (c *Client) FetchTimeline(ctx context.Context, owner, repo string, number int) ([]TimelineItem, error) {
	items := make([]TimelineItem, 0, 128)
	err := c.StreamTimeline(ctx, owner, repo, number, func(item TimelineItem) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) StreamTimeline(ctx context.Context, owner, repo string, number int, onItem func(TimelineItem) error) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/timeline?per_page=100", url.PathEscape(owner), url.PathEscape(repo), number)

	for path != "" {
		req, err := c.newRequest(ctx, path)
		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("github request failed: %w", err)
		}

		if err := checkStatus(resp); err != nil {
			_ = resp.Body.Close()
			return err
		}

		var page []json.RawMessage
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read github response body: %w", err)
		}

		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("failed to decode github timeline response: %w", err)
		}

		for _, raw := range page {
			if err := onItem(TimelineItem{Raw: raw}); err != nil {
				return err
			}
		}

		path = nextLinkPath(resp.Header.Get("Link"))
	}

	return nil
}

func (c *Client) FetchReviewComments(ctx context.Context, owner, repo string, number int) ([]ReviewComment, error) {
	items := make([]ReviewComment, 0, 64)
	err := c.StreamReviewComments(ctx, owner, repo, number, func(item ReviewComment) error {
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (c *Client) StreamReviewComments(ctx context.Context, owner, repo string, number int, onItem func(ReviewComment) error) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?per_page=100", url.PathEscape(owner), url.PathEscape(repo), number)

	for path != "" {
		req, err := c.newRequest(ctx, path)
		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("github request failed: %w", err)
		}

		if err := checkStatus(resp); err != nil {
			_ = resp.Body.Close()
			return err
		}

		var page []ReviewComment
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read github response body: %w", err)
		}

		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("failed to decode github review comments response: %w", err)
		}

		for _, item := range page {
			if err := onItem(item); err != nil {
				return err
			}
		}
		path = nextLinkPath(resp.Header.Get("Link"))
	}

	return nil
}

func (c *Client) StreamNotifications(ctx context.Context, onItem func(Notification) error) error {
	path := "/notifications?all=true&per_page=100"

	for path != "" {
		req, err := c.newRequest(ctx, path)
		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("github request failed: %w", err)
		}

		if err := checkStatus(resp); err != nil {
			_ = resp.Body.Close()
			return err
		}

		var page []Notification
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read github response body: %w", err)
		}

		if err := json.Unmarshal(body, &page); err != nil {
			return fmt.Errorf("failed to decode github notifications response: %w", err)
		}

		for _, item := range page {
			if err := onItem(item); err != nil {
				return err
			}
		}

		path = nextLinkPath(resp.Header.Get("Link"))
	}

	return nil
}

func (c *Client) ArchiveNotificationThread(ctx context.Context, threadID string) error {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return fmt.Errorf("thread id is empty")
	}

	path := fmt.Sprintf("/notifications/threads/%s", url.PathEscape(threadID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if err := checkStatus(resp); err != nil {
		return err
	}

	return nil
}

func (c *Client) UnsubscribeNotificationThread(ctx context.Context, threadID string) error {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return fmt.Errorf("thread id is empty")
	}

	path := fmt.Sprintf("/notifications/threads/%s/subscription", url.PathEscape(threadID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if err := checkStatus(resp); err != nil {
		return err
	}

	return nil
}

func (c *Client) FetchViewer(ctx context.Context) (User, error) {
	var user User
	if err := c.getJSON(ctx, "/user", &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Client) FetchRequestedReviewers(ctx context.Context, owner, repo string, number int) (RequestedReviewers, error) {
	var out RequestedReviewers
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", url.PathEscape(owner), url.PathEscape(repo), number)
	if err := c.getJSON(ctx, path, &out); err != nil {
		return RequestedReviewers{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, path)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if err := checkStatus(resp); err != nil {
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode github response: %w", err)
	}

	return nil
}

func (c *Client) getText(ctx context.Context, path string, accept string) (string, error) {
	req, err := c.newRequest(ctx, path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if err := checkStatus(resp); err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read github response body: %w", err)
	}

	return string(body), nil
}

func (c *Client) FetchCommitDiff(ctx context.Context, diffURL string) (string, error) {
	diff, err := c.getText(ctx, diffURL, "application/vnd.github.v3.diff")
	if err == nil {
		return diff, nil
	}

	alt := toAPICommitURL(diffURL)
	if alt == "" || alt == diffURL {
		return "", err
	}

	return c.getText(ctx, alt, "application/vnd.github.v3.diff")
}

func (c *Client) FetchForcePushInterdiff(ctx context.Context, owner, repo string, number int, eventID string) (ForcePushInterdiff, error) {
	before, after, err := c.fetchForcePushBeforeAfter(ctx, owner, repo, number, eventID)
	if err != nil {
		return ForcePushInterdiff{}, err
	}
	if before == "" || after == "" {
		return ForcePushInterdiff{}, fmt.Errorf("force-push event %s missing before/after commit", eventID)
	}

	comparePath := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(before), url.PathEscape(after))
	diff, err := c.getText(ctx, comparePath, "application/vnd.github.v3.diff")
	if err != nil {
		return ForcePushInterdiff{}, err
	}

	return ForcePushInterdiff{
		BeforeSHA:  before,
		AfterSHA:   after,
		CompareURL: fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s", owner, repo, before, after),
		Diff:       diff,
	}, nil
}

func (c *Client) fetchForcePushBeforeAfter(ctx context.Context, owner, repo string, number int, eventID string) (string, string, error) {
	query := `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      timelineItems(first: 100, after: $after, itemTypes: [HEAD_REF_FORCE_PUSHED_EVENT]) {
        pageInfo { hasNextPage endCursor }
        nodes {
          ... on HeadRefForcePushedEvent {
            id
            beforeCommit { oid }
            afterCommit { oid }
          }
        }
      }
    }
  }
}`

	type gqlNode struct {
		ID           string `json:"id"`
		BeforeCommit *struct {
			OID string `json:"oid"`
		} `json:"beforeCommit"`
		AfterCommit *struct {
			OID string `json:"oid"`
		} `json:"afterCommit"`
	}
	var resp struct {
		Data struct {
			Repository *struct {
				PullRequest *struct {
					TimelineItems struct {
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
						Nodes []gqlNode `json:"nodes"`
					} `json:"timelineItems"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	afterCursor := ""
	for {
		variables := map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  nil,
		}
		if afterCursor != "" {
			variables["after"] = afterCursor
		}
		if err := c.graphQL(ctx, query, variables, &resp); err != nil {
			return "", "", err
		}
		if len(resp.Errors) > 0 {
			return "", "", fmt.Errorf("github graphql error: %s", resp.Errors[0].Message)
		}
		if resp.Data.Repository == nil || resp.Data.Repository.PullRequest == nil {
			return "", "", fmt.Errorf("pull request not found")
		}

		for _, node := range resp.Data.Repository.PullRequest.TimelineItems.Nodes {
			if strings.TrimSpace(eventID) != strings.TrimSpace(node.ID) {
				continue
			}
			before := ""
			after := ""
			if node.BeforeCommit != nil {
				before = strings.TrimSpace(node.BeforeCommit.OID)
			}
			if node.AfterCommit != nil {
				after = strings.TrimSpace(node.AfterCommit.OID)
			}
			return before, after, nil
		}

		page := resp.Data.Repository.PullRequest.TimelineItems.PageInfo
		if !page.HasNextPage || page.EndCursor == "" {
			break
		}
		afterCursor = page.EndCursor
	}

	return "", "", fmt.Errorf("force-push event %s not found in pull request timeline", eventID)
}

func (c *Client) graphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("failed to encode graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlEndpointURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create github graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if err := checkStatus(resp); err != nil {
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode github graphql response: %w", err)
	}
	return nil
}

func (c *Client) graphqlEndpointURL() string {
	base := strings.TrimRight(c.baseURL, "/")
	if strings.HasSuffix(base, "/api/v3") {
		return strings.TrimSuffix(base, "/api/v3") + "/api/graphql"
	}
	if strings.HasSuffix(base, "/api") {
		return base + "/graphql"
	}
	return base + "/graphql"
}

func toAPICommitURL(rawURL string) string {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return ""
	}
	if strings.Contains(u, "api.github.com/repos/") && strings.Contains(u, "/commits/") {
		return u
	}
	if !(strings.HasPrefix(u, "https://github.com/") || strings.HasPrefix(u, "http://github.com/")) {
		return ""
	}

	trimmed := strings.TrimPrefix(u, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	trimmed = strings.TrimSuffix(trimmed, ".diff")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 4 || parts[2] != "commit" {
		return ""
	}
	owner, repo, sha := parts[0], parts[1], parts[3]
	if owner == "" || repo == "" || sha == "" {
		return ""
	}

	return "https://api.github.com/repos/" + owner + "/" + repo + "/commits/" + sha
}

func (c *Client) newRequest(ctx context.Context, path string) (*http.Request, error) {
	var fullURL string
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		fullURL = path
	} else {
		fullURL = c.baseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create github request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return req, nil
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return "github api error: status=" + strconv.Itoa(e.StatusCode) + " message=" + e.Message
}

func nextLinkPath(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	segments := strings.Split(linkHeader, ",")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if !strings.Contains(seg, `rel="next"`) {
			continue
		}

		start := strings.Index(seg, "<")
		end := strings.Index(seg, ">")
		if start == -1 || end == -1 || end <= start+1 {
			return ""
		}

		return seg[start+1 : end]
	}

	return ""
}
