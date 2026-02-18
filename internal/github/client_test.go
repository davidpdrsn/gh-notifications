package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
