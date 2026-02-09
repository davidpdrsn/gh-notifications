package tui

import (
	"strings"
	"testing"
	"time"

	"gh-pr/ghpr"
)

func TestColumnCopyTextReturnsTimelineRowsWhenFocused(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}

	body := "Needs timeline copy"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}}
	state.TimelineByRef[state.CurrentRef].insertTimelineEvent(ev)

	column, text := columnCopyText(state)
	if column != "timeline" {
		t.Fatalf("expected timeline column, got %q", column)
	}
	if !strings.Contains(text, "Needs timeline copy") {
		t.Fatalf("expected timeline content in copied text, got %q", text)
	}
}

func TestColumnCopyTextReturnsDetailWhenFocused(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}

	body := "Detailed body"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{Body: &body}}
	ts := state.TimelineByRef[state.CurrentRef]
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	column, text := columnCopyText(state)
	if column != "detail" {
		t.Fatalf("expected detail column, got %q", column)
	}
	if !strings.Contains(text, "type: github.timeline.commented") {
		t.Fatalf("expected event type in detail text, got %q", text)
	}
	if !strings.Contains(text, "Detailed body") {
		t.Fatalf("expected body in detail text, got %q", text)
	}
}

func TestDetailColumnTextIncludesCommitDiffBody(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID: map[string]commitDiffState{
			"e1": {body: "diff --git a/main.go b/main.go\n+added"},
		},
	}

	sha := "abc123"
	url := "https://github.com/owner/repo/commit/abc123"
	diffURL := "https://api.github.com/repos/owner/repo/commits/abc123"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.committed", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Commit: &ghpr.CommitContext{SHA: &sha, URL: &url}, DiffURL: &diffURL}
	ts := state.TimelineByRef[state.CurrentRef]
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	_, text := columnCopyText(state)
	if !strings.Contains(text, "sha: abc123") {
		t.Fatalf("expected commit sha in detail text, got %q", text)
	}
	if !strings.Contains(text, "diff --git a/main.go b/main.go") {
		t.Fatalf("expected commit diff in detail text, got %q", text)
	}
}

func TestNotificationsColumnTextAlignsTitleColumn(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", repo: "o/repo", title: "first title"},
		{id: "n2", repo: "owner/very-long-repository-name", title: "second title"},
	}

	_, text := columnCopyText(state)
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %v", lines)
	}
	idx1 := strings.Index(lines[0], "first title")
	idx2 := strings.Index(lines[1], "second title")
	if idx1 < 0 || idx2 < 0 {
		t.Fatalf("expected titles in copied text, got %q", text)
	}
	if idx1 != idx2 {
		t.Fatalf("expected aligned title columns, got indexes %d and %d in %q", idx1, idx2, text)
	}
}

func TestDetailColumnTextIncludesForcePushInterdiff(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		forcePushByID: map[string]forcePushDiffState{
			"fp1": {
				beforeSHA:  "1111111",
				afterSHA:   "2222222",
				compareURL: "https://github.com/owner/repo/compare/1111111...2222222",
				body:       "diff --git a/main.go b/main.go\n+added",
			},
		},
	}
	ev := ghpr.TimelineEvent{ID: "fp1", Type: "github.timeline.head_ref_force_pushed", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC)}
	ts := state.TimelineByRef[state.CurrentRef]
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("fp1")

	_, text := columnCopyText(state)
	if !strings.Contains(text, "before: 1111111") {
		t.Fatalf("expected before sha in detail text, got %q", text)
	}
	if !strings.Contains(text, "compare: https://github.com/owner/repo/compare/1111111...2222222") {
		t.Fatalf("expected compare url in detail text, got %q", text)
	}
	if !strings.Contains(text, "diff --git a/main.go b/main.go") {
		t.Fatalf("expected interdiff body in detail text, got %q", text)
	}
}

