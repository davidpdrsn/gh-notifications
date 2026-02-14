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
