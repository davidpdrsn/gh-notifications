package timeline

import (
	"testing"
	"time"

	"gh-pr/internal/github"
	"gh-pr/internal/timelineapi"
)

func TestBuildWithComments_StableNonEmptyIDs(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	pr := github.PullRequest{
		Number:    42,
		Title:     "Test",
		Body:      "Body",
		CreatedAt: createdAt,
		User: github.User{
			Login: "alice",
			ID:    11,
			Type:  "",
		},
	}

	rawItems := []github.TimelineItem{
		{Raw: []byte(`{"id":123,"event":"committed","created_at":"2024-01-02T03:05:00Z","sha":"abc","html_url":"https://github.com/o/r/commit/abc"}`)},
		{Raw: []byte(`{"event":"cross-referenced","created_at":"2024-01-02T03:06:00Z","actor":{"login":"bob","id":22},"source":{"issue":{"id":991,"number":7}}}`)},
	}

	reviewComments := []github.ReviewComment{
		{
			ID:                777,
			NodeID:            "",
			InReplyToID:       nil,
			Path:              "src/main.rs",
			Body:              "looks good",
			DiffHunk:          "",
			HTMLURL:           "",
			PullRequestReview: 0,
			Position:          nil,
			OriginalPosition:  nil,
			Line:              nil,
			StartLine:         nil,
			CommitID:          "abc",
			OriginalCommitID:  "",
			User: github.User{
				Login: "carol",
				ID:    33,
				Type:  "",
			},
			CreatedAt: ptrTime(time.Date(2024, 1, 2, 3, 7, 0, 0, time.UTC)),
			UpdatedAt: nil,
		},
		{
			ID:                778,
			NodeID:            "",
			InReplyToID:       ptrInt64(777),
			Path:              "src/lib.rs",
			Body:              "nit",
			DiffHunk:          "",
			HTMLURL:           "",
			PullRequestReview: 0,
			Position:          nil,
			OriginalPosition:  nil,
			Line:              nil,
			StartLine:         nil,
			CommitID:          "def",
			OriginalCommitID:  "def",
			User: github.User{
				Login: "dave",
				ID:    44,
				Type:  "",
			},
			CreatedAt: ptrTime(time.Date(2024, 1, 2, 3, 8, 0, 0, time.UTC)),
			UpdatedAt: nil,
		},
	}

	eventsA, _ := BuildWithComments(pr, rawItems, reviewComments)
	eventsB, _ := BuildWithComments(pr, rawItems, reviewComments)

	if len(eventsA) != len(eventsB) {
		t.Fatalf("event count mismatch: %d vs %d", len(eventsA), len(eventsB))
	}

	for i := range eventsA {
		if eventsA[i].Id == "" {
			t.Fatalf("event %d has empty id (type=%s)", i, eventsA[i].Type)
		}
		if eventsA[i].Id != eventsB[i].Id {
			t.Fatalf("event %d id not stable: %q vs %q", i, eventsA[i].Id, eventsB[i].Id)
		}
	}

	threadIDs := make([]string, 0, 2)
	for _, event := range eventsA {
		if event.Type != "github.review_comment" || event.Comment == nil {
			continue
		}
		if event.Comment.ThreadId == nil {
			t.Fatalf("missing thread id on review comment")
		}
		threadIDs = append(threadIDs, *event.Comment.ThreadId)
	}
	if len(threadIDs) != 2 {
		t.Fatalf("expected 2 review comment events, got %d", len(threadIDs))
	}
	if threadIDs[0] != "777" || threadIDs[1] != "777" {
		t.Fatalf("unexpected thread ids: %v", threadIDs)
	}
}

