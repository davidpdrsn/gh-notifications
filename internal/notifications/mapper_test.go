package notifications

import (
	"testing"

	"gh-pr/internal/github"
)

func TestMapNotificationPullRequest(t *testing.T) {
	n := github.Notification{
		ID: "1",
		Repository: github.NotificationRepository{
			FullName: "owner/repo",
		},
		Subject: github.NotificationSubject{
			Type:  "PullRequest",
			Title: "Test PR",
			URL:   "https://api.github.com/repos/owner/repo/pulls/42",
		},
	}

	e, ok := MapNotification(n)
	if !ok {
		t.Fatalf("expected notification to be mapped")
	}
	if e.Target.Kind != "pr" || e.Target.Number != 42 || e.Target.Ref != "owner/repo#42" {
		t.Fatalf("unexpected target: %+v", e.Target)
	}
}

func TestMapNotificationIssue(t *testing.T) {
	n := github.Notification{
		ID: "2",
		Repository: github.NotificationRepository{
			FullName: "owner/repo",
		},
		Subject: github.NotificationSubject{
			Type:  "Issue",
			Title: "Test issue",
			URL:   "https://api.github.com/repos/owner/repo/issues/7",
		},
	}

	e, ok := MapNotification(n)
	if !ok {
		t.Fatalf("expected notification to be mapped")
	}
	if e.Target.Kind != "issue" || e.Target.Number != 7 || e.Target.Ref != "owner/repo#7" {
		t.Fatalf("unexpected target: %+v", e.Target)
	}
}

func TestMapNotificationSkipsUnsupportedTypes(t *testing.T) {
	n := github.Notification{
		ID: "3",
		Repository: github.NotificationRepository{
			FullName: "owner/repo",
		},
		Subject: github.NotificationSubject{
			Type:  "Discussion",
			Title: "Test discussion",
			URL:   "https://api.github.com/repos/owner/repo/discussions/9",
		},
	}

	_, ok := MapNotification(n)
	if ok {
		t.Fatalf("expected unsupported notification to be skipped")
	}
}
