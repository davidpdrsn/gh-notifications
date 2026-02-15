package ghpr

import (
	"context"
	"strings"

	"gh-pr/internal/github"
	"gh-pr/internal/notifications"
	"gh-pr/internal/notificationsapi"
	"gh-pr/internal/schema"
	"gh-pr/internal/timeline"
	"gh-pr/internal/timelineapi"
)

type Client struct {
	github *github.Client
}

func NewClient(token string) *Client {
	return &Client{github: github.NewClient(token, "")}
}

func NewClientWithBaseURL(token string, baseURL string) *Client {
	return &Client{github: github.NewClient(token, baseURL)}
}

func NewClientFromEnv(ctx context.Context) (*Client, error) {
	resolver := github.NewTokenResolver()
	token, err := resolver.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return NewClient(token), nil
}

func (c *Client) StreamTimeline(ctx context.Context, ref string, onEvent func(TimelineEvent) error, onWarning func(string)) error {
	parsed, err := ParseTimelineRef(ref)
	if err != nil {
		return err
	}

	emit := func(e timelineapi.Event) error {
		if onEvent == nil {
			return nil
		}
		return onEvent(fromTimelineAPIEvent(e))
	}

	if parsed.KindHint == "pr" {
		pr, err := c.github.FetchPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
		if err != nil {
			return err
		}
		if err := emit(timeline.OpenedEvent(pr)); err != nil {
			return err
		}
		return c.streamPRTimeline(ctx, parsed.Owner, parsed.Repo, parsed.Number, emit, onWarning)
	}

	issue, err := c.github.FetchIssue(ctx, parsed.Owner, parsed.Repo, parsed.Number)
	if err != nil {
		return err
	}

	if issue.PullRequest != nil {
		pr, err := c.github.FetchPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
		if err != nil {
			return err
		}
		if err := emit(timeline.OpenedEvent(pr)); err != nil {
			return err
		}
		return c.streamPRTimeline(ctx, parsed.Owner, parsed.Repo, parsed.Number, emit, onWarning)
	}

	if err := emit(timeline.OpenedIssueEvent(issue)); err != nil {
		return err
	}

	return c.github.StreamTimeline(ctx, parsed.Owner, parsed.Repo, parsed.Number, func(item github.TimelineItem) error {
		e, warning, ok := timeline.MapTimelineItem(item.Raw)
		if warning != "" && onWarning != nil {
			onWarning(warning)
		}
		if ok {
			return emit(e)
		}
		return nil
	})
}

func (c *Client) StreamNotifications(ctx context.Context, onNotification func(NotificationEvent) error) error {
	return c.github.StreamNotifications(ctx, func(item github.Notification) error {
		event, ok := notifications.MapNotification(item)
		if !ok {
			return nil
		}
		if onNotification == nil {
			return nil
		}
		return onNotification(fromNotificationsAPIEvent(event))
	})
}

func TimelineSchemaJSON() ([]byte, error) {
	return schema.TimelineOpenAPIJSON()
}

func NotificationsSchemaJSON() ([]byte, error) {
	return schema.NotificationsOpenAPIJSON()
}

func (c *Client) FetchCommitDiff(ctx context.Context, diffURL string) (string, error) {
	return c.github.FetchCommitDiff(ctx, diffURL)
}

func (c *Client) FetchForcePushInterdiff(ctx context.Context, ref string, eventID string) (ForcePushInterdiff, error) {
	parsed, err := ParseTimelineRef(ref)
	if err != nil {
		return ForcePushInterdiff{}, err
	}
	interdiff, err := c.github.FetchForcePushInterdiff(ctx, parsed.Owner, parsed.Repo, parsed.Number, eventID)
	if err != nil {
		return ForcePushInterdiff{}, err
	}
	return ForcePushInterdiff{
		BeforeSHA:  interdiff.BeforeSHA,
		AfterSHA:   interdiff.AfterSHA,
		CompareURL: interdiff.CompareURL,
		Diff:       interdiff.Diff,
	}, nil
}

func (c *Client) ArchiveNotificationThread(ctx context.Context, threadID string) error {
	return c.github.ArchiveNotificationThread(ctx, threadID)
}

