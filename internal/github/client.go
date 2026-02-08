package github

import (
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
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `json:"user"`
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
	Repository NotificationRepository `json:"repository"`
	Subject    NotificationSubject    `json:"subject"`
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
	path := "/notifications?per_page=100"

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
