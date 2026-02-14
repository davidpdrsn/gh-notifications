package ghpr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviewRequestStatusForViewerClosedPR(t *testing.T) {
	requestedCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":1,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","state":"closed","user":{"login":"octo","id":1}}`))
		case "/repos/o/r/pulls/1/requested_reviewers":
			requestedCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"users":[{"login":"alice","id":2}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL("", server.URL)
	status, err := client.ReviewRequestStatusForViewer(context.Background(), "o/r#1", "alice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Pending || status.Merged || !status.Closed {
		t.Fatalf("expected closed status only, got %+v", status)
	}
	if requestedCalled {
		t.Fatalf("expected requested_reviewers endpoint not to be called for closed pr")
	}
}

func TestReviewRequestStatusForViewerOpenPendingPR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":2,"title":"x","body":"","created_at":"` + time.Now().UTC().Format(time.RFC3339) + `","state":"open","user":{"login":"octo","id":1}}`))
		case "/repos/o/r/pulls/2/requested_reviewers":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"users":[{"login":"alice","id":2}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL("", server.URL)
	status, err := client.ReviewRequestStatusForViewer(context.Background(), "o/r#2", "alice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !status.Pending || status.Merged || status.Closed {
		t.Fatalf("expected pending open status, got %+v", status)
	}
}

func TestReviewRequestStatusForViewerMergedPR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/pulls/3":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":3,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","state":"closed","merged_at":"2024-01-02T00:00:00Z","user":{"login":"octo","id":1}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL("", server.URL)
	status, err := client.ReviewRequestStatusForViewer(context.Background(), "o/r#3", "alice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Pending || !status.Merged || status.Closed {
		t.Fatalf("expected merged status only, got %+v", status)
	}
}
