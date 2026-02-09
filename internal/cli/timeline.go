package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"gh-pr/internal/github"
	"gh-pr/internal/notifications"
	"gh-pr/internal/notificationsapi"
	"gh-pr/internal/schema"
	"gh-pr/internal/timeline"
	"gh-pr/internal/timelineapi"
	"gh-pr/internal/tui"

	"github.com/spf13/cobra"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	app := &app{
		stdout: stdout,
		stderr: stderr,
	}

	root := &cobra.Command{
		Use:           "gh-pr",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(app.timelineCommand(ctx))
	root.AddCommand(app.notificationsCommand(ctx))
	root.AddCommand(app.tuiCommand(ctx))
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		appErr := toAppError(err)
		writeErrorText(stderr, appErr)
		return appErr.ExitCode
	}

	return 0
}

type app struct {
	stdout io.Writer
	stderr io.Writer
}

func (a *app) timelineCommand(ctx context.Context) *cobra.Command {
	var printSchema bool

	cmd := &cobra.Command{
		Use:   "timeline <owner>/<repo>#<number>",
		Short: "Fetch and normalize an issue or PR timeline",
		Example: strings.Join([]string{
			"gh-pr timeline tokio-rs/axum#2398",
			"gh-pr timeline https://github.com/lun-energy/calor/pull/1556",
			"gh-pr timeline https://github.com/lun-energy/calor/issues/1556",
			"gh-pr timeline --schema",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if printSchema {
				if len(args) != 0 {
					return &AppError{Code: "parse_error", Message: "--schema cannot be used with issue or pull request argument", ExitCode: ExitParse, Details: nil, Cause: nil}
				}
				b, err := schema.TimelineOpenAPIJSON()
				if err != nil {
					return &AppError{Code: "internal_error", Message: "failed to render schema JSON", ExitCode: ExitInternal, Details: nil, Cause: err}
				}
				_, _ = a.stdout.Write(b)
				_, _ = a.stdout.Write([]byte("\n"))
				return nil
			}

			if len(args) == 0 {
				usage := cmd.UsageString()
				return &AppError{
					Code:     "parse_error",
					Message:  "missing issue or pull request reference\n\n" + strings.TrimSpace(usage),
					ExitCode: ExitParse,
					Details:  nil,
					Cause:    nil,
				}
			}

			if len(args) != 1 {
				usage := cmd.UsageString()
				return &AppError{Code: "parse_error", Message: "expected a single issue or pull request reference or GitHub URL\n\n" + strings.TrimSpace(usage), ExitCode: ExitParse, Details: nil, Cause: nil}
			}

			ref, err := parseTimelineRef(args[0])
			if err != nil {
				return &AppError{Code: "parse_error", Message: err.Error(), ExitCode: ExitParse, Details: nil, Cause: nil}
			}

			resolver := github.NewTokenResolver()
			token, err := resolver.Resolve(ctx)
			if err != nil {
				return &AppError{Code: "auth_error", Message: "failed to resolve GitHub auth token", ExitCode: ExitAuth, Details: nil, Cause: err}
			}

			client := github.NewClient(token, "")

			enc := json.NewEncoder(a.stdout)
			enc.SetEscapeHTML(false)

			writeLine := func(event timelineapi.Event) error {
				if err := enc.Encode(event); err != nil {
					return &AppError{Code: "internal_error", Message: "failed to encode JSON output", ExitCode: ExitInternal, Details: nil, Cause: err}
				}
				return nil
			}

			if ref.KindHint == "pr" {
				pr, err := client.FetchPullRequest(ctx, ref.Owner, ref.Repo, ref.Number)
				if err != nil {
					return mapGitHubErrorWithNotFound(err, "issue or pull request not found")
				}
				if err := writeLine(timeline.OpenedEvent(pr)); err != nil {
					return err
				}
				return a.streamTimelineWithReviewComments(ctx, client, ref.Owner, ref.Repo, ref.Number, writeLine)
			}

			issue, err := client.FetchIssue(ctx, ref.Owner, ref.Repo, ref.Number)
			if err != nil {
				return mapGitHubErrorWithNotFound(err, "issue or pull request not found")
			}

			if issue.PullRequest != nil {
				pr, err := client.FetchPullRequest(ctx, ref.Owner, ref.Repo, ref.Number)
				if err != nil {
					return mapGitHubErrorWithNotFound(err, "issue or pull request not found")
				}
				if err := writeLine(timeline.OpenedEvent(pr)); err != nil {
					return err
				}
				return a.streamTimelineWithReviewComments(ctx, client, ref.Owner, ref.Repo, ref.Number, writeLine)
			}

			if err := writeLine(timeline.OpenedIssueEvent(issue)); err != nil {
				return err
			}
			return a.streamTimelineOnly(ctx, client, ref.Owner, ref.Repo, ref.Number, writeLine)
		},
	}

	cmd.Flags().BoolVar(&printSchema, "schema", false, "print command schema as OpenAPI JSON")
	return cmd
}

func (a *app) streamTimelineOnly(ctx context.Context, client *github.Client, owner, repo string, number int, writeLine func(timelineapi.Event) error) error {
	err := client.StreamTimeline(ctx, owner, repo, number, func(item github.TimelineItem) error {
		event, warning, ok := timeline.MapTimelineItem(item.Raw)
		if warning != "" {
			_, _ = fmt.Fprintln(a.stderr, warning)
		}
		if ok {
			return writeLine(event)
		}
		return nil
	})
	if err != nil {
		return mapGitHubErrorWithNotFound(err, "issue or pull request not found")
	}
	return nil
}

func (a *app) streamTimelineWithReviewComments(ctx context.Context, client *github.Client, owner, repo string, number int, writeLine func(timelineapi.Event) error) error {
	eventsCh := make(chan timelineapi.Event, 128)
	warningsCh := make(chan string, 128)
	errCh := make(chan error, 2)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := client.StreamTimeline(streamCtx, owner, repo, number, func(item github.TimelineItem) error {
			event, warning, ok := timeline.MapTimelineItem(item.Raw)
			if warning != "" {
				warningsCh <- warning
			}
			if ok {
				if timeline.ShouldIgnorePRTimelineEvent(event) {
					return nil
				}
				eventsCh <- event
			}
			return nil
		})
		if err != nil {
			errCh <- mapGitHubErrorWithNotFound(err, "issue or pull request not found")
		}
	}()

	go func() {
		defer wg.Done()
		err := client.StreamReviewComments(streamCtx, owner, repo, number, func(comment github.ReviewComment) error {
			eventsCh <- timeline.MapReviewComment(comment)
			return nil
		})
		if err != nil {
			errCh <- mapGitHubErrorWithNotFound(err, "issue or pull request not found")
		}
	}()

	go func() {
		wg.Wait()
		close(eventsCh)
		close(warningsCh)
		close(errCh)
	}()

	var firstErr error
	for eventsCh != nil || warningsCh != nil || errCh != nil {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				continue
			}
			if firstErr == nil {
				if err := writeLine(event); err != nil {
					firstErr = err
					cancel()
				}
			}
		case warning, ok := <-warningsCh:
			if !ok {
				warningsCh = nil
				continue
			}
			if firstErr == nil {
				_, _ = fmt.Fprintln(a.stderr, warning)
			}
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil && firstErr == nil {
				firstErr = err
				cancel()
			}
		}
	}

	if firstErr != nil {
		return firstErr
	}

	return nil
}

