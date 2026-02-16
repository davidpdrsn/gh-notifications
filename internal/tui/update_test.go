package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateKeyMsgDoesNotSpawnAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""), nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for key message")
	}
}

func TestUpdateAsyncMsgRearmsAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""), nil)

	_, cmd := m.Update(notifDoneMsg{gen: m.state.NotifGen})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for async message")
	}
}

func TestShouldSkipArchivedNotification(t *testing.T) {
	archivedAt := time.Now().UTC().Add(-time.Minute)
	archived := map[string]time.Time{"42": archivedAt}

	if !shouldSkipArchivedNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt}, archived) {
		t.Fatalf("expected archived notification with same updated_at to be skipped")
	}
	if shouldSkipArchivedNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt.Add(time.Second)}, archived) {
		t.Fatalf("expected newer notification not to be skipped")
	}
}

func TestShouldUnarchiveNotification(t *testing.T) {
	archivedAt := time.Now().UTC().Add(-time.Minute)
	archived := map[string]time.Time{"42": archivedAt}

	if shouldUnarchiveNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt}, archived) {
		t.Fatalf("expected same timestamp not to unarchive")
	}
	if !shouldUnarchiveNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt.Add(time.Second)}, archived) {
		t.Fatalf("expected newer timestamp to unarchive")
	}
}

func TestShouldSkipIgnoredNotificationCachesLookup(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notifications/threads/42/subscription" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subscribed":false,"ignored":true}`))
	}))
	defer server.Close()

	client := ghpr.NewClientWithBaseURL("", server.URL)
	cache := map[string]bool{}

	skip, err := shouldSkipIgnoredNotification(context.Background(), client, "42", cache)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !skip {
		t.Fatalf("expected notification to be skipped")
	}

	skip, err = shouldSkipIgnoredNotification(context.Background(), client, "42", cache)
	if err != nil {
		t.Fatalf("expected no error on cache hit, got %v", err)
	}
	if !skip {
		t.Fatalf("expected cached notification to be skipped")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected one subscription lookup, got %d", got)
	}
}

func TestShouldSkipIgnoredNotificationFailOpenOnLookupError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer server.Close()

	client := ghpr.NewClientWithBaseURL("", server.URL)
	skip, err := shouldSkipIgnoredNotification(context.Background(), client, "42", map[string]bool{})
	if err == nil {
		t.Fatalf("expected lookup error")
	}
	if skip {
		t.Fatalf("expected fail-open behavior, got skip=true")
	}
}

func TestShouldSkipIgnoredNotificationShowsWhenThreadNoLongerIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notifications/threads/42/subscription" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subscribed":true,"ignored":false}`))
	}))
	defer server.Close()

	client := ghpr.NewClientWithBaseURL("", server.URL)
	skip, err := shouldSkipIgnoredNotification(context.Background(), client, "42", map[string]bool{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if skip {
		t.Fatalf("expected non-ignored thread to be shown")
	}
}

func TestShouldSkipIgnoredNotificationShowsAfterMentionOrReviewRequestUnmutesThread(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notifications/threads/42/subscription" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		call := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		if call == 1 {
			_, _ = w.Write([]byte(`{"subscribed":false,"ignored":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"subscribed":true,"ignored":false}`))
	}))
	defer server.Close()

	client := ghpr.NewClientWithBaseURL("", server.URL)

	firstRefreshCache := map[string]bool{}
	skip, err := shouldSkipIgnoredNotification(context.Background(), client, "42", firstRefreshCache)
	if err != nil {
		t.Fatalf("expected no error on first refresh, got %v", err)
	}
	if !skip {
		t.Fatalf("expected ignored thread to be hidden on first refresh")
	}

	secondRefreshCache := map[string]bool{}
	skip, err = shouldSkipIgnoredNotification(context.Background(), client, "42", secondRefreshCache)
	if err != nil {
		t.Fatalf("expected no error on second refresh, got %v", err)
	}
	if skip {
		t.Fatalf("expected unmuted thread to be shown on later refresh")
	}
}
