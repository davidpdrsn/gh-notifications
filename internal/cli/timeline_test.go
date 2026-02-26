package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"gh-pr/internal/github"
)

func TestRunTimelineWithoutArgsShowsHelpTextOnStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"timeline"}, &stdout, &stderr)
	if exitCode != ExitParse {
		t.Fatalf("expected exit code %d, got %d", ExitParse, exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	errText := stderr.String()
	if !strings.Contains(errText, "missing issue or pull request reference") {
		t.Fatalf("expected missing argument message, got %q", errText)
	}
	if !strings.Contains(errText, "gh-pr timeline <owner>/<repo>#<number>") {
		t.Fatalf("expected usage help in stderr, got %q", errText)
	}
}

func TestRunNotificationsWithArgsShowsHelpTextOnStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"notifications", "extra"}, &stdout, &stderr)
	if exitCode != ExitParse {
		t.Fatalf("expected exit code %d, got %d", ExitParse, exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	errText := stderr.String()
	if !strings.Contains(errText, "notifications does not accept positional arguments") {
		t.Fatalf("expected positional argument error, got %q", errText)
	}
	if !strings.Contains(errText, "gh-pr notifications") {
		t.Fatalf("expected usage help in stderr, got %q", errText)
	}
}

func TestMapGitHubErrorWithNotFoundIncludesRateLimitDetails(t *testing.T) {
	retryAfter := 3
	remaining := 0
	reset := time.Unix(1710000000, 0).UTC()

	appErr := mapGitHubErrorWithNotFound(&github.APIError{
		StatusCode:         http.StatusForbidden,
		Message:            "API rate limit exceeded",
		RetryAfterSeconds:  &retryAfter,
		RateLimitRemaining: &remaining,
		RateLimitReset:     &reset,
	}, "not found")

	if appErr == nil {
		t.Fatalf("expected app error")
	}
	var typed *AppError
	if _, ok := appErr.(*AppError); !ok {
		t.Fatalf("expected *AppError, got %T", appErr)
	}
	typed = appErr.(*AppError)
	if typed.Code != "rate_limit" {
		t.Fatalf("expected rate_limit code, got %q", typed.Code)
	}
	if typed.Details["retry_after_seconds"] != retryAfter {
		t.Fatalf("expected retry_after_seconds=%d, got %v", retryAfter, typed.Details["retry_after_seconds"])
	}
	if typed.Details["rate_limit_remaining"] != remaining {
		t.Fatalf("expected rate_limit_remaining=%d, got %v", remaining, typed.Details["rate_limit_remaining"])
	}
	if typed.Details["rate_limit_reset_at"] != reset.Format(time.RFC3339) {
		t.Fatalf("unexpected rate_limit_reset_at: %v", typed.Details["rate_limit_reset_at"])
	}
}
