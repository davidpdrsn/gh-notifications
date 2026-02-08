package timeline

import (
	"testing"
	"time"

	"gh-pr/internal/github"
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