func TestDetailColumnTextShowsThreadRootDiffAboveBody(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "t1"
	rootBody := "Root comment body"
	replyBody := "Reply body"
	rootHunk := "@@ -10,2 +10,3 @@\n-old\n+new"
	root := ghpr.TimelineEvent{
		ID:         "c1",
		Type:       "github.review_comment",
		OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC),
		Comment: &ghpr.CommentContext{
			ThreadID: &threadID,
			Body:     &rootBody,
			DiffHunk: &rootHunk,
		},
	}
	reply := ghpr.TimelineEvent{
		ID:         "c2",
		Type:       "github.review_comment",
		OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC),
		Comment: &ghpr.CommentContext{
			ThreadID: &threadID,
			Body:     &replyBody,
		},
	}
	ts.insertTimelineEvent(root)
	ts.insertTimelineEvent(reply)
	ts.selectedID = threadHeaderID(threadID)
	ts.activeThreadID = threadID
	ts.threadSelectedID = threadChildID(threadID, "c1")
	ts.threadSelectedIndex = 0

	_, text := columnCopyText(state)
	diffPos := strings.Index(text, "diff:")
	bodyPos := strings.Index(text, rootBody)
	if diffPos < 0 {
		t.Fatalf("expected diff section in detail text, got %q", text)
	}
	if bodyPos < 0 {
		t.Fatalf("expected root body in detail text, got %q", text)
	}
	if diffPos > bodyPos {
		t.Fatalf("expected diff above body; diff at %d body at %d in %q", diffPos, bodyPos, text)
	}
}

func TestDetailColumnTextShowsUsefulSummaryForAllEventTypes(t *testing.T) {
	actor := &ghpr.Actor{Login: "octocat", ID: 1}
	now := time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC)

	type tc struct {
		event    ghpr.TimelineEvent
		contains string
	}

	cases := []tc{
		{event: ghpr.TimelineEvent{ID: "e-pr-opened", Type: "pr.opened", OccurredAt: now, Actor: actor}, contains: "summary: octocat opened this pull request"},
		{event: ghpr.TimelineEvent{ID: "e-issue-opened", Type: "issue.opened", OccurredAt: now, Actor: actor}, contains: "summary: octocat opened this issue"},
		{event: ghpr.TimelineEvent{ID: "e-review-comment", Type: "github.review_comment", OccurredAt: now, Actor: actor}, contains: "summary: octocat left a review comment"},
	}

	knownTimelineEvents := []string{
		"assigned", "unassigned", "labeled", "unlabeled",
		"milestoned", "demilestoned", "renamed", "review_requested",
		"review_request_removed", "reviewed", "commented", "line-commented",
		"committed", "closed", "reopened",
		"merged", "head_ref_deleted", "head_ref_restored", "head_ref_force_pushed",
		"base_ref_changed", "referenced", "ready_for_review", "converted_to_draft",
		"locked", "unlocked",
		"unsubscribed", "pinned", "unpinned", "deployed",
		"deployment_environment_changed", "auto_merge_enabled", "auto_merge_disabled",
		"auto_squash_disabled",
		"review_dismissed", "connected", "disconnected", "transferred",
	}

	for _, name := range knownTimelineEvents {
		evName := name
		cases = append(cases, tc{
			event: ghpr.TimelineEvent{
				ID:         "e-" + evName,
				Type:       "github.timeline." + evName,
				OccurredAt: now,
				Actor:      actor,
				Event:      &evName,
			},
			contains: "summary: octocat ",
		})
	}

	for _, c := range cases {
		state := NewState()
		state.Focus = focusDetail
		state.CurrentRef = "owner/repo#1"
		state.TimelineByRef[state.CurrentRef] = &timelineState{
			ref:             state.CurrentRef,
			rowIndexByID:    map[string]int{},
			threadByID:      map[string]*threadGroup{},
			expandedThreads: map[string]bool{},
		}

		ts := state.TimelineByRef[state.CurrentRef]
		ts.insertTimelineEvent(c.event)
		ts.selectedID = eventRowID(c.event.ID)

		_, text := columnCopyText(state)
		if !strings.Contains(text, c.contains) {
			t.Fatalf("expected %q in detail text for %s, got %q", c.contains, c.event.Type, text)
		}
	}
}

func TestDetailColumnTextShowsRequestedReviewerInSummary(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}

	eventName := "review_requested"
	target := "@alice"
	ev := ghpr.TimelineEvent{
		ID:         "e-review-requested",
		Type:       "github.timeline.review_requested",
		OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC),
		Actor:      &ghpr.Actor{Login: "octocat", ID: 1},
		Event:      &eventName,
		Comment:    &ghpr.CommentContext{Body: &target},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID(ev.ID)

	_, text := columnCopyText(state)
	if !strings.Contains(text, "summary: octocat requested review from @alice") {
		t.Fatalf("expected requested reviewer in summary, got %q", text)
	}
}
