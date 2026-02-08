package notifications

import (
	"fmt"
	"strconv"
	"strings"

	"gh-pr/internal/github"
	"gh-pr/internal/notificationsapi"
)

func MapNotification(n github.Notification) (notificationsapi.NotificationEvent, bool) {
	owner, repo, ok := splitFullName(n.Repository.FullName)
	if !ok {
		return notificationsapi.NotificationEvent{}, false
	}

	kind, number, ok := parseTarget(n.Subject.Type, n.Subject.URL)
	if !ok {
		return notificationsapi.NotificationEvent{}, false
	}

	return notificationsapi.NotificationEvent{
		Id:        n.ID,
		UpdatedAt: n.UpdatedAt,
		Repository: notificationsapi.RepositoryRef{
			Owner: owner,
			Repo:  repo,
		},
		Subject: notificationsapi.NotificationSubject{
			Type:  n.Subject.Type,
			Title: n.Subject.Title,
			Url:   n.Subject.URL,
		},
		Target: notificationsapi.NotificationTarget{
			Kind:   notificationsapi.NotificationTargetKind(kind),
			Number: number,
			Ref:    fmt.Sprintf("%s/%s#%d", owner, repo, number),
		},
	}, true
}

func splitFullName(fullName string) (string, string, bool) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseTarget(subjectType string, subjectURL string) (string, int, bool) {
	parts := strings.Split(strings.Trim(subjectURL, "/"), "/")
	if len(parts) < 5 {
		return "", 0, false
	}
	if parts[0] != "https:" && parts[0] != "http:" {
		return "", 0, false
	}
	if parts[2] != "api.github.com" {
		return "", 0, false
	}
	if parts[3] != "repos" {
		return "", 0, false
	}

	number, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || number <= 0 {
		return "", 0, false
	}

	switch subjectType {
	case "PullRequest":
		if parts[len(parts)-2] != "pulls" {
			return "", 0, false
		}
		return "pr", number, true
	case "Issue":
		if parts[len(parts)-2] != "issues" {
			return "", 0, false
		}
		return "issue", number, true
	default:
		return "", 0, false
	}
}
