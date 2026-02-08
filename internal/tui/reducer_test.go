package tui

import (
	"testing"
	"time"

	"gh-pr/ghpr"
)

func TestReduceNotificationsSortedNewestFirstAndSelectionStable(t *testing.T) {
	state := NewState()

	older := ghpr.NotificationEvent{ID: "n1", UpdatedAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Repository: ghpr.NotificationRepository{Owner: "o", Repo: "r"}, Subject: ghpr.NotificationSubject{Title: "older"}, Target: ghpr.NotificationTarget{Kind: "issue", Number: 1, Ref: "o/r#1"}}
	newer := ghpr.NotificationEvent{ID: "n2", UpdatedAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Repository: ghpr.NotificationRepository{Owner: "o", Repo: "r"}, Subject: ghpr.NotificationSubject{Title: "newer"}, Target: ghpr.NotificationTarget{Kind: "issue", Number: 2, Ref: "o/r#2"}}

	state, _ = Reduce(state, NotificationsArrivedEvent{Generation: state.NotifGen, Item: older})
	if state.SelectedNotif != "n1" {
		t.Fatalf("expected first notification selected, got %q", state.SelectedNotif)
	}

	state, _ = Reduce(state, NotificationsArrivedEvent{Generation: state.NotifGen, Item: newer})

	if len(state.Notifications) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(state.Notifications))
	}
	if state.Notifications[0].id != "n2" || state.Notifications[1].id != "n1" {
		t.Fatalf("unexpected order: %+v", state.Notifications)
	}
	if state.SelectedNotif != "n1" {
		t.Fatalf("expected selection to stay on n1, got %q", state.SelectedNotif)
	}
}

func TestReduceTimelineSortedOldestFirst(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	later := ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC)}
	earlier := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC)}

	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: later})
	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: earlier})

	ts := state.TimelineByRef[state.CurrentRef]
	if len(ts.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(ts.rows))
	}
	if ts.rows[0].id != eventRowID("e1") || ts.rows[1].id != eventRowID("e2") {
		t.Fatalf("unexpected timeline order: %+v", ts.rows)
	}
}

func TestReduceThreadGrouping(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	threadID := "777"
	t1 := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}
	t2 := ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}

	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: t2})
	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: t1})

	ts := state.TimelineByRef[state.CurrentRef]
	if len(ts.rows) != 1 {
		t.Fatalf("expected 1 base thread row, got %d", len(ts.rows))
	}
	display := ts.displayRows()
	if len(display) != 3 {
		t.Fatalf("expected thread header + 2 children, got %d", len(display))
	}
	if display[0].id != threadHeaderID(threadID) {
		t.Fatalf("unexpected thread header id: %q", display[0].id)
	}
}

func TestReduceIgnoresStaleTimelineGeneration(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 2
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC()}
	state, _ = Reduce(state, TimelineArrivedEvent{Generation: 1, Ref: state.CurrentRef, Event: ev})

	ts := state.TimelineByRef[state.CurrentRef]
	if len(ts.rows) != 0 {
		t.Fatalf("expected stale event to be ignored, got %d rows", len(ts.rows))
	}
}

func TestMoveNotificationsCancelsPreviousTimelineEvenIfTargetCached(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1"},
		{id: "n2", ref: "o/r#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, loading: true}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, done: true}

	next, effects := Reduce(state, KeyEvent{Key: "j"})

	if next.CurrentRef != "o/r#2" {
		t.Fatalf("expected current ref o/r#2, got %q", next.CurrentRef)
	}
	if next.TimelineGen != 1 {
		t.Fatalf("expected timeline generation 1 after switching refs, got %d", next.TimelineGen)
	}
	hasCancel := false
	for _, eff := range effects {
		if _, ok := eff.(CancelTimelineEffect); ok {
			hasCancel = true
			break
		}
	}
	if !hasCancel {
		t.Fatalf("expected CancelTimelineEffect when switching refs")
	}
}

func TestReduceIgnoresTimelineArrivedForNonCurrentRefEvenIfGenerationMatches(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#2"
	state.TimelineGen = 3
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC()}
	next, _ := Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: "o/r#1", Event: ev})

	if got := len(next.TimelineByRef["o/r#1"].rows); got != 0 {
		t.Fatalf("expected non-current ref event to be ignored, got %d rows", got)
	}
}

func TestReduceIgnoresTimelineTerminalEventsForNonCurrentRef(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#2"
	state.TimelineGen = 4
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, loading: true}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, loading: true}

	next, _ := Reduce(state, TimelineDoneEvent{Generation: state.TimelineGen, Ref: "o/r#1"})
	if !next.TimelineByRef["o/r#1"].loading {
		t.Fatalf("expected non-current ref done event to be ignored")
	}

	next, _ = Reduce(next, TimelineErrEvent{Generation: state.TimelineGen, Ref: "o/r#1", Err: "boom"})
	if next.TimelineByRef["o/r#1"].err != "" {
		t.Fatalf("expected non-current ref error event to be ignored")
	}
}

