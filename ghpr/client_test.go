package ghpr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestParseStatusCheckRollupFailedWins(t *testing.T) {
	status, ok := parseStatusCheckRollup([]byte(`{
		"statusCheckRollup": [
			{"status":"COMPLETED","conclusion":"SUCCESS"},
			{"status":"COMPLETED","conclusion":"FAILURE"}
		]
	}`))
	if !ok {
		t.Fatalf("expected parser to succeed")
	}
	if status != CIStatusFailed {
		t.Fatalf("expected failed status, got %q", status)
	}
}

func TestParseStatusCheckRollupPending(t *testing.T) {
	status, ok := parseStatusCheckRollup([]byte(`{
		"statusCheckRollup": [
			{"status":"IN_PROGRESS"}
		]
	}`))
	if !ok {
		t.Fatalf("expected parser to succeed")
	}
	if status != CIStatusPending {
		t.Fatalf("expected pending status, got %q", status)
	}
}

func TestParseStatusCheckRollupSuccess(t *testing.T) {
	status, ok := parseStatusCheckRollup([]byte(`{
		"statusCheckRollup": [
			{"status":"COMPLETED","conclusion":"SUCCESS"},
			{"state":"SUCCESS"}
		]
	}`))
	if !ok {
		t.Fatalf("expected parser to succeed")
	}
	if status != CIStatusSuccess {
		t.Fatalf("expected success status, got %q", status)
	}
}

func TestCIStatusForPRFailsOpenWhenGhFails(t *testing.T) {
	original := runGhPRViewStatusCheckRollup
	t.Cleanup(func() {
		runGhPRViewStatusCheckRollup = original
	})
	runGhPRViewStatusCheckRollup = func(ctx context.Context, repo string, number int) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}

	client := NewClientWithBaseURL("", "https://api.github.example")
	status := client.CIStatusForPR(context.Background(), "o/r#42")
	if status != CIStatusUnknown {
		t.Fatalf("expected unknown status on gh failure, got %q", status)
	}
}

func TestStreamTimelineCommittedEventEnrichesActorFromCommit(t *testing.T) {
	commitCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/issues/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":1,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","user":{"login":"octo","id":1},"pull_request":{"url":"https://api.github.com/repos/o/r/pulls/1"}}`))
		case "/repos/o/r/pulls/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":1,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","state":"open","user":{"login":"octo","id":1}}`))
		case "/repos/o/r/issues/1/timeline":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":101,"event":"committed","created_at":"2024-01-01T01:00:00Z","sha":"abc123","html_url":"https://github.com/o/r/commit/abc123"},
				{"id":102,"event":"committed","created_at":"2024-01-01T01:01:00Z","sha":"abc123","html_url":"https://github.com/o/r/commit/abc123"}
			]`))
		case "/repos/o/r/commits/abc123":
			commitCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"author":{"login":"alice","id":2}}`))
		case "/repos/o/r/pulls/1/comments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(fmt.Sprintf("unexpected path: %s", r.URL.Path)))
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL("", server.URL)
	var events []TimelineEvent
	err := client.StreamTimeline(context.Background(), "o/r#1", func(event TimelineEvent) error {
		events = append(events, event)
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if commitCalls != 1 {
		t.Fatalf("expected commit actor lookup to be cached, got %d calls", commitCalls)
	}

	commitEvents := 0
	for _, event := range events {
		if event.Type != "github.timeline.committed" {
			continue
		}
		commitEvents++
		if event.Actor == nil || event.Actor.Login != "alice" {
			t.Fatalf("expected committed actor alice, got %+v", event.Actor)
		}
	}
	if commitEvents != 2 {
		t.Fatalf("expected 2 committed events, got %d", commitEvents)
	}
}

func TestStreamTimelineCommitActorWarningOmitsHTMLErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/issues/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":1,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","user":{"login":"octo","id":1},"pull_request":{"url":"https://api.github.com/repos/o/r/pulls/1"}}`))
		case "/repos/o/r/pulls/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":1,"title":"x","body":"","created_at":"2024-01-01T00:00:00Z","state":"open","user":{"login":"octo","id":1}}`))
		case "/repos/o/r/issues/1/timeline":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"id":101,"event":"committed","created_at":"2024-01-01T01:00:00Z","sha":"abc123","html_url":"https://github.com/o/r/commit/abc123"}
			]`))
		case "/repos/o/r/commits/abc123":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>boom</body></html>"))
		case "/repos/o/r/pulls/1/comments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(fmt.Sprintf("unexpected path: %s", r.URL.Path)))
		}
	}))
	defer server.Close()

	client := NewClientWithBaseURL("", server.URL)
	warnings := make([]string, 0, 1)
	err := client.StreamTimeline(context.Background(), "o/r#1", func(event TimelineEvent) error {
		return nil
	}, func(w string) {
		warnings = append(warnings, w)
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if strings.Contains(strings.ToLower(warnings[0]), "doctype") || strings.Contains(strings.ToLower(warnings[0]), "<html") {
		t.Fatalf("expected compact warning without html payload, got %q", warnings[0])
	}
	if !strings.Contains(warnings[0], "status=502") || !strings.Contains(warnings[0], "Bad Gateway") {
		t.Fatalf("expected status and concise message in warning, got %q", warnings[0])
	}
	if strings.Contains(warnings[0], "\n") {
		t.Fatalf("expected warning to be single-line, got %q", warnings[0])
	}
}