func (c *Client) FetchViewerLogin(ctx context.Context) (string, error) {
	viewer, err := c.github.FetchViewer(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(viewer.Login), nil
}

func (c *Client) ReviewRequestStatusForViewer(ctx context.Context, ref string, viewerLogin string) (ReviewRequestStatus, error) {
	viewerLogin = strings.TrimSpace(viewerLogin)
	if viewerLogin == "" {
		return ReviewRequestStatus{}, nil
	}
	parsed, err := ParseTimelineRef(ref)
	if err != nil {
		return ReviewRequestStatus{}, err
	}
	pr, err := c.github.FetchPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
	if err != nil {
		return ReviewRequestStatus{}, err
	}
	author := strings.TrimSpace(pr.User.Login)
	if pr.MergedAt != nil {
		return ReviewRequestStatus{Merged: true, Draft: pr.Draft, Author: author}, nil
	}
	if !strings.EqualFold(strings.TrimSpace(pr.State), "open") {
		return ReviewRequestStatus{Closed: true, Draft: pr.Draft, Author: author}, nil
	}
	requested, err := c.github.FetchRequestedReviewers(ctx, parsed.Owner, parsed.Repo, parsed.Number)
	if err != nil {
		return ReviewRequestStatus{}, err
	}
	for _, u := range requested.Users {
		if strings.EqualFold(strings.TrimSpace(u.Login), viewerLogin) {
			return ReviewRequestStatus{Pending: true, Draft: pr.Draft, Author: author}, nil
		}
	}
	return ReviewRequestStatus{Draft: pr.Draft, Author: author}, nil
}

func (c *Client) streamPRTimeline(ctx context.Context, owner, repo string, number int, emit func(timelineapi.Event) error, onWarning func(string)) error {
	if err := c.github.StreamTimeline(ctx, owner, repo, number, func(item github.TimelineItem) error {
		e, warning, ok := timeline.MapTimelineItem(item.Raw)
		if warning != "" && onWarning != nil {
			onWarning(warning)
		}
		if ok {
			if timeline.ShouldIgnorePRTimelineEvent(e) {
				return nil
			}
			return emit(e)
		}
		return nil
	}); err != nil {
		return err
	}

	return c.github.StreamReviewComments(ctx, owner, repo, number, func(comment github.ReviewComment) error {
		return emit(timeline.MapReviewComment(comment))
	})
}

func fromTimelineAPIEvent(e timelineapi.Event) TimelineEvent {
	out := TimelineEvent{
		Type:       e.Type,
		OccurredAt: e.OccurredAt,
		ID:         e.Id,
		Event:      e.Event,
		DiffURL:    e.DiffUrl,
	}
	if e.Actor != nil {
		out.Actor = &Actor{Login: e.Actor.Login, ID: e.Actor.Id}
	}
	if e.Pr != nil {
		out.Pr = &PROpenedData{Title: e.Pr.Title, Body: e.Pr.Body}
	}
	if e.Issue != nil {
		out.Issue = &IssueOpenedData{Title: e.Issue.Title, Body: e.Issue.Body}
	}
	if e.Comment != nil {
		out.Comment = &CommentContext{
			Path:             e.Comment.Path,
			Body:             e.Comment.Body,
			DiffHunk:         e.Comment.DiffHunk,
			Position:         e.Comment.Position,
			OriginalPosition: e.Comment.OriginalPosition,
			Line:             e.Comment.Line,
			StartLine:        e.Comment.StartLine,
			ReviewID:         e.Comment.ReviewId,
			ThreadID:         e.Comment.ThreadId,
			URL:              e.Comment.Url,
		}
	}
	if e.Commit != nil {
		out.Commit = &CommitContext{SHA: e.Commit.Sha, URL: e.Commit.Url}
	}
	return out
}

func fromNotificationsAPIEvent(e notificationsapi.NotificationEvent) NotificationEvent {
	return NotificationEvent{
		ID:        e.Id,
		UpdatedAt: e.UpdatedAt,
		Repository: NotificationRepository{
			Owner: e.Repository.Owner,
			Repo:  e.Repository.Repo,
		},
		Subject: NotificationSubject{
			Type:  e.Subject.Type,
			Title: e.Subject.Title,
			URL:   e.Subject.Url,
		},
		Target: NotificationTarget{
			Kind:   string(e.Target.Kind),
			Number: e.Target.Number,
			Ref:    e.Target.Ref,
		},
	}
}