func TestMapOne_FallbackIDStableAcrossJSONKeyOrder(t *testing.T) {
	raw1 := []byte(`{"event":"cross-referenced","created_at":"2024-01-02T03:06:00Z","actor":{"login":"bob","id":22},"source":{"issue":{"id":991,"number":7}}}`)
	raw2 := []byte(`{"source":{"issue":{"number":7,"id":991}},"actor":{"id":22,"login":"bob"},"created_at":"2024-01-02T03:06:00Z","event":"cross-referenced"}`)

	e1, w1, ok1 := MapTimelineItem(raw1)
	e2, w2, ok2 := MapTimelineItem(raw2)

	if w1 != "" || w2 != "" {
		t.Fatalf("unexpected warnings: %q %q", w1, w2)
	}
	if !ok1 || !ok2 {
		t.Fatalf("mapOne unexpectedly skipped event")
	}
	if e1.Id == "" || e2.Id == "" {
		t.Fatalf("fallback id missing: %q %q", e1.Id, e2.Id)
	}
	if e1.Id != e2.Id {
		t.Fatalf("fallback id changed with key order: %q vs %q", e1.Id, e2.Id)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func ptrInt64(v int64) *int64 {
	return &v
}

func TestOpenedIssueEvent_HasStableNonEmptyID(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	issue := github.Issue{
		Number:    17,
		Title:     "Issue title",
		Body:      "Issue body",
		CreatedAt: createdAt,
		User: github.User{
			Login: "alice",
			ID:    11,
			Type:  "",
		},
		PullRequest: nil,
	}

	e1 := OpenedIssueEvent(issue)
	e2 := OpenedIssueEvent(issue)

	if e1.Type != "issue.opened" {
		t.Fatalf("unexpected type: %q", e1.Type)
	}
	if e1.Id == "" {
		t.Fatalf("expected non-empty id")
	}
	if e1.Id != e2.Id {
		t.Fatalf("id not stable: %q vs %q", e1.Id, e2.Id)
	}
	if e1.Issue == nil {
		t.Fatalf("expected issue payload")
	}
	if e1.Issue.Title != "Issue title" || e1.Issue.Body != "Issue body" {
		t.Fatalf("unexpected issue payload: %+v", e1.Issue)
	}
}

func TestMapTimelineItem_CommentedIncludesCommentURL(t *testing.T) {
	raw := []byte(`{"id":555,"event":"commented","created_at":"2024-01-02T03:06:00Z","body":"issue comment","html_url":"https://github.com/o/r/issues/1#issuecomment-555","actor":{"login":"bob","id":22}}`)

	e, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("unexpected warning: %q", w)
	}
	if !ok {
		t.Fatalf("expected event to be mapped")
	}
	if e.Comment == nil || e.Comment.Url == nil {
		t.Fatalf("expected comment url to be set")
	}
	if *e.Comment.Url != "https://github.com/o/r/issues/1#issuecomment-555" {
		t.Fatalf("unexpected comment url: %q", *e.Comment.Url)
	}
}

func TestMapTimelineItem_LineCommentedIncludesCommentURL(t *testing.T) {
	raw := []byte(`{"id":777,"event":"line-commented","created_at":"2024-01-02T03:07:00Z","body":"pr line comment","html_url":"https://github.com/o/r/pull/2#discussion_r777","path":"main.go","line":12,"actor":{"login":"carol","id":33}}`)

	e, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("unexpected warning: %q", w)
	}
	if !ok {
		t.Fatalf("expected event to be mapped")
	}
	if e.Comment == nil || e.Comment.Url == nil {
		t.Fatalf("expected comment url to be set")
	}
	if *e.Comment.Url != "https://github.com/o/r/pull/2#discussion_r777" {
		t.Fatalf("unexpected comment url: %q", *e.Comment.Url)
	}
}

func TestMapTimelineItem_CommittedUsesAuthorDateWhenCreatedAtMissing(t *testing.T) {
	raw := []byte(`{"id":123,"event":"committed","sha":"abc","html_url":"https://github.com/o/r/commit/abc","author":{"date":"2024-01-02T03:05:00Z"}}`)

	e, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("unexpected warning: %q", w)
	}
	if !ok {
		t.Fatalf("expected event to be mapped")
	}
	if got := e.OccurredAt.Format(time.RFC3339); got != "2024-01-02T03:05:00Z" {
		t.Fatalf("unexpected occurred_at: %s", got)
	}
}

func TestCommitDiffURLPrefersAPICommitEndpoint(t *testing.T) {
	if got := commitDiffURL("https://github.com/o/r/commit/abc123"); got != "https://api.github.com/repos/o/r/commits/abc123" {
		t.Fatalf("unexpected api diff url from html commit url: %q", got)
	}
	if got := commitDiffURL("https://github.com/o/r/commit/abc123.diff"); got != "https://api.github.com/repos/o/r/commits/abc123" {
		t.Fatalf("unexpected api diff url from html diff url: %q", got)
	}
	if got := commitDiffURL("https://api.github.com/repos/o/r/commits/abc123"); got != "https://api.github.com/repos/o/r/commits/abc123" {
		t.Fatalf("expected api commit url to stay unchanged: %q", got)
	}
}