func (a *app) notificationsCommand(ctx context.Context) *cobra.Command {
	var printSchema bool

	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "Fetch GitHub notifications for PRs and issues",
		Example: strings.Join([]string{
			"gh-pr notifications",
			"gh-pr notifications --schema",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if printSchema {
				if len(args) != 0 {
					return &AppError{Code: "parse_error", Message: "--schema cannot be used with positional arguments", ExitCode: ExitParse, Details: nil, Cause: nil}
				}
				b, err := schema.NotificationsOpenAPIJSON()
				if err != nil {
					return &AppError{Code: "internal_error", Message: "failed to render notifications schema JSON", ExitCode: ExitInternal, Details: nil, Cause: err}
				}
				_, _ = a.stdout.Write(b)
				_, _ = a.stdout.Write([]byte("\n"))
				return nil
			}

			if len(args) != 0 {
				usage := cmd.UsageString()
				return &AppError{Code: "parse_error", Message: "notifications does not accept positional arguments\n\n" + strings.TrimSpace(usage), ExitCode: ExitParse, Details: nil, Cause: nil}
			}

			resolver := github.NewTokenResolver()
			token, err := resolver.Resolve(ctx)
			if err != nil {
				return &AppError{Code: "auth_error", Message: "failed to resolve GitHub auth token", ExitCode: ExitAuth, Details: nil, Cause: err}
			}

			client := github.NewClient(token, "")
			enc := json.NewEncoder(a.stdout)
			enc.SetEscapeHTML(false)

			writeLine := func(event notificationsapi.NotificationEvent) error {
				if err := enc.Encode(event); err != nil {
					return &AppError{Code: "internal_error", Message: "failed to encode JSON output", ExitCode: ExitInternal, Details: nil, Cause: err}
				}
				return nil
			}

			err = client.StreamNotifications(ctx, func(item github.Notification) error {
				event, ok := notifications.MapNotification(item)
				if !ok {
					return nil
				}
				return writeLine(event)
			})
			if err != nil {
				return mapGitHubErrorWithNotFound(err, "notifications not found")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&printSchema, "schema", false, "print command schema as OpenAPI JSON")
	return cmd
}

func (a *app) tuiCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Browse notifications and timelines interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				usage := cmd.UsageString()
				return &AppError{Code: "parse_error", Message: "tui does not accept positional arguments\n\n" + strings.TrimSpace(usage), ExitCode: ExitParse, Details: nil, Cause: nil}
			}

			resolver := github.NewTokenResolver()
			token, err := resolver.Resolve(ctx)
			if err != nil {
				return &AppError{Code: "auth_error", Message: "failed to resolve GitHub auth token", ExitCode: ExitAuth, Details: nil, Cause: err}
			}

			if err := tui.Run(ctx, token, a.stdout); err != nil {
				return &AppError{Code: "internal_error", Message: "failed to run tui", ExitCode: ExitInternal, Details: nil, Cause: err}
			}
			return nil
		},
	}

	return cmd
}

