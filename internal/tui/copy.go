package tui

import (
	"fmt"
	"gh-pr/ghpr"
	"strings"
	"time"
)

func columnCopyText(state AppState) (string, string) {
	switch state.Focus {
	case focusNotifications:
		return "notifications", notificationsColumnText(state)
	case focusTimeline:
		return "timeline", timelineColumnText(state)
	case focusThread:
		return "thread", threadColumnText(state)
	case focusDetail:
		return "detail", detailColumnText(state)
	default:
		return "timeline", timelineColumnText(state)
	}
}

func threadColumnText(state AppState) string {
	ts := state.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		return ""
	}
	rows := ts.threadRows(ts.activeThreadID, state.HideRead)
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, row.label)
	}
	return strings.Join(lines, "\n")
}

func notificationsColumnText(state AppState) string {
	visible := state.visibleNotifications()
	if len(visible) == 0 {
		return ""
	}

	lines := make([]string, 0, len(visible))
	repoColWidth := notificationRepoColumnWidth(visible)
	for _, n := range visible {
		repo := padToDisplayWidth(oneLine(n.repo), repoColWidth)
		line := strings.TrimSpace(stringsJoin([]string{repo, oneLine(n.title)}, "  "))
		if line == "" {
			line = oneLine(n.ref)
		}
		if line == "" {
			line = n.id
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func timelineColumnText(state AppState) string {
	ts := state.currentTimeline()
	if ts == nil {
		return ""
	}
	rows := ts.displayRows(state.HideRead)
	if len(rows) == 0 {
		return ""
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.event != nil {
			lines = append(lines, row.label)
			continue
		}
		if row.isThreadHeader {
			lines = append(lines, row.label)
			continue
		}
		lines = append(lines, row.label)
	}

	return strings.Join(lines, "\n")
}

func detailColumnText(state AppState) string {
	lines := detailLines(state)
	if len(lines) == 0 {
		return ""
	}
	for i := range lines {
		lines[i] = sanitizeForRender(lines[i])
	}
	return strings.Join(lines, "\n")
}

func detailLines(state AppState) []string {
	if state.Focus == focusNotifications {
		n := state.selectedNotification()
		if n == nil {
			return nil
		}
		return []string{
			fmt.Sprintf("%s %s", n.kind, n.ref),
			n.repo,
			n.title,
		}
	}

	ev := state.selectedDetailEvent()
	if ev == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("type: %s", ev.Type),
		fmt.Sprintf("id: %s", ev.ID),
		fmt.Sprintf("at: %s", ev.OccurredAt.Format(time.RFC3339)),
	}
	if selected, ok := selectedDetailRead(state); ok {
		if selected {
			lines = append(lines, "status: read")
		} else {
			lines = append(lines, "status: unread")
		}
	}
	if ev.Event != nil && oneLine(*ev.Event) != "" {
		lines = append(lines, fmt.Sprintf("event: %s", oneLine(*ev.Event)))
	}
	if ev.Actor != nil && oneLine(ev.Actor.Login) != "" {
		lines = append(lines, fmt.Sprintf("actor: %s", oneLine(ev.Actor.Login)))
	}
	if summary := detailSummaryText(*ev); summary != "" {
		lines = append(lines, fmt.Sprintf("summary: %s", summary))
	}
	if ev.Comment != nil {
		if selectedDetailIsThreadRoot(state) && ev.Comment.DiffHunk != nil && strings.TrimSpace(*ev.Comment.DiffHunk) != "" {
			lines = append(lines, "", "diff:")
			lines = append(lines, strings.Split(strings.ReplaceAll(*ev.Comment.DiffHunk, "\r\n", "\n"), "\n")...)
		}
		if ev.Comment.Body != nil {
			lines = append(lines, "", *ev.Comment.Body)
		}
		if ev.Comment.URL != nil {
			lines = append(lines, "", "link: "+*ev.Comment.URL)
		}
	}
	if ev.Type == "github.timeline.committed" && ev.Commit != nil {
		if ev.Commit.SHA != nil && *ev.Commit.SHA != "" {
			lines = append(lines, "", "sha: "+*ev.Commit.SHA)
		}
		if ev.Commit.URL != nil && *ev.Commit.URL != "" {
			lines = append(lines, "url: "+*ev.Commit.URL)
		}
		if ev.DiffURL != nil && *ev.DiffURL != "" {
			lines = append(lines, "diff: "+*ev.DiffURL)
		}
		lines = append(lines, "")
		lines = append(lines, commitDiffDetailLines(state, ev.ID)...)
	}
	if ev.Type == "github.timeline.head_ref_force_pushed" {
		lines = append(lines, "")
		lines = append(lines, forcePushInterdiffDetailLines(state, ev.ID)...)
	}
	if ev.Pr != nil {
		lines = append(lines, "", ev.Pr.Title, ev.Pr.Body)
	}
	if ev.Issue != nil {
		lines = append(lines, "", ev.Issue.Title, ev.Issue.Body)
	}
	return lines
}

func detailSummaryText(ev ghpr.TimelineEvent) string {
	actor := oneLine(eventActorLabel(ev))
	action := detailActionText(ev)
	if action == "" {
		return ""
	}
	if actor == "" {
		return action
	}
	return actor + " " + action
}

func detailActionText(ev ghpr.TimelineEvent) string {
	switch ev.Type {
	case "pr.opened":
		return "opened this pull request"
	case "issue.opened":
		return "opened this issue"
	case "github.review_comment":
		return "left a review comment"
	}

	name := timelineEventName(ev)
	switch name {
	case "assigned":
		return "assigned this pull request"
	case "unassigned":
		return "removed an assignee from this pull request"
	case "labeled":
		return "added a label"
	case "unlabeled":
		return "removed a label"
	case "milestoned":
		return "added a milestone"
	case "demilestoned":
		return "removed the milestone"
	case "renamed":
		return "renamed this pull request"
	case "review_requested":
		if target := requestedReviewTarget(ev); target != "" {
			return "requested review from " + target
		}
		return "requested a review"
	case "review_request_removed":
		return "removed a review request"
	case "reviewed":
		return "submitted a review"
	case "commented":
		return "commented"
	case "line-commented":
		return "commented on a specific line"
	case "committed":
		return "pushed a commit"
	case "closed":
		return "closed this pull request"
	case "reopened":
		return "reopened this pull request"
	case "merged":
		return "merged this pull request"
	case "head_ref_deleted":
		return "deleted the head branch"
	case "head_ref_restored":
		return "restored the head branch"
	case "head_ref_force_pushed":
		return "force-pushed the head branch"
	case "base_ref_changed":
		return "changed the base branch"
	case "referenced":
		return "referenced this pull request from a commit"
	case "ready_for_review":
		return "marked this pull request ready for review"
	case "converted_to_draft":
		return "converted this pull request to draft"
	case "locked":
		return "locked the conversation"
	case "unlocked":
		return "unlocked the conversation"
	case "mentioned":
		return ""
	case "subscribed":
		return ""
	case "unsubscribed":
		return "unsubscribed from notifications"
	case "pinned":
		return "pinned this pull request"
	case "unpinned":
		return "unpinned this pull request"
	case "deployed":
		return "deployed changes from this pull request"
	case "deployment_environment_changed":
		return "changed the deployment environment"
	case "auto_merge_enabled":
		return "enabled auto-merge"
	case "auto_merge_disabled":
		return "disabled auto-merge"
	case "auto_squash_disabled":
		return "disabled auto-squash"
	case "review_dismissed":
		return "dismissed a review"
	case "connected":
		return "connected this pull request"
	case "disconnected":
		return "disconnected this pull request"
	case "transferred":
		return "transferred this issue"
	}

	if name != "" {
		return strings.ReplaceAll(name, "_", " ")
	}
	return ""
}

func timelineEventName(ev ghpr.TimelineEvent) string {
	if ev.Event != nil {
		if name := oneLine(*ev.Event); name != "" {
			return name
		}
	}

	prefix := "github.timeline."
	if strings.HasPrefix(ev.Type, prefix) && len(ev.Type) > len(prefix) {
		return ev.Type[len(prefix):]
	}
	return ""
}

func requestedReviewTarget(ev ghpr.TimelineEvent) string {
	if ev.Comment == nil || ev.Comment.Body == nil {
		return ""
	}
	return strings.TrimSpace(oneLine(*ev.Comment.Body))
}

func selectedDetailIsThreadRoot(state AppState) bool {
	ts := state.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		return false
	}
	rows := ts.threadRows(ts.activeThreadID, state.HideRead)
	idx := indexOfThreadSelection(rows, ts.threadSelectedID)
	if idx < 0 || idx >= len(rows) {
		return false
	}
	return rows[idx].isThreadRoot
}

func selectedDetailRead(state AppState) (bool, bool) {
	ts := state.currentTimeline()
	if ts == nil {
		return false, false
	}
	if ts.activeThreadID != "" && (state.Focus == focusThread || state.Focus == focusDetail) {
		rows := ts.threadRows(ts.activeThreadID, state.HideRead)
		idx := indexOfThreadSelection(rows, ts.threadSelectedID)
		if idx < 0 || idx >= len(rows) {
			return false, false
		}
		return ts.rowRead(rows[idx]), true
	}
	rows := ts.displayRows(state.HideRead)
	idx := indexOfTimelineSelection(rows, ts.selectedID)
	if idx < 0 || idx >= len(rows) {
		return false, false
	}
	return ts.rowRead(rows[idx]), true
}

func forcePushInterdiffDetailLines(state AppState, eventID string) []string {
	ts := state.currentTimeline()
	if ts == nil || eventID == "" {
		return []string{"interdiff unavailable"}
	}
	status, ok := ts.forcePushByID[eventID]
	if !ok || status.loading {
		return []string{"loading interdiff..."}
	}
	if status.err != "" {
		return []string{"failed to load interdiff: " + status.err}
	}
	lines := make([]string, 0, 8)
	if status.beforeSHA != "" {
		lines = append(lines, "before: "+status.beforeSHA)
	}
	if status.afterSHA != "" {
		lines = append(lines, "after: "+status.afterSHA)
	}
	if status.compareURL != "" {
		lines = append(lines, "compare: "+status.compareURL)
	}
	if strings.TrimSpace(status.body) == "" {
		if len(lines) == 0 {
			return []string{"interdiff unavailable"}
		}
		return lines
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, strings.Split(strings.ReplaceAll(status.body, "\r\n", "\n"), "\n")...)
	return lines
}

func commitDiffDetailLines(state AppState, eventID string) []string {
	ts := state.currentTimeline()
	if ts == nil || eventID == "" {
		return []string{"diff unavailable"}
	}
	status, ok := ts.commitDiffByID[eventID]
	if !ok || status.loading {
		return []string{"loading diff..."}
	}
	if status.err != "" {
		return []string{"failed to load diff: " + status.err}
	}
	if strings.TrimSpace(status.body) == "" {
		return []string{"diff unavailable"}
	}
	return strings.Split(strings.ReplaceAll(status.body, "\r\n", "\n"), "\n")
}
