package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestStreamNotificationsRequestsAllNotifications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notifications" {
			t.Fatalf("expected /notifications path, got %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("all"); got != "true" {
			t.Fatalf("expected all=true query param, got %q", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Fatalf("expected per_page=100 query param, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	err := client.StreamNotifications(context.Background(), func(item Notification) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestArchiveNotificationThreadUsesDeleteEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %q", r.Method)
		}
		if r.URL.Path != "/notifications/threads/42" {
			t.Fatalf("expected thread path, got %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	if err := client.ArchiveNotificationThread(context.Background(), "42"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUnsubscribeNotificationThreadUsesDeleteSubscriptionEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE method, got %q", r.Method)
		}
		if r.URL.Path != "/notifications/threads/42/subscription" {
			t.Fatalf("expected thread subscription path, got %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	if err := client.UnsubscribeNotificationThread(context.Background(), "42"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestFetchNotificationThreadSubscriptionUsesGetEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %q", r.Method)
		}
		if r.URL.Path != "/notifications/threads/42/subscription" {
			t.Fatalf("expected thread subscription path, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subscribed":false,"ignored":true}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	out, err := client.FetchNotificationThreadSubscription(context.Background(), "42")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.Subscribed {
		t.Fatalf("expected subscribed=false")
	}
	if !out.Ignored {
		t.Fatalf("expected ignored=true")
	}
}

func TestFetchNotificationThreadSubscriptionRejectsEmptyThreadID(t *testing.T) {
	client := NewClient("", "https://api.github.com")
	if _, err := client.FetchNotificationThreadSubscription(context.Background(), " "); err == nil {
		t.Fatalf("expected error for empty thread id")
	}
}

func TestFetchViewerUsesUserEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %q", r.Method)
		}
		if r.URL.Path != "/user" {
			t.Fatalf("expected /user path, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"alice","id":1}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	viewer, err := client.FetchViewer(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if viewer.Login != "alice" {
		t.Fatalf("expected login alice, got %q", viewer.Login)
	}
}

func TestFetchRequestedReviewersUsesPREndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %q", r.Method)
		}
		if r.URL.Path != "/repos/o/r/pulls/7/requested_reviewers" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"login":"alice","id":1}]}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	out, err := client.FetchRequestedReviewers(context.Background(), "o", "r", 7)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.Users) != 1 || out.Users[0].Login != "alice" {
		t.Fatalf("unexpected reviewers payload: %+v", out.Users)
	}
}

func TestFetchCommitUserPrefersAuthor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %q", r.Method)
		}
		if r.URL.Path != "/repos/o/r/commits/abc123" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"author":{"login":"alice","id":1},"committer":{"login":"bot","id":2}}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	user, err := client.FetchCommitUser(context.Background(), "o", "r", "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user == nil {
		t.Fatalf("expected user to be resolved")
	}
	if user.Login != "alice" {
		t.Fatalf("expected author login alice, got %q", user.Login)
	}
}

func TestFetchCommitUserFallsBackToCommitter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method, got %q", r.Method)
		}
		if r.URL.Path != "/repos/o/r/commits/abc123" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"author":null,"committer":{"login":"alice","id":1}}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	user, err := client.FetchCommitUser(context.Background(), "o", "r", "abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user == nil {
		t.Fatalf("expected fallback committer user")
	}
	if user.Login != "alice" {
		t.Fatalf("expected committer login alice, got %q", user.Login)
	}
}

func TestFetchViewerRetriesOnRateLimitRetryAfter(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"alice","id":1}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	_, err := client.FetchViewer(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestFetchViewerRetriesOnRateLimitResetHeader(t *testing.T) {
	attempts := 0
	reset := time.Now().Add(50 * time.Millisecond).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"alice","id":1}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	_, err := client.FetchViewer(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestFetchViewerDoesNotRetryForbiddenWithoutRateLimitSignal(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	client := NewClient("", server.URL)
	_, err := client.FetchViewer(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestFetchViewerRateLimitWaitRespectsContextCancel(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	client := NewClient("", server.URL)
	_, err := client.FetchViewer(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestFetchViewerRateLimitResetWaitIsNotCapped(t *testing.T) {
	attempts := 0
	reset := time.Now().Add(2 * time.Minute).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	client := NewClient("", server.URL)
	var waitSeen time.Duration
	client.SetRateLimitRetryHook(func(_ APIError, wait time.Duration, _ int) {
		waitSeen = wait
	})

	_, err := client.FetchViewer(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if waitSeen < 90*time.Second {
		t.Fatalf("expected reset-based wait to be uncapped, got %s", waitSeen)
	}
}

func TestParseRetryAfterHeaderSupportsHTTPDate(t *testing.T) {
	now := time.Now().UTC()
	raw := now.Add(2 * time.Second).Format(http.TimeFormat)

	wait, seconds, ok := parseRetryAfterHeader(raw, now)
	if !ok {
		t.Fatalf("expected header to parse")
	}
	if seconds != nil {
		t.Fatalf("expected nil seconds for HTTP-date retry-after, got %v", *seconds)
	}
	if wait <= 0 {
		t.Fatalf("expected positive wait, got %s", wait)
	}
}