func mapGitHubErrorWithNotFound(err error, notFoundMessage string) error {
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		details := map[string]any{
			"status_code": apiErr.StatusCode,
		}
		switch apiErr.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			if strings.Contains(strings.ToLower(apiErr.Message), "rate limit") {
				return &AppError{Code: "rate_limit", Message: "GitHub API rate limit reached", ExitCode: ExitRateLimit, Details: details, Cause: err}
			}
			return &AppError{Code: "auth_error", Message: "GitHub authentication failed", ExitCode: ExitAuth, Details: details, Cause: err}
		case http.StatusNotFound:
			return &AppError{Code: "not_found", Message: notFoundMessage, ExitCode: ExitNotFound, Details: details, Cause: err}
		case http.StatusTooManyRequests:
			return &AppError{Code: "rate_limit", Message: "GitHub API rate limit reached", ExitCode: ExitRateLimit, Details: details, Cause: err}
		default:
			return &AppError{Code: "transport_error", Message: "GitHub API request failed", ExitCode: ExitTransport, Details: details, Cause: err}
		}
	}

	return &AppError{Code: "transport_error", Message: "GitHub request failed", ExitCode: ExitTransport, Details: nil, Cause: err}
}

func toAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return &AppError{Code: "internal_error", Message: "internal error", ExitCode: ExitInternal, Details: nil, Cause: err}
}