func TestMapTimelineItem_ForcePushedPrefersNodeID(t *testing.T) {
	raw := []byte(`{"id":22570049714,"node_id":"FP_node_id_123","event":"head_ref_force_pushed","created_at":"2026-02-05T20:19:15Z"}`)

	e, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("unexpected warning: %q", w)
	}
	if !ok {
		t.Fatalf("expected event to be mapped")
	}
	if e.Id != "FP_node_id_123" {
		t.Fatalf("expected force-push event id to prefer node_id, got %q", e.Id)
	}
}

func TestMapTimelineItem_UnsubscribedIsIgnored(t *testing.T) {
	raw := []byte(`{"id":888,"event":"unsubscribed","created_at":"2024-01-02T03:08:00Z"}`)

	_, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("expected no warning for ignored unsubscribed event, got %q", w)
	}
	if ok {
		t.Fatalf("expected unsubscribed event to be ignored")
	}
}

func TestShouldIgnorePRTimelineEvent(t *testing.T) {
	e := timelineEventWithName("cross-referenced")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected cross-referenced to be ignored")
	}

	e = timelineEventWithName("labeled")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected labeled to be ignored")
	}

	e = timelineEventWithName("subscribed")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected subscribed to be ignored")
	}

	e = timelineEventWithName("mentioned")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected mentioned to be ignored")
	}

	e = timelineEventWithName("auto_squash_enabled")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected auto_squash_enabled to be ignored")
	}

	e = timelineEventWithName("head_ref_deleted")
	if !ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected head_ref_deleted to be ignored")
	}

	e = timelineEventWithName("commented")
	if ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected commented to be included")
	}

	e = timelineEventWithName("committed")
	if ShouldIgnorePRTimelineEvent(e) {
		t.Fatalf("expected committed to be included")
	}
}

func TestBuildWithComments_IgnoresConfiguredPREvents(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	pr := github.PullRequest{
		Number:    42,
		Title:     "Test",
		Body:      "Body",
		CreatedAt: createdAt,
		User: github.User{
			Login: "alice",
			ID:    11,
			Type:  "",
		},
	}

	rawItems := []github.TimelineItem{
		{Raw: []byte(`{"id":100,"event":"cross-referenced","created_at":"2024-01-02T03:06:00Z"}`)},
		{Raw: []byte(`{"id":104,"event":"head_ref_deleted","created_at":"2024-01-02T03:06:10Z"}`)},
		{Raw: []byte(`{"id":101,"event":"labeled","created_at":"2024-01-02T03:06:30Z"}`)},
		{Raw: []byte(`{"id":102,"event":"subscribed","created_at":"2024-01-02T03:06:45Z"}`)},
		{Raw: []byte(`{"id":105,"event":"mentioned","created_at":"2024-01-02T03:06:46Z"}`)},
		{Raw: []byte(`{"id":106,"event":"auto_squash_enabled","created_at":"2024-01-02T03:06:47Z"}`)},
		{Raw: []byte(`{"id":103,"event":"commented","created_at":"2024-01-02T03:07:00Z","body":"keep me"}`)},
	}

	events, _ := BuildWithComments(pr, rawItems, nil)

	if len(events) != 2 {
		t.Fatalf("expected opened + commented only, got %d events", len(events))
	}
	if events[0].Type != "pr.opened" {
		t.Fatalf("expected first event to be pr.opened, got %q", events[0].Type)
	}
	if events[1].Event == nil || *events[1].Event != "commented" {
		t.Fatalf("expected second event to be commented, got %+v", events[1])
	}
}

func TestMapTimelineItem_ReviewRequestedIncludesRequestedReviewerInCommentBody(t *testing.T) {
	raw := []byte(`{"id":200,"event":"review_requested","created_at":"2024-01-02T03:07:00Z","actor":{"login":"bob","id":2},"requested_reviewer":{"login":"alice","id":3}}`)

	e, w, ok := MapTimelineItem(raw)
	if w != "" {
		t.Fatalf("unexpected warning: %q", w)
	}
	if !ok {
		t.Fatalf("expected event to be mapped")
	}
	if e.Comment == nil || e.Comment.Body == nil || *e.Comment.Body != "@alice" {
		t.Fatalf("expected requested reviewer in comment body, got %+v", e.Comment)
	}
}

func timelineEventWithName(name string) timelineapi.Event {
	return timelineapi.Event{Event: &name}
}