func TestMoveNotificationsRestartsTimelineWhenReturningToStaleLoadingRef(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1"},
		{id: "n2", ref: "o/r#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, loading: true}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	next, effects := Reduce(state, KeyEvent{Key: "j"})
	if next.CurrentRef != "o/r#2" {
		t.Fatalf("expected current ref o/r#2, got %q", next.CurrentRef)
	}
	if next.TimelineGen != 1 {
		t.Fatalf("expected timeline generation 1 after first switch, got %d", next.TimelineGen)
	}
	foundStartR2 := false
	for _, eff := range effects {
		if start, ok := eff.(StartTimelineEffect); ok && start.Ref == "o/r#2" {
			foundStartR2 = true
		}
	}
	if !foundStartR2 {
		t.Fatalf("expected StartTimelineEffect for o/r#2")
	}

	next, effects = Reduce(next, KeyEvent{Key: "k"})
	if next.CurrentRef != "o/r#1" {
		t.Fatalf("expected current ref o/r#1, got %q", next.CurrentRef)
	}
	if next.TimelineGen != 2 {
		t.Fatalf("expected timeline generation 2 after switching back, got %d", next.TimelineGen)
	}

	foundStartR1 := false
	for _, eff := range effects {
		if start, ok := eff.(StartTimelineEffect); ok && start.Ref == "o/r#1" {
			foundStartR1 = true
		}
	}
	if !foundStartR1 {
		t.Fatalf("expected StartTimelineEffect for o/r#1 when returning to stale-loading ref")
	}
}

func TestSwitchToCachedRefInvalidatesOldGenerationEvents(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1"},
		{id: "n2", ref: "o/r#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, loading: true}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, done: true}

	next, _ := Reduce(state, KeyEvent{Key: "j"})
	if next.TimelineGen != 1 {
		t.Fatalf("expected timeline generation 1 after switching to cached ref, got %d", next.TimelineGen)
	}

	ev := ghpr.TimelineEvent{ID: "e-old", Type: "github.timeline.commented", OccurredAt: time.Now().UTC()}
	next, _ = Reduce(next, TimelineArrivedEvent{Generation: 0, Ref: "o/r#1", Event: ev})
	if got := len(next.TimelineByRef["o/r#1"].rows); got != 0 {
		t.Fatalf("expected stale old-generation event to be ignored, got %d rows", got)
	}
}

func TestReduceThreadGroupingDedupsSameCommentID(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}

	threadID := "777"
	ev := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}

	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: ev})
	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: ev})

	ts := state.TimelineByRef[state.CurrentRef]
	if got := len(ts.threadByID[threadID].items); got != 1 {
		t.Fatalf("expected 1 unique thread item, got %d", got)
	}
}

func TestBackspaceOnThreadChildCollapsesThreadBeforeLeavingPane(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	ev := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}
	ts.insertTimelineEvent(ev)
	ts.selectedID = threadChildID(threadID, "c1")

	next, _ := Reduce(state, KeyEvent{Key: "backspace"})
	nextTS := next.TimelineByRef[next.CurrentRef]
	if next.Focus != focusTimeline {
		t.Fatalf("expected to stay in timeline focus, got %v", next.Focus)
	}
	if nextTS.expandedThreads[threadID] {
		t.Fatalf("expected thread to collapse")
	}
	if nextTS.selectedID != threadHeaderID(threadID) {
		t.Fatalf("expected selection on thread header, got %q", nextTS.selectedID)
	}
}

func TestNotificationsScrollFollowsSelectionAtBottom(t *testing.T) {
	state := NewState()
	state.Height = 10
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1"},
		{id: "n2", ref: "o/r#2"},
		{id: "n3", ref: "o/r#3"},
		{id: "n4", ref: "o/r#4"},
		{id: "n5", ref: "o/r#5"},
		{id: "n6", ref: "o/r#6"},
		{id: "n7", ref: "o/r#7"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	for i := 0; i < 6; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "j"})
	}

	if state.NotifSelected != 6 {
		t.Fatalf("expected selection at last row, got %d", state.NotifSelected)
	}
	viewport := notificationViewportRows(state)
	if state.NotifSelected < state.NotifScroll || state.NotifSelected >= state.NotifScroll+viewport {
		t.Fatalf("expected selection to remain visible; selected=%d scroll=%d viewport=%d", state.NotifSelected, state.NotifScroll, viewport)
	}
}
