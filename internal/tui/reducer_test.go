package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gh-pr/ghpr"

	"github.com/charmbracelet/lipgloss"
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
	body1 := "First thread message"
	body2 := "Second thread message"
	t1 := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Actor: &ghpr.Actor{Login: "alice"}, Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body1}}
	t2 := ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Actor: &ghpr.Actor{Login: "bob"}, Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body2}}

	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: t2})
	state, _ = Reduce(state, TimelineArrivedEvent{Generation: state.TimelineGen, Ref: state.CurrentRef, Event: t1})

	ts := state.TimelineByRef[state.CurrentRef]
	if len(ts.rows) != 1 {
		t.Fatalf("expected 1 base thread row, got %d", len(ts.rows))
	}
	display := ts.displayRows(false)
	if len(display) != 1 {
		t.Fatalf("expected only thread root row in timeline, got %d", len(display))
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

func TestBackspaceInThreadFocusReturnsToTimelineAndClearsDrill(t *testing.T) {
	state := NewState()
	state.Focus = focusThread
	state.CurrentRef = "o/r#1"
	state.Notifications = []notifRow{{id: "n1", ref: "o/r#1"}}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	ev := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}
	ts.insertTimelineEvent(ev)
	ts.selectedID = threadHeaderID(threadID)
	ts.activeThreadID = threadID

	next, _ := Reduce(state, KeyEvent{Key: "backspace"})
	if next.Focus != focusTimeline {
		t.Fatalf("expected to return to timeline focus, got %v", next.Focus)
	}
	if next.TimelineByRef[next.CurrentRef].activeThreadID != "" {
		t.Fatalf("expected active thread cleared after backspace")
	}
}

func TestDrillInOnThreadHeaderOpensThread(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	body1 := "first"
	body2 := "second"
	ev1 := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body1}}
	ev2 := ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body2}}
	ts.insertTimelineEvent(ev1)
	ts.insertTimelineEvent(ev2)
	ts.selectedID = threadHeaderID(threadID)

	next, _ := Reduce(state, KeyEvent{Key: "l"})
	nextTS := next.TimelineByRef[next.CurrentRef]
	if next.Focus != focusThread {
		t.Fatalf("expected to drill into thread from root, got %v", next.Focus)
	}
	if nextTS.activeThreadID != threadID {
		t.Fatalf("expected active thread %q, got %q", threadID, nextTS.activeThreadID)
	}
	if nextTS.threadSelectedID != threadChildID(threadID, "c1") {
		t.Fatalf("expected root selected, got %q", nextTS.threadSelectedID)
	}
}

func TestDrillInOnThreadFocusOpensDetail(t *testing.T) {
	state := NewState()
	state.Focus = focusThread
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	body1 := "first"
	body2 := "second"
	ev1 := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body1}}
	ev2 := ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body2}}
	ts.insertTimelineEvent(ev1)
	ts.insertTimelineEvent(ev2)
	ts.selectedID = threadHeaderID(threadID)
	ts.activeThreadID = threadID
	ts.threadSelectedID = threadChildID(threadID, "c2")
	ts.threadSelectedIndex = 0

	next, _ := Reduce(state, KeyEvent{Key: "l"})
	if next.Focus != focusDetail {
		t.Fatalf("expected to open detail from thread focus, got %v", next.Focus)
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

func TestNotificationsScrollFollowsSelectionWithWrappedRows(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 10
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", updatedAt: time.Now().Add(-time.Hour), repo: "owner/repo", title: "A very long title that wraps in this viewport width"},
		{id: "n2", ref: "o/r#2", updatedAt: time.Now().Add(-2 * time.Hour), repo: "owner/repo", title: "Another very long title that also wraps and consumes lines"},
		{id: "n3", ref: "o/r#3", updatedAt: time.Now().Add(-3 * time.Hour), repo: "owner/repo", title: "Third long title that should force scroll accounting by wrapped lines"},
		{id: "n4", ref: "o/r#4", updatedAt: time.Now().Add(-4 * time.Hour), repo: "owner/repo", title: "Fourth long title for wrapped row scrolling"},
		{id: "n5", ref: "o/r#5", updatedAt: time.Now().Add(-5 * time.Hour), repo: "owner/repo", title: "Fifth long title"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	for i := 0; i < len(state.Notifications)-1; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "j"})
		if !notificationSelectionVisibleWithWrap(state) {
			t.Fatalf("expected wrapped selection visible after move %d; selected=%d scroll=%d", i+1, state.NotifSelected, state.NotifScroll)
		}
	}
}

func notificationSelectionVisibleWithWrap(state AppState) bool {
	viewport := notificationViewportRows(state)
	mode := state.currentPaneMode()
	leftW, _, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	avail := contentWidth(leftW)
	if avail < 1 {
		avail = 1
	}
	timeColWidth := notificationTimeColumnWidth(state.Notifications)

	used := 0
	selectedOffset := -1
	selectedHeight := 1
	for i := state.NotifScroll; i < len(state.Notifications); i++ {
		prefix := padToDisplayWidth(timeAgo(state.Notifications[i].updatedAt), timeColWidth) + " "
		label := prefix + state.Notifications[i].repo + "  " + oneLine(state.Notifications[i].title)
		h := len(wrapDisplayWidth(label, avail, strings.Repeat(" ", lipgloss.Width(prefix))))
		if h < 1 {
			h = 1
		}
		if i == state.NotifSelected {
			selectedOffset = used
			selectedHeight = h
			break
		}
		used += h
		if used >= viewport {
			break
		}
	}
	if selectedOffset < 0 {
		return false
	}
	return selectedOffset < viewport && selectedOffset+selectedHeight > 0
}

func timelineSelectionVisibleWithWrap(state AppState) bool {
	ts := state.currentTimeline()
	if ts == nil {
		return false
	}
	rows := ts.displayRows(false)
	if len(rows) == 0 || ts.selectedIndex < 0 || ts.selectedIndex >= len(rows) {
		return false
	}
	viewport := timelineViewportRows(state)
	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	avail := contentWidth(midW)
	if avail < 1 {
		avail = 1
	}
	kindWidth := timelineKindColumnWidth(rows)
	timeWidth := timelineTimeColumnWidth(rows)
	actorWidth := timelineActorColumnWidth(rows)

	used := 0
	for i := ts.scrollOffset; i < len(rows); i++ {
		h := len(wrapTimelineRow(rows[i], ts, avail, timeWidth, kindWidth, actorWidth, true))
		if h < 1 {
			h = 1
		}
		if i == ts.selectedIndex {
			return used < viewport && used+h > 0
		}
		if used+h > viewport && used > 0 {
			break
		}
		used += h
		if used >= viewport {
			break
		}
	}
	return false
}

func TestThreadRowsContainOnlyReplies(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	t1 := ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}
	t2 := ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Date(2024, 1, 2, 4, 0, 0, 0, time.UTC), Comment: &ghpr.CommentContext{ThreadID: &threadID}}

	ts.insertTimelineEvent(t1)
	ts.insertTimelineEvent(t2)

	display := ts.displayRows(false)
	if len(display) != 1 {
		t.Fatalf("expected only thread root in timeline display, got %d rows", len(display))
	}
	replies := ts.threadRows(threadID, false)
	if len(replies) != 2 {
		t.Fatalf("expected root + one reply row, got %d", len(replies))
	}
	if replies[0].id != threadChildID(threadID, "c1") {
		t.Fatalf("expected root row id for first comment, got %q", replies[0].id)
	}
	if replies[1].id != threadChildID(threadID, "c2") {
		t.Fatalf("expected reply row id for newest comment, got %q", replies[1].id)
	}
}

func TestCompactThreadPath(t *testing.T) {
	got := compactThreadPath("RoomByRoom/RoomByRoom/Views/Project/RoomOverview/RoomOverviewView.swift")
	want := "RoomByRoom/../RoomOverview/RoomOverviewView.swift"
	if got != want {
		t.Fatalf("expected compact path %q, got %q", want, got)
	}

	short := compactThreadPath("RoomByRoom/RoomOverviewView.swift")
	if short != "RoomByRoom/RoomOverviewView.swift" {
		t.Fatalf("expected short path unchanged, got %q", short)
	}
}

func TestThreadHeaderUsesCompactedPath(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	threadID := "777"
	path := "RoomByRoom/RoomByRoom/Views/Project/RoomOverview/RoomOverviewView.swift"
	ev := ghpr.TimelineEvent{
		ID:         "c1",
		Type:       "github.review_comment",
		OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC),
		Comment:    &ghpr.CommentContext{ThreadID: &threadID, Path: &path},
	}
	ts.insertTimelineEvent(ev)

	display := ts.displayRows(false)
	if len(display) == 0 {
		t.Fatal("expected at least one display row")
	}
	lines := wrapTimelineRow(display[0], ts, 120, 3, 12, 12, true)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "RoomByRoom/../RoomOverview/RoomOverviewView.swift") {
		t.Fatalf("expected compacted thread path in rendered row, got %q", joined)
	}
}

func TestTimelineRowsUseCompactCommentPreview(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	ts := state.TimelineByRef[state.CurrentRef]

	actor := ghpr.Actor{Login: "octocat"}
	body := strings.Repeat("Very long comment body segment ", 20)
	ev := ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC),
		Actor:      &actor,
		Comment:    &ghpr.CommentContext{Body: &body},
	}
	ts.insertTimelineEvent(ev)

	display := ts.displayRows(false)
	if len(display) != 1 {
		t.Fatalf("expected 1 row, got %d", len(display))
	}
	label := display[0].label
	if !strings.HasPrefix(label, "commented  octocat  ") {
		t.Fatalf("expected compact label with kind and actor, got %q", label)
	}
	if strings.Contains(label, body) {
		t.Fatalf("expected long body to be truncated in row label")
	}
	if !strings.Contains(label, "...") {
		t.Fatalf("expected truncated row label to end with ellipsis, got %q", label)
	}
}

func TestShiftCCreatesCopyEffectForFocusedColumn(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{{id: "n1", repo: "owner/repo", title: "Add keybind", ref: "owner/repo#1"}}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"

	next, effects := Reduce(state, KeyEvent{Key: "C"})

	if next.Status != "" {
		t.Fatalf("expected status unchanged before copy result, got %q", next.Status)
	}
	if len(effects) != 1 {
		t.Fatalf("expected exactly one effect, got %d", len(effects))
	}
	copyEff, ok := effects[0].(CopyColumnEffect)
	if !ok {
		t.Fatalf("expected CopyColumnEffect, got %T", effects[0])
	}
	if copyEff.Column != "notifications" {
		t.Fatalf("expected notifications column, got %q", copyEff.Column)
	}
	if !strings.Contains(copyEff.Text, "owner/repo") {
		t.Fatalf("expected copied text to include notification content, got %q", copyEff.Text)
	}
}

func TestClipboardResultUpdatesStatus(t *testing.T) {
	state := NewState()

	next, _ := Reduce(state, ClipboardCopiedEvent{Column: "timeline"})
	if next.Status != "copied timeline column" {
		t.Fatalf("unexpected success status: %q", next.Status)
	}

	next, _ = Reduce(next, ClipboardCopyFailedEvent{Column: "timeline", Err: "boom"})
	if next.Status != "copy failed (timeline): boom" {
		t.Fatalf("unexpected failure status: %q", next.Status)
	}
}

func TestSelectingCommittedEventQueuesDiffLoad(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	ts := state.TimelineByRef[state.CurrentRef]

	sha := "abc123"
	url := "https://github.com/o/r/commit/abc123"
	diff := "https://api.github.com/repos/o/r/commits/abc123"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.committed", OccurredAt: time.Now().UTC(), Commit: &ghpr.CommitContext{SHA: &sha, URL: &url}, DiffURL: &diff}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	next, effects := Reduce(state, WindowSizeEvent{Width: 100, Height: 20})

	if len(effects) != 1 {
		t.Fatalf("expected one effect, got %d", len(effects))
	}
	start, ok := effects[0].(StartCommitDiffEffect)
	if !ok {
		t.Fatalf("expected StartCommitDiffEffect, got %T", effects[0])
	}
	if start.Ref != state.CurrentRef || start.EventID != "e1" {
		t.Fatalf("unexpected effect payload: %+v", start)
	}
	status := next.TimelineByRef[next.CurrentRef].commitDiffByID["e1"]
	if !status.loading {
		t.Fatalf("expected commit diff to be marked loading")
	}
}

func TestCommitDiffEventsUpdateState(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{"e1": {loading: true}}}

	next, _ := Reduce(state, CommitDiffLoadedEvent{Ref: state.CurrentRef, EventID: "e1", Diff: "diff --git"})
	if next.TimelineByRef[state.CurrentRef].commitDiffByID["e1"].body == "" {
		t.Fatalf("expected loaded diff body")
	}

	next, _ = Reduce(next, CommitDiffErrEvent{Ref: state.CurrentRef, EventID: "e2", Err: "boom"})
	if next.TimelineByRef[state.CurrentRef].commitDiffByID["e2"].err != "boom" {
		t.Fatalf("expected error state for e2")
	}
	if next.Status != "failed to load commit diff" {
		t.Fatalf("expected status update on diff error, got %q", next.Status)
	}
}

func TestSelectingForcePushEventQueuesInterdiffLoad(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}, forcePushByID: map[string]forcePushDiffState{}}
	ts := state.TimelineByRef[state.CurrentRef]

	ev := ghpr.TimelineEvent{ID: "fp1", Type: "github.timeline.head_ref_force_pushed", OccurredAt: time.Now().UTC()}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("fp1")

	next, effects := Reduce(state, WindowSizeEvent{Width: 100, Height: 20})

	if len(effects) != 1 {
		t.Fatalf("expected one effect, got %d", len(effects))
	}
	start, ok := effects[0].(StartForcePushInterdiffEffect)
	if !ok {
		t.Fatalf("expected StartForcePushInterdiffEffect, got %T", effects[0])
	}
	if start.Ref != state.CurrentRef || start.EventID != "fp1" {
		t.Fatalf("unexpected effect payload: %+v", start)
	}
	status := next.TimelineByRef[next.CurrentRef].forcePushByID["fp1"]
	if !status.loading {
		t.Fatalf("expected force-push interdiff to be marked loading")
	}
}

func TestForcePushInterdiffEventsUpdateState(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}, forcePushByID: map[string]forcePushDiffState{"fp1": {loading: true}}}

	next, _ := Reduce(state, ForcePushInterdiffLoadedEvent{Ref: state.CurrentRef, EventID: "fp1", BeforeSHA: "a1", AfterSHA: "b2", CompareURL: "https://github.com/o/r/compare/a1...b2", Diff: "diff --git"})
	got := next.TimelineByRef[state.CurrentRef].forcePushByID["fp1"]
	if got.body == "" || got.beforeSHA != "a1" || got.afterSHA != "b2" {
		t.Fatalf("expected loaded force-push interdiff state, got %+v", got)
	}

	next, _ = Reduce(next, ForcePushInterdiffErrEvent{Ref: state.CurrentRef, EventID: "fp2", Err: "boom"})
	if next.TimelineByRef[state.CurrentRef].forcePushByID["fp2"].err != "boom" {
		t.Fatalf("expected error state for fp2")
	}
	if next.Status != "failed to load force-push interdiff" {
		t.Fatalf("expected status update on force-push interdiff error, got %q", next.Status)
	}
}

func TestDetailScrollKeybindsWorkRegardlessOfFocus(t *testing.T) {
	state := NewState()
	state.Width = 100
	state.Height = 20

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+n"})
	if next.DetailScroll != 1 {
		t.Fatalf("expected detail scroll to increment, got %d", next.DetailScroll)
	}

	next.Focus = focusTimeline
	next.CurrentRef = "o/r#1"
	long := strings.Repeat("line\n", 80)
	next.TimelineByRef[next.CurrentRef] = &timelineState{ref: next.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	ts := next.TimelineByRef[next.CurrentRef]
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &long}}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	next, _ = Reduce(next, KeyEvent{Key: "ctrl+n"})
	if next.DetailScroll != 2 {
		t.Fatalf("expected detail scroll to increment in timeline focus, got %d", next.DetailScroll)
	}

	next, _ = Reduce(next, KeyEvent{Key: "ctrl+p"})
	if next.DetailScroll != 1 {
		t.Fatalf("expected detail scroll to decrement, got %d", next.DetailScroll)
	}

	next.Focus = focusDetail
	next, _ = Reduce(next, KeyEvent{Key: "ctrl+d"})
	if next.DetailScroll != 11 {
		t.Fatalf("expected detail scroll to jump by 10 in detail focus, got %d", next.DetailScroll)
	}

	next, _ = Reduce(next, KeyEvent{Key: "ctrl+u"})
	if next.DetailScroll != 1 {
		t.Fatalf("expected detail scroll to jump back by 10 in detail focus, got %d", next.DetailScroll)
	}
}

func TestCtrlDUScrollsFocusedNotificationsByTen(t *testing.T) {
	state := NewState()
	state.Width = 160
	state.Height = 24
	state.Focus = focusNotifications
	state.Notifications = make([]notifRow, 0, 25)
	for i := 0; i < 25; i++ {
		state.Notifications = append(state.Notifications, notifRow{
			id:        fmt.Sprintf("n%d", i+1),
			repo:      "o/r",
			ref:       fmt.Sprintf("o/r#%d", i+1),
			updatedAt: time.Now().UTC().Add(-time.Duration(i) * time.Minute),
		})
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+d"})
	if next.NotifSelected != 10 {
		t.Fatalf("expected ctrl+d to move notifications selection down 10, got %d", next.NotifSelected)
	}
	if next.SelectedNotif != "n11" {
		t.Fatalf("expected selected notification n11, got %q", next.SelectedNotif)
	}

	next, _ = Reduce(next, KeyEvent{Key: "ctrl+u"})
	if next.NotifSelected != 0 {
		t.Fatalf("expected ctrl+u to move notifications selection up 10, got %d", next.NotifSelected)
	}
	if next.SelectedNotif != "n1" {
		t.Fatalf("expected selected notification n1, got %q", next.SelectedNotif)
	}
}

func TestCtrlDUScrollsFocusedTimelineByTen(t *testing.T) {
	state := NewState()
	state.Width = 160
	state.Height = 24
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := "body"
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("e%d", i+1)
		ts.insertTimelineEvent(ghpr.TimelineEvent{ID: id, Type: "github.timeline.commented", OccurredAt: time.Now().UTC().Add(time.Duration(i) * time.Minute), Comment: &ghpr.CommentContext{Body: &body}})
	}
	ts.selectedID = eventRowID("e1")

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+d"})
	rows := next.TimelineByRef[next.CurrentRef].rowsReadyForDisplay(next.TimelineByRef[next.CurrentRef].displayRows(next.HideRead))
	idx := indexOfTimelineSelection(rows, next.TimelineByRef[next.CurrentRef].selectedID)
	if idx != 10 {
		t.Fatalf("expected ctrl+d to move timeline selection down 10, got %d", idx)
	}

	next, _ = Reduce(next, KeyEvent{Key: "ctrl+u"})
	rows = next.TimelineByRef[next.CurrentRef].rowsReadyForDisplay(next.TimelineByRef[next.CurrentRef].displayRows(next.HideRead))
	idx = indexOfTimelineSelection(rows, next.TimelineByRef[next.CurrentRef].selectedID)
	if idx != 0 {
		t.Fatalf("expected ctrl+u to move timeline selection up 10, got %d", idx)
	}
}

func TestCtrlDUKeepsTimelineSelectionVisibleWithWrappedRows(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 8
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	long := strings.Repeat("wrapped timeline row content ", 25)
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("wd%d", i)
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &long},
		})
	}
	ts.selectedID = eventRowID("wd0")

	for i := 0; i < 3; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "ctrl+d"})
		if !timelineSelectionVisibleWithWrap(state) {
			t.Fatalf("expected timeline selection visible after ctrl+d #%d; selected=%d scroll=%d", i+1, ts.selectedIndex, ts.scrollOffset)
		}
	}
	for i := 0; i < 3; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "ctrl+u"})
		if !timelineSelectionVisibleWithWrap(state) {
			t.Fatalf("expected timeline selection visible after ctrl+u #%d; selected=%d scroll=%d", i+1, ts.selectedIndex, ts.scrollOffset)
		}
	}
}

func TestDetailArrowMovesLeftPaneSelectionInDetailFocus(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	ts := &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	state.TimelineByRef[state.CurrentRef] = ts

	body1 := strings.Repeat("line of detail text ", 200)
	body2 := strings.Repeat("another line of detail text ", 200)
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body1}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{Body: &body2}})
	ts.selectedID = eventRowID("e1")
	state.DetailScroll = 7

	state, _ = Reduce(state, KeyEvent{Key: "down"})
	if ts.selectedID != eventRowID("e2") {
		t.Fatalf("expected timeline selection to move down from detail focus, got %q", ts.selectedID)
	}
	if state.DetailScroll != 0 {
		t.Fatalf("expected detail scroll reset on selection change, got %d", state.DetailScroll)
	}

	state, _ = Reduce(state, KeyEvent{Key: "up"})
	if ts.selectedID != eventRowID("e1") {
		t.Fatalf("expected timeline selection to move up from detail focus, got %q", ts.selectedID)
	}
}

func TestDetailArrowMovesThreadSelectionInDetailFocus(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	ts := &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	state.TimelineByRef[state.CurrentRef] = ts

	threadID := "t1"
	rootBody := "root"
	replyBody := "reply"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &rootBody}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &replyBody}})
	ts.selectedID = threadHeaderID(threadID)
	ts.activeThreadID = threadID
	ts.threadSelectedID = threadChildID(threadID, "c1")
	ts.threadSelectedIndex = 0
	state.DetailScroll = 5

	state, _ = Reduce(state, KeyEvent{Key: "down"})
	if ts.threadSelectedID != threadChildID(threadID, "c2") {
		t.Fatalf("expected thread selection to move down from detail focus, got %q", ts.threadSelectedID)
	}
	if state.DetailScroll != 0 {
		t.Fatalf("expected detail scroll reset on thread selection change, got %d", state.DetailScroll)
	}

	state, _ = Reduce(state, KeyEvent{Key: "up"})
	if ts.threadSelectedID != threadChildID(threadID, "c1") {
		t.Fatalf("expected thread selection to move up from detail focus, got %q", ts.threadSelectedID)
	}
}

func TestTimelineScrollFollowsSelectionWithWrappedRows(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 8
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	ts := state.TimelineByRef[state.CurrentRef]

	long := strings.Repeat("wrapped timeline row content ", 20)
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("e%d", i+1)
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &long},
		})
	}
	ts.selectedID = eventRowID("e1")

	for i := 0; i < 6; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "down"})
	}
	if ts.selectedIndex <= 0 {
		t.Fatalf("expected selection to move down, got %d", ts.selectedIndex)
	}
	if ts.scrollOffset <= 0 {
		t.Fatalf("expected timeline scroll offset to increase, got %d", ts.scrollOffset)
	}
}

func TestTimelineSelectionRemainsVisibleWithJKWrappedRows(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 8
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	long := strings.Repeat("wrapped timeline row content ", 25)
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("j%d", i)
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &long},
		})
	}
	ts.selectedID = eventRowID("j0")

	for i := 0; i < 10; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "j"})
		if !timelineSelectionVisibleWithWrap(state) {
			t.Fatalf("expected timeline selection visible after move %d; selected=%d scroll=%d", i+1, ts.selectedIndex, ts.scrollOffset)
		}
	}
}

func TestTimelineSelectionVisibleWhenPreviousRowsWrapPastViewport(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 8
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	ts := state.TimelineByRef[state.CurrentRef]

	veryLong := strings.Repeat("very wrapped timeline row content ", 40)
	short := "short"
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("w%d", i)
		body := veryLong
		if i == 5 {
			body = short
		}
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &body},
		})
	}
	ts.selectedID = eventRowID("w0")

	for i := 0; i < 5; i++ {
		state, _ = Reduce(state, KeyEvent{Key: "j"})
	}
	if !timelineSelectionVisibleWithWrap(state) {
		t.Fatalf("expected selected row to remain visible after wrapped rows; selected=%d scroll=%d", ts.selectedIndex, ts.scrollOffset)
	}
}

func TestMouseWheelScrollsNotifications(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusNotifications
	state.NotifScroll = 0
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", updatedAt: time.Now(), title: "first"},
		{id: "n2", ref: "o/r#2", updatedAt: time.Now().Add(-time.Minute), title: "second"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0

	state, _ = Reduce(state, MouseWheelEvent{X: 1, Y: 10, Delta: 1})

	if state.NotifScroll != 1 {
		t.Fatalf("expected notifications scroll to move down by 1, got %d", state.NotifScroll)
	}
}

func TestMouseWheelScrollsTimeline(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: make(map[string]bool),
	}
	ts := state.TimelineByRef[state.CurrentRef]
	ts.scrollOffset = 2
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now().Add(-time.Minute),
	})
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e2",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now(),
	})

	state, _ = Reduce(state, MouseWheelEvent{X: 1, Y: 10, Delta: -1})

	if state.TimelineByRef[state.CurrentRef].scrollOffset != 1 {
		t.Fatalf("expected timeline scroll to move up by 1, got %d", state.TimelineByRef[state.CurrentRef].scrollOffset)
	}
}

func TestMouseWheelTimelineScrollStaysAtOffsetAfterTimelineUpdate(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: make(map[string]bool),
	}
	ts := state.TimelineByRef[state.CurrentRef]
	for i := 0; i < 20; i++ {
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         string(rune('a' + i)),
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}
	ts.selectedID = eventRowID(string(rune('a')))

	state, _ = Reduce(state, MouseWheelEvent{X: 1, Y: 10, Delta: 5})
	if state.TimelineByRef[state.CurrentRef].scrollOffset != 5 {
		t.Fatalf("expected timeline scroll to move to 5, got %d", state.TimelineByRef[state.CurrentRef].scrollOffset)
	}

	state, _ = Reduce(state, TimelineArrivedEvent{
		Generation: state.TimelineGen,
		Ref:        state.CurrentRef,
		Event:      ghpr.TimelineEvent{ID: "zz", Type: "github.timeline.commented", OccurredAt: time.Now().Add(30 * time.Minute)},
	})
	if state.TimelineByRef[state.CurrentRef].scrollOffset != 5 {
		t.Fatalf("expected timeline scroll offset to stay after timeline update, got %d", state.TimelineByRef[state.CurrentRef].scrollOffset)
	}
}

func TestMouseWheelScrollsDetail(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusDetail
	state.CurrentRef = "o/r#1"
	state.DetailScroll = 10
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := strings.Repeat("line ", 400)
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now(),
		Comment:    &ghpr.CommentContext{Body: &body},
	})

	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	state, _ = Reduce(state, MouseWheelEvent{X: midW + 2, Y: 10, Delta: 1})

	if state.DetailScroll != 11 {
		t.Fatalf("expected detail scroll to move down by 1, got %d", state.DetailScroll)
	}
}

func TestMouseClickOnNotificationRowSelectsAndFocusesNotifications(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusNotifications
	state.NotifScroll = 0
	state.NotifLoading = false
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", updatedAt: time.Now(), title: "first"},
		{id: "n2", ref: "o/r#2", updatedAt: time.Now().Add(-time.Minute), title: "second"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0

	state, _ = Reduce(state, MouseClickEvent{X: 1, Y: 2, Button: mouseButtonLeft})

	if state.Focus != focusNotifications {
		t.Fatalf("expected focus notifications, got %v", state.Focus)
	}
	if state.SelectedNotif != "n2" {
		t.Fatalf("expected selected notification to be second row, got %q", state.SelectedNotif)
	}
	if state.CurrentRef != "o/r#2" {
		t.Fatalf("expected current ref to second notification, got %q", state.CurrentRef)
	}
}

func TestMouseClickOnTimelineRowSelectsAndFocusesTimeline(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusNotifications
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	state.CurrentRef = "o/r#1"
	first := "first"
	second := "second"
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now().Add(-time.Minute),
		Comment:    &ghpr.CommentContext{Body: &first},
	})
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e2",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now(),
		Comment:    &ghpr.CommentContext{Body: &second},
	})

	mode := state.currentPaneMode()
	leftW, _, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	state, _ = Reduce(state, MouseClickEvent{X: leftW + 2, Y: 1, Button: mouseButtonLeft})

	if state.Focus != focusTimeline {
		t.Fatalf("expected focus timeline, got %v", state.Focus)
	}
	if state.DetailScroll != 0 {
		t.Fatalf("expected detail scroll reset, got %d", state.DetailScroll)
	}
	ts = state.TimelineByRef[state.CurrentRef]
	if ts.selectedID != eventRowID("e2") {
		t.Fatalf("expected timeline selection to change, got %q", ts.selectedID)
	}
}

func TestMouseClickOnDetailPaneFocusesDetail(t *testing.T) {
	state := NewState()
	state.Width = 90
	state.Height = 20
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.NotifScroll = 0
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
		forcePushByID:   map[string]forcePushDiffState{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	detail := "first"
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now(),
		Comment:    &ghpr.CommentContext{Body: &detail},
	})

	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	state, _ = Reduce(state, MouseClickEvent{X: midW + 2, Y: 1, Button: mouseButtonLeft})

	if state.Focus != focusDetail {
		t.Fatalf("expected focus detail, got %v", state.Focus)
	}
}

func TestMouseClickOnNotificationTabSelectsTab(t *testing.T) {
	state := NewState()
	state.Width = 100
	state.Height = 20
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", repo: "lun-energy/integrations", ref: "lun-energy/integrations#1"},
		{id: "n2", repo: "godotengine/godot", ref: "godotengine/godot#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "lun-energy/integrations#1"
	state.NotifTab = allNotificationsTab

	tabX := notifTabX(state, 1)
	state, _ = Reduce(state, MouseClickEvent{X: tabX, Y: 0, Button: mouseButtonLeft})

	if state.NotifTab != "lun-energy" {
		t.Fatalf("expected lun-energy tab, got %q", state.NotifTab)
	}
	if state.CurrentRef != "lun-energy/integrations#1" {
		t.Fatalf("expected current ref to stay on selected tab row, got %q", state.CurrentRef)
	}
}

func notifTabX(state AppState, index int) int {
	tabs := state.notificationTabs()
	if index < 0 || index >= len(tabs) {
		return 0
	}
	offset := 0
	for i, tab := range tabs {
		labelWidth := len(" " + tab + " ")
		if i == index {
			return offset + 1
		}
		offset += labelWidth + 1
	}
	return 0
}

func TestTabCyclesNotificationOrgTabs(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", repo: "lun-energy/integrations", ref: "lun-energy/integrations#1"},
		{id: "n2", repo: "godotengine/godot", ref: "godotengine/godot#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "lun-energy/integrations#1"
	state.NotifTab = allNotificationsTab

	next, effects := Reduce(state, KeyEvent{Key: "tab"})
	if next.NotifTab != "lun-energy" {
		t.Fatalf("expected tab to cycle to lun-energy, got %q", next.NotifTab)
	}
	if next.SelectedNotif != "n1" {
		t.Fatalf("expected selection to remain n1 in lun-energy tab, got %q", next.SelectedNotif)
	}
	if len(effects) != 0 {
		t.Fatalf("expected no timeline effects when ref unchanged, got %d", len(effects))
	}

	next, effects = Reduce(next, KeyEvent{Key: "tab"})
	if next.NotifTab != "godotengine" {
		t.Fatalf("expected tab to cycle to godotengine, got %q", next.NotifTab)
	}
	if next.SelectedNotif != "n2" {
		t.Fatalf("expected selection to move to first notification in tab, got %q", next.SelectedNotif)
	}
	if next.CurrentRef != "godotengine/godot#2" {
		t.Fatalf("expected current ref to follow selected tab item, got %q", next.CurrentRef)
	}
	if len(effects) < 2 {
		t.Fatalf("expected timeline reload effects after ref change, got %d", len(effects))
	}

	next, effects = Reduce(next, KeyEvent{Key: "shift+tab"})
	if next.NotifTab != "lun-energy" {
		t.Fatalf("expected shift+tab to cycle back to lun-energy, got %q", next.NotifTab)
	}
	if next.SelectedNotif != "n1" {
		t.Fatalf("expected selection to move back to lun-energy item, got %q", next.SelectedNotif)
	}
	if next.CurrentRef != "lun-energy/integrations#1" {
		t.Fatalf("expected current ref to follow reverse tab selection, got %q", next.CurrentRef)
	}
	if len(effects) < 2 {
		t.Fatalf("expected timeline reload effects after reverse tab ref change, got %d", len(effects))
	}
}

func TestTabIgnoredOutsideRootView(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", repo: "lun-energy/integrations", ref: "lun-energy/integrations#1"},
		{id: "n2", repo: "godotengine/godot", ref: "godotengine/godot#2"},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "lun-energy/integrations#1"
	state.NotifTab = allNotificationsTab
	state.Focus = focusTimeline

	next, effects := Reduce(state, KeyEvent{Key: "tab"})
	if next.NotifTab != allNotificationsTab {
		t.Fatalf("expected tab ignored outside root view, got tab %q", next.NotifTab)
	}
	if len(effects) != 0 {
		t.Fatalf("expected no effects when tab is ignored, got %d", len(effects))
	}
}

func TestToggleReadOnThreadHeaderTogglesAllChildren(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{},
		readKnownByEventID: map[string]bool{},
		readLoadInFlight:   map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	threadID := "t1"
	root := "root"
	reply := "reply"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &root}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &reply}})
	ts.selectedID = threadHeaderID(threadID)

	next, effects := Reduce(state, KeyEvent{Key: "r"})

	if !next.TimelineByRef[next.CurrentRef].readByEventID["c1"] || !next.TimelineByRef[next.CurrentRef].readByEventID["c2"] {
		t.Fatalf("expected thread children to be marked read")
	}
	foundPersist := false
	for _, eff := range effects {
		p, ok := eff.(PersistReadStateEffect)
		if !ok {
			continue
		}
		foundPersist = true
		if !p.Read || len(p.EventIDs) != 2 {
			t.Fatalf("unexpected persist payload: %+v", p)
		}
	}
	if !foundPersist {
		t.Fatalf("expected PersistReadStateEffect")
	}
}

func TestToggleReadFromNotificationsTogglesAllTimelineChildren(t *testing.T) {
	tests := []struct {
		name       string
		readByID   map[string]bool
		expectRead bool
	}{
		{
			name:       "all unread become read",
			readByID:   map[string]bool{"e1": false, "c1": false, "c2": false},
			expectRead: true,
		},
		{
			name:       "all read become unread",
			readByID:   map[string]bool{"e1": true, "c1": true, "c2": true},
			expectRead: false,
		},
		{
			name:       "mixed become read",
			readByID:   map[string]bool{"e1": true, "c1": false, "c2": true},
			expectRead: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := NewState()
			state.Focus = focusNotifications
			state.Notifications = []notifRow{{id: "n1", repo: "o/r", title: "n", ref: "o/r#1"}}
			state.rebuildNotifIndex()
			state.SelectedNotif = "n1"
			state.NotifSelected = 0
			state.CurrentRef = "o/r#1"
			state.TimelineByRef[state.CurrentRef] = &timelineState{
				ref:                state.CurrentRef,
				rowIndexByID:       map[string]int{},
				threadByID:         map[string]*threadGroup{},
				expandedThreads:    map[string]bool{},
				readByEventID:      map[string]bool{},
				readKnownByEventID: map[string]bool{},
				readLoadInFlight:   map[string]bool{},
			}
			ts := state.TimelineByRef[state.CurrentRef]
			threadID := "t1"
			body := "body"
			ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
			ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body}})
			ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(2 * time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body}})
			for id, read := range tc.readByID {
				ts.readByEventID[id] = read
				ts.readKnownByEventID[id] = true
			}

			next, effects := Reduce(state, KeyEvent{Key: "r"})

			nextTS := next.TimelineByRef[next.CurrentRef]
			for _, id := range []string{"e1", "c1", "c2"} {
				if nextTS.readByEventID[id] != tc.expectRead {
					t.Fatalf("expected %s read=%t, got %t", id, tc.expectRead, nextTS.readByEventID[id])
				}
			}

			foundPersist := false
			for _, eff := range effects {
				p, ok := eff.(PersistReadStateEffect)
				if !ok {
					continue
				}
				foundPersist = true
				if p.Read != tc.expectRead {
					t.Fatalf("expected persist read=%t, got %t", tc.expectRead, p.Read)
				}
				if len(p.EventIDs) != 3 {
					t.Fatalf("expected 3 persisted ids, got %d", len(p.EventIDs))
				}
				seen := map[string]bool{}
				for _, id := range p.EventIDs {
					seen[id] = true
				}
				for _, id := range []string{"e1", "c1", "c2"} {
					if !seen[id] {
						t.Fatalf("expected persisted ids to contain %s", id)
					}
				}
			}
			if !foundPersist {
				t.Fatalf("expected PersistReadStateEffect")
			}
		})
	}
}

func TestToggleReadFromNotificationsAdvancesToNextNotification(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", repo: "o/r", title: "first", ref: "o/r#1", updatedAt: time.Now()},
		{id: "n2", repo: "o/r", title: "second", ref: "o/r#2", updatedAt: time.Now().Add(-time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	state.TimelineByRef["o/r#1"] = &timelineState{
		ref:                "o/r#1",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	state.TimelineByRef["o/r#2"] = &timelineState{
		ref:                "o/r#2",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e2": false},
		readKnownByEventID: map[string]bool{"e2": true},
		readLoadInFlight:   map[string]bool{},
	}
	body := "body"
	state.TimelineByRef["o/r#1"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
	state.TimelineByRef["o/r#2"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	next, effects := Reduce(state, KeyEvent{Key: "r"})

	if !next.TimelineByRef["o/r#1"].readByEventID["e1"] {
		t.Fatalf("expected selected notification timeline marked read")
	}
	if next.SelectedNotif != "n2" {
		t.Fatalf("expected selection to advance to n2, got %q", next.SelectedNotif)
	}
	if next.CurrentRef != "o/r#2" {
		t.Fatalf("expected current ref to advance to o/r#2, got %q", next.CurrentRef)
	}

	foundPersist := false
	for _, eff := range effects {
		switch e := eff.(type) {
		case PersistReadStateEffect:
			foundPersist = true
			if e.Ref != "o/r#1" || len(e.EventIDs) != 1 || e.EventIDs[0] != "e1" || !e.Read {
				t.Fatalf("unexpected persist payload: %+v", e)
			}
		}
	}
	if !foundPersist {
		t.Fatalf("expected PersistReadStateEffect")
	}
}

func TestToggleReadFromNotificationsContinuesLoadingAndMarksLateChildrenRead(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", repo: "o/r", title: "first", ref: "o/r#1", updatedAt: time.Now()},
		{id: "n2", repo: "o/r", title: "second", ref: "o/r#2", updatedAt: time.Now().Add(-time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1

	state.TimelineByRef["o/r#1"] = &timelineState{
		ref:                "o/r#1",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
		done:               false,
		loading:            true,
	}
	body := "body"
	state.TimelineByRef["o/r#1"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	next, effects := Reduce(state, KeyEvent{Key: "r"})

	if next.SelectedNotif != "n2" {
		t.Fatalf("expected selection to advance to n2, got %q", next.SelectedNotif)
	}
	if next.ReadThroughRef != "o/r#1" {
		t.Fatalf("expected read-through ref to stay on first notification, got %q", next.ReadThroughRef)
	}
	if next.TimelineLoadingRef != "o/r#1" {
		t.Fatalf("expected timeline loader to remain on first notification, got %q", next.TimelineLoadingRef)
	}
	for _, eff := range effects {
		if start, ok := eff.(StartTimelineEffect); ok && start.Ref == "o/r#2" {
			t.Fatalf("did not expect selection change to start next notification timeline while read-through is active")
		}
	}

	gen := next.TimelineGen
	next, _ = Reduce(next, TimelineArrivedEvent{Generation: gen, Ref: "o/r#1", Event: ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{Body: &body}}})
	if !next.TimelineByRef["o/r#1"].readByEventID["e2"] || !next.TimelineByRef["o/r#1"].readKnownByEventID["e2"] {
		t.Fatalf("expected late-arriving child event to be marked read")
	}

	next, doneEffects := Reduce(next, TimelineDoneEvent{Generation: gen, Ref: "o/r#1"})
	if next.ReadThroughRef != "" {
		t.Fatalf("expected read-through to finish after timeline done")
	}
	foundPersistLate := false
	foundResumeCurrent := false
	for _, eff := range doneEffects {
		switch e := eff.(type) {
		case PersistReadStateEffect:
			if e.Ref == "o/r#1" {
				for _, id := range e.EventIDs {
					if id == "e2" {
						foundPersistLate = true
						break
					}
				}
			}
		case StartTimelineEffect:
			if e.Ref == "o/r#2" {
				foundResumeCurrent = true
			}
		}
	}
	if !foundPersistLate {
		t.Fatalf("expected read-through to persist late-arriving child event")
	}
	if !foundResumeCurrent {
		t.Fatalf("expected selected notification timeline load to resume after read-through")
	}
}

func TestRootMarkReadSetsParentOverrideImmediately(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{{id: "n1", repo: "o/r", title: "first", ref: "o/r#1", updatedAt: time.Now()}}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1", rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, done: false}

	next, effects := Reduce(state, KeyEvent{Key: "r"})

	if !next.ParentReadByRef["o/r#1"] {
		t.Fatalf("expected parent read override to be set immediately")
	}
	foundPersistParent := false
	for _, eff := range effects {
		if p, ok := eff.(PersistParentReadStateEffect); ok {
			if p.Ref == "o/r#1" && p.Read {
				foundPersistParent = true
			}
		}
	}
	if !foundPersistParent {
		t.Fatalf("expected PersistParentReadStateEffect for root mark-read")
	}
}

func TestChildUnreadClearsParentOverride(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.ParentReadByRef["o/r#1"] = true
	state.ParentReadLoadedByRef["o/r#1"] = true
	state.TimelineByRef["o/r#1"] = &timelineState{
		ref:                "o/r#1",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": true},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
		done:               true,
	}
	body := "body"
	state.TimelineByRef["o/r#1"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
	state.TimelineByRef["o/r#1"].selectedID = eventRowID("e1")

	next, effects := Reduce(state, KeyEvent{Key: "r"})

	if next.ParentReadByRef["o/r#1"] {
		t.Fatalf("expected child unread toggle to clear parent read override")
	}
	foundPersistParentUnread := false
	for _, eff := range effects {
		if p, ok := eff.(PersistParentReadStateEffect); ok {
			if p.Ref == "o/r#1" && !p.Read {
				foundPersistParentUnread = true
			}
		}
	}
	if !foundPersistParentUnread {
		t.Fatalf("expected PersistParentReadStateEffect unread when child toggled unread")
	}
}

func TestOpenKeyFromNotificationsUsesTargetURL(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{{id: "n1", repo: "o/r", title: "title", kind: "pr", ref: "o/r#42", updatedAt: time.Now().UTC()}}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#42"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	body := "body"
	state.TimelineByRef[state.CurrentRef].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	next, effects := Reduce(state, KeyEvent{Key: "o"})

	if next.Status != "opened in browser" {
		t.Fatalf("expected open status, got %q", next.Status)
	}
	if !next.TimelineByRef[state.CurrentRef].readByEventID["e1"] {
		t.Fatalf("expected selected notification timeline to be marked read")
	}

	foundOpen := false
	foundPersist := false
	for _, eff := range effects {
		switch e := eff.(type) {
		case OpenURLEffect:
			foundOpen = true
			if e.URL != "https://github.com/o/r/pull/42" {
				t.Fatalf("unexpected open URL %q", e.URL)
			}
		case PersistReadStateEffect:
			foundPersist = true
			if e.Ref != "o/r#42" || !e.Read || len(e.EventIDs) != 1 || e.EventIDs[0] != "e1" {
				t.Fatalf("unexpected persist payload: %+v", e)
			}
		}
	}
	if !foundOpen {
		t.Fatalf("expected OpenURLEffect")
	}
	if !foundPersist {
		t.Fatalf("expected PersistReadStateEffect")
	}
}

func TestOpenKeyOnThreadHeaderUsesThreadURL(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	threadID := "t1"
	body := "root"
	url := "https://github.com/o/r/pull/1#discussion_r77"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &body, URL: &url}})
	ts.selectedID = threadHeaderID(threadID)

	next, effects := Reduce(state, KeyEvent{Key: "o"})

	if next.Status != "opened in browser" {
		t.Fatalf("expected open status, got %q", next.Status)
	}
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	openEff, ok := effects[0].(OpenURLEffect)
	if !ok {
		t.Fatalf("expected OpenURLEffect, got %T", effects[0])
	}
	if openEff.URL != url {
		t.Fatalf("expected thread URL %q, got %q", url, openEff.URL)
	}
}

func TestOpenKeyFallsBackToCurrentRefURL(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#7"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	issue := ghpr.TimelineEvent{ID: "e0", Type: "issue.opened", OccurredAt: time.Now().UTC(), Issue: &ghpr.IssueOpenedData{Title: "issue"}}
	ts.insertTimelineEvent(issue)
	ts.selectedID = eventRowID("e0")

	next, effects := Reduce(state, KeyEvent{Key: "o"})

	if next.Status != "opened in browser" {
		t.Fatalf("expected open status, got %q", next.Status)
	}
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	openEff, ok := effects[0].(OpenURLEffect)
	if !ok {
		t.Fatalf("expected OpenURLEffect, got %T", effects[0])
	}
	if openEff.URL != "https://github.com/o/r/issues/7" {
		t.Fatalf("expected issue URL fallback, got %q", openEff.URL)
	}
}

func TestHideReadFiltersTimelineAndThreadPane(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": true},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	threadID := "t2"
	root := "root"
	reply := "reply"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &root}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c1", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &root}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "c2", Type: "github.review_comment", OccurredAt: time.Now().UTC().Add(2 * time.Minute), Comment: &ghpr.CommentContext{ThreadID: &threadID, Body: &reply}})
	ts.readByEventID["c1"] = true
	ts.readByEventID["c2"] = true

	next, _ := Reduce(state, KeyEvent{Key: "H"})
	if !next.HideRead {
		t.Fatalf("expected hide-read enabled")
	}
	rows := next.TimelineByRef[next.CurrentRef].displayRows(next.HideRead)
	if len(rows) != 0 {
		t.Fatalf("expected all-read timeline rows hidden, got %d", len(rows))
	}

	next.Focus = focusThread
	next.TimelineByRef[next.CurrentRef].activeThreadID = threadID
	threadRows := next.TimelineByRef[next.CurrentRef].threadRows(threadID, next.HideRead)
	if len(threadRows) != 0 {
		t.Fatalf("expected all-read thread rows hidden, got %d", len(threadRows))
	}
}

func TestHideReadFiltersReadNotificationsInRootPane(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.NotifLoading = false
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", repo: "o/r", title: "read", updatedAt: time.Now()},
		{id: "n2", ref: "o/r#2", repo: "o/r", title: "unread", updatedAt: time.Now().Add(-time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	state.TimelineByRef["o/r#1"] = &timelineState{
		ref:                "o/r#1",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": true},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
		done:               true,
	}
	state.TimelineByRef["o/r#2"] = &timelineState{
		ref:                "o/r#2",
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e2": false},
		readKnownByEventID: map[string]bool{"e2": true},
		readLoadInFlight:   map[string]bool{},
		done:               true,
	}
	body := "body"
	state.TimelineByRef["o/r#1"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
	state.TimelineByRef["o/r#2"].insertTimelineEvent(ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	next, _ := Reduce(state, KeyEvent{Key: "shift+h"})

	if !next.HideRead {
		t.Fatalf("expected hide-read enabled")
	}
	visible := next.visibleNotifications()
	if len(visible) != 1 || visible[0].id != "n2" {
		t.Fatalf("expected only unread notification visible, got %+v", visible)
	}
	if next.SelectedNotif != "n2" {
		t.Fatalf("expected selection to move to first visible notification, got %q", next.SelectedNotif)
	}
}

func TestShiftHTogglesHideReadFromRootPane(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications

	next, _ := Reduce(state, KeyEvent{Key: "shift+h"})
	if !next.HideRead {
		t.Fatalf("expected shift+h to toggle hide-read on")
	}

	next, _ = Reduce(next, KeyEvent{Key: "shift+h"})
	if next.HideRead {
		t.Fatalf("expected shift+h to toggle hide-read off")
	}
}

func TestTimelineRowsStayHiddenWhileReadStateLoads(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineGen = 1
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{},
		readKnownByEventID: map[string]bool{},
		readLoadInFlight:   map[string]bool{},
	}

	body := "hello"
	next, effects := Reduce(state, TimelineArrivedEvent{
		Generation: state.TimelineGen,
		Ref:        state.CurrentRef,
		Event: ghpr.TimelineEvent{
			ID:         "e1",
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().UTC(),
			Comment:    &ghpr.CommentContext{Body: &body},
		},
	})

	ts := next.TimelineByRef[next.CurrentRef]
	rows := ts.rowsReadyForDisplay(ts.displayRows(false))
	if len(rows) != 0 {
		t.Fatalf("expected timeline row hidden until read state is loaded, got %d", len(rows))
	}

	hasLoadEffect := false
	for _, effect := range effects {
		if _, ok := effect.(LoadReadStateEffect); ok {
			hasLoadEffect = true
			break
		}
	}
	if !hasLoadEffect {
		t.Fatalf("expected read-state load effect")
	}

	next, _ = Reduce(next, ReadStateLoadedEvent{Ref: next.CurrentRef, EventIDs: []string{"e1"}, ReadIDs: nil})
	ts = next.TimelineByRef[next.CurrentRef]
	rows = ts.rowsReadyForDisplay(ts.displayRows(false))
	if len(rows) != 1 {
		t.Fatalf("expected timeline row to appear after read state load, got %d", len(rows))
	}
}

func TestPersistReadFailureRollsBackOptimisticState(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := "b"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
	ts.selectedID = eventRowID("e1")

	next, _ := Reduce(state, KeyEvent{Key: "r"})
	if !next.TimelineByRef[next.CurrentRef].readByEventID["e1"] {
		t.Fatalf("expected optimistic read state")
	}
	if len(next.PendingRead) != 1 {
		t.Fatalf("expected one pending read op")
	}
	var opID int64
	for id := range next.PendingRead {
		opID = id
	}

	next, _ = Reduce(next, ReadStatePersistFailedEvent{OpID: opID, Err: "boom"})
	if next.TimelineByRef[next.CurrentRef].readByEventID["e1"] {
		t.Fatalf("expected read state rollback after failure")
	}
}

func TestNotificationUnreadMarkerUsesCacheWhileReadStateUnknown(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	n := notifRow{id: "n1", repo: "o/r", ref: state.CurrentRef, title: "t"}
	state.Notifications = []notifRow{n}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{},
		readKnownByEventID: map[string]bool{},
		readLoadInFlight:   map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := "b"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	if got := state.notificationUnreadMarker(n); got != " ●  " {
		t.Fatalf("expected unread marker while unknown, got %q", got)
	}

	ts.readKnownByEventID["e1"] = true
	ts.readByEventID["e1"] = false
	if got := state.notificationUnreadMarker(n); got != " ●  " {
		t.Fatalf("expected unread marker once known, got %q", got)
	}

	ts.readKnownByEventID["e1"] = false
	if got := state.notificationUnreadMarker(n); got != " ●  " {
		t.Fatalf("expected cached unread marker while unknown again, got %q", got)
	}
}

func TestMotionCountPrefixMovesMultipleRows(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = make([]notifRow, 0, 30)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("n%d", i)
		state.Notifications = append(state.Notifications, notifRow{id: id, repo: "o/r", ref: fmt.Sprintf("o/r#%d", i), updatedAt: time.Now().UTC().Add(-time.Duration(i) * time.Minute)})
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = state.Notifications[0].id
	state.NotifSelected = 0

	state, _ = Reduce(state, KeyEvent{Key: "1"})
	state, _ = Reduce(state, KeyEvent{Key: "0"})
	state, _ = Reduce(state, KeyEvent{Key: "j"})

	visible := state.visibleNotifications()
	idx := indexOfNotificationByID(visible, state.SelectedNotif)
	if idx != 10 {
		t.Fatalf("expected selection to move down 10 rows, got %d", idx)
	}

	state, _ = Reduce(state, KeyEvent{Key: "1"})
	state, _ = Reduce(state, KeyEvent{Key: "5"})
	state, _ = Reduce(state, KeyEvent{Key: "k"})
	visible = state.visibleNotifications()
	idx = indexOfNotificationByID(visible, state.SelectedNotif)
	if idx != 0 {
		t.Fatalf("expected selection to clamp at top, got %d", idx)
	}
}

func TestBracketKeysJumpToTopAndBottomOfNotificationsPane(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", repo: "o/r", ref: "o/r#1", updatedAt: time.Now().UTC()},
		{id: "n2", repo: "o/r", ref: "o/r#2", updatedAt: time.Now().UTC().Add(-time.Minute)},
		{id: "n3", repo: "o/r", ref: "o/r#3", updatedAt: time.Now().UTC().Add(-2 * time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"

	state, _ = Reduce(state, KeyEvent{Key: "]"})
	if state.SelectedNotif != "n3" {
		t.Fatalf("expected ] to jump to last notification, got %q", state.SelectedNotif)
	}
	if state.CurrentRef != "o/r#3" {
		t.Fatalf("expected current ref to follow selection, got %q", state.CurrentRef)
	}

	state, _ = Reduce(state, KeyEvent{Key: "["})
	if state.SelectedNotif != "n1" {
		t.Fatalf("expected [ to jump to first notification, got %q", state.SelectedNotif)
	}
	if state.CurrentRef != "o/r#1" {
		t.Fatalf("expected current ref to follow selection, got %q", state.CurrentRef)
	}
}

func TestBracketKeysJumpToTopAndBottomOfDetailPane(t *testing.T) {
	state := NewState()
	state.Width = 80
	state.Height = 12
	state.Focus = focusDetail
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := strings.Repeat("long detail line ", 120)
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})
	ts.selectedID = eventRowID("e1")

	next, _ := Reduce(state, KeyEvent{Key: "]"})
	if next.DetailScroll <= 0 {
		t.Fatalf("expected ] to jump to bottom of detail pane, got %d", next.DetailScroll)
	}

	next, _ = Reduce(next, KeyEvent{Key: "["})
	if next.DetailScroll != 0 {
		t.Fatalf("expected [ to jump to top of detail pane, got %d", next.DetailScroll)
	}
}

func TestCtrlRStartsRefreshCurrentRefFirst(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#2"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1"}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2"}
	state.TimelineByRef["o/r#3"] = &timelineState{ref: "o/r#3"}

	next, effects := Reduce(state, KeyEvent{Key: "ctrl+r"})

	if !next.RefreshInFlight {
		t.Fatalf("expected refresh to start")
	}
	if next.RefreshActiveRef != "o/r#2" {
		t.Fatalf("expected current ref refreshed first, got %q", next.RefreshActiveRef)
	}
	if next.TimelineLoadingRef != "o/r#2" {
		t.Fatalf("expected timeline loading ref set, got %q", next.TimelineLoadingRef)
	}
	if len(next.RefreshQueue) != 2 || next.RefreshQueue[0] != "o/r#1" || next.RefreshQueue[1] != "o/r#3" {
		t.Fatalf("unexpected refresh queue order: %+v", next.RefreshQueue)
	}

	foundStart := false
	foundSpinner := false
	for _, eff := range effects {
		switch e := eff.(type) {
		case StartTimelineEffect:
			if e.Ref == "o/r#2" {
				foundStart = true
			}
		case ScheduleRefreshSpinnerTickEffect:
			foundSpinner = true
		}
	}
	if !foundStart {
		t.Fatalf("expected timeline start effect for current ref")
	}
	if !foundSpinner {
		t.Fatalf("expected spinner tick scheduling effect")
	}
}

func TestSelectionDuringRefreshDoesNotInterruptRefreshLoader(t *testing.T) {
	state := NewState()
	state.Focus = focusNotifications
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", repo: "o/r", title: "one", updatedAt: time.Now()},
		{id: "n2", ref: "o/r#2", repo: "o/r", title: "two", updatedAt: time.Now().Add(-time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n1"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef["o/r#1"] = &timelineState{ref: "o/r#1"}
	state.TimelineByRef["o/r#2"] = &timelineState{ref: "o/r#2"}

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+r"})
	beforeGen := next.TimelineGen
	beforeLoadingRef := next.TimelineLoadingRef

	afterMove, effects := Reduce(next, KeyEvent{Key: "j"})

	if afterMove.SelectedNotif != "n2" {
		t.Fatalf("expected selection to move to n2, got %q", afterMove.SelectedNotif)
	}
	if afterMove.TimelineGen != beforeGen {
		t.Fatalf("expected navigation during refresh not to change timeline generation")
	}
	if afterMove.TimelineLoadingRef != beforeLoadingRef {
		t.Fatalf("expected refresh loader ownership to stay on %q, got %q", beforeLoadingRef, afterMove.TimelineLoadingRef)
	}
	for _, eff := range effects {
		switch eff.(type) {
		case CancelTimelineEffect:
			t.Fatalf("unexpected cancel timeline effect during refresh navigation")
		case StartTimelineEffect:
			t.Fatalf("unexpected start timeline effect during refresh navigation")
		}
	}

	afterDone, doneEffects := Reduce(afterMove, TimelineDoneEvent{Generation: beforeGen, Ref: "o/r#1"})
	if !afterDone.RefreshInFlight {
		t.Fatalf("expected refresh to continue after first timeline finishes")
	}
	if afterDone.RefreshActiveRef != "o/r#2" {
		t.Fatalf("expected refresh to continue with o/r#2, got %q", afterDone.RefreshActiveRef)
	}
	if afterDone.TimelineLoadingRef != "o/r#2" {
		t.Fatalf("expected timeline loading ref to move to o/r#2, got %q", afterDone.TimelineLoadingRef)
	}
	foundNextStart := false
	for _, eff := range doneEffects {
		if start, ok := eff.(StartTimelineEffect); ok && start.Ref == "o/r#2" {
			foundNextStart = true
			break
		}
	}
	if !foundNextStart {
		t.Fatalf("expected next timeline refresh start effect")
	}
}

func TestRefreshTimelineFallsBackToClosestSelectionWhenMissing(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{},
		readKnownByEventID: map[string]bool{"e1": true, "e2": true, "e3": true},
	}
	ts := state.TimelineByRef[state.CurrentRef]
	body := "b"
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().Add(-time.Minute), Comment: &ghpr.CommentContext{Body: &body}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.commented", OccurredAt: time.Now(), Comment: &ghpr.CommentContext{Body: &body}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e3", Type: "github.timeline.commented", OccurredAt: time.Now().Add(time.Minute), Comment: &ghpr.CommentContext{Body: &body}})
	ts.selectedID = eventRowID("e2")
	ts.selectedIndex = 1
	next, _ := Reduce(state, KeyEvent{Key: "ctrl+r"})
	gen := next.TimelineGen

	next, _ = Reduce(next, TimelineArrivedEvent{Generation: gen, Ref: state.CurrentRef, Event: ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().Add(-time.Minute), Comment: &ghpr.CommentContext{Body: &body}}})
	next, _ = Reduce(next, TimelineArrivedEvent{Generation: gen, Ref: state.CurrentRef, Event: ghpr.TimelineEvent{ID: "e3", Type: "github.timeline.commented", OccurredAt: time.Now().Add(time.Minute), Comment: &ghpr.CommentContext{Body: &body}}})
	next, _ = Reduce(next, TimelineDoneEvent{Generation: gen, Ref: state.CurrentRef})

	got := next.TimelineByRef[state.CurrentRef].selectedID
	if got != eventRowID("e3") {
		t.Fatalf("expected closest fallback selection e3, got %q", got)
	}
}

func TestRefreshNotificationsFallsBackToClosestSelectionWhenMissing(t *testing.T) {
	state := NewState()
	state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", repo: "o/r", title: "one", updatedAt: time.Now()},
		{id: "n2", ref: "o/r#2", repo: "o/r", title: "two", updatedAt: time.Now().Add(-time.Minute)},
		{id: "n3", ref: "o/r#3", repo: "o/r", title: "three", updatedAt: time.Now().Add(-2 * time.Minute)},
	}
	state.rebuildNotifIndex()
	state.SelectedNotif = "n2"
	state.NotifSelected = 1
	state.CurrentRef = "o/r#2"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef}

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+r"})
	gen := next.TimelineGen
	next, _ = Reduce(next, TimelineDoneEvent{Generation: gen, Ref: state.CurrentRef})
	nGen := next.NotifGen

	next, _ = Reduce(next, NotificationsArrivedEvent{Generation: nGen, Item: ghpr.NotificationEvent{ID: "n1", UpdatedAt: time.Now(), Repository: ghpr.NotificationRepository{Owner: "o", Repo: "r"}, Subject: ghpr.NotificationSubject{Title: "one"}, Target: ghpr.NotificationTarget{Kind: "issue", Ref: "o/r#1"}}})
	next, _ = Reduce(next, NotificationsArrivedEvent{Generation: nGen, Item: ghpr.NotificationEvent{ID: "n3", UpdatedAt: time.Now().Add(-time.Minute), Repository: ghpr.NotificationRepository{Owner: "o", Repo: "r"}, Subject: ghpr.NotificationSubject{Title: "three"}, Target: ghpr.NotificationTarget{Kind: "issue", Ref: "o/r#3"}}})
	next, _ = Reduce(next, NotificationsDoneEvent{Generation: nGen})

	if next.SelectedNotif != "n3" {
		t.Fatalf("expected closest fallback notification n3, got %q", next.SelectedNotif)
	}
}

func TestTimelineWarnSkipsMapperWarningNoise(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineLoadingRef = state.CurrentRef
	state.TimelineGen = 1
	state.Status = "existing"

	next, _ := Reduce(state, TimelineWarnEvent{
		Generation: state.TimelineGen,
		Ref:        state.CurrentRef,
		Message:    "warning: skipping unknown timeline event type=\"convert_to_draft\" id=1 occurred_at=2026-02-13T20:06:15Z",
	})

	if next.Status != "existing" {
		t.Fatalf("expected mapper warning to be suppressed, got %q", next.Status)
	}
}

func TestAutoRefreshTickDoesNotQueueWhileRefreshInFlight(t *testing.T) {
	state := NewState()
	state.RefreshInFlight = true
	state.RefreshStage = "timeline"
	state.RefreshPending = false

	next, effects := Reduce(state, AutoRefreshTickEvent{})

	if next.RefreshPending {
		t.Fatalf("expected auto-refresh tick not to queue another refresh")
	}
	foundSchedule := false
	for _, eff := range effects {
		if _, ok := eff.(ScheduleAutoRefreshTickEffect); ok {
			foundSchedule = true
			break
		}
	}
	if !foundSchedule {
		t.Fatalf("expected auto-refresh schedule effect")
	}
}

func TestFinishRefreshDoesNotSetRefreshedStatus(t *testing.T) {
	state := NewState()
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{ref: state.CurrentRef}

	next, _ := Reduce(state, KeyEvent{Key: "ctrl+r"})
	gen := next.TimelineGen
	next, _ = Reduce(next, TimelineDoneEvent{Generation: gen, Ref: state.CurrentRef})
	nGen := next.NotifGen
	next, _ = Reduce(next, NotificationsDoneEvent{Generation: nGen})

	if next.Status == "refreshed" {
		t.Fatalf("expected no refreshed status text")
	}
}

func TestArchiveKeyOpensConfirmFromNonNotificationPanes(t *testing.T) {
	for _, focus := range []focusColumn{focusTimeline, focusThread, focusDetail} {
		t.Run(fmt.Sprintf("focus=%d", focus), func(t *testing.T) {
			state := NewState()
			state.Focus = focus
			state.Notifications = []notifRow{{id: "42", repo: "o/r", title: "title", ref: "o/r#1", updatedAt: time.Now().UTC()}}
			state.rebuildNotifIndex()
			state.SelectedNotif = "42"
			state.NotifSelected = 0
			state.CurrentRef = "o/r#1"

			next, effects := Reduce(state, KeyEvent{Key: "a"})

			if !next.ArchiveConfirmOpen {
				t.Fatalf("expected archive confirm to open")
			}
			if next.ArchiveConfirmThreadID != "42" {
				t.Fatalf("expected target thread id 42, got %q", next.ArchiveConfirmThreadID)
			}
			if len(effects) != 0 {
				t.Fatalf("expected no side effects before confirmation, got %d", len(effects))
			}
		})
	}
}

func TestArchiveConfirmMarksReadAndRemovesNotificationOnSuccess(t *testing.T) {
	state := NewState()
	state.Focus = focusTimeline
	state.Notifications = []notifRow{{id: "42", repo: "o/r", title: "title", ref: "o/r#1", updatedAt: time.Now().UTC()}}
	state.rebuildNotifIndex()
	state.SelectedNotif = "42"
	state.NotifSelected = 0
	state.CurrentRef = "o/r#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:                state.CurrentRef,
		rowIndexByID:       map[string]int{},
		threadByID:         map[string]*threadGroup{},
		expandedThreads:    map[string]bool{},
		readByEventID:      map[string]bool{"e1": false},
		readKnownByEventID: map[string]bool{"e1": true},
		readLoadInFlight:   map[string]bool{},
	}
	body := "body"
	state.TimelineByRef[state.CurrentRef].insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}})

	next, _ := Reduce(state, KeyEvent{Key: "a"})
	next, effects := Reduce(next, KeyEvent{Key: "a"})

	if next.ArchiveConfirmOpen {
		t.Fatalf("expected archive confirm to close after confirmation")
	}
	foundArchive := false
	archiveOpID := int64(0)
	foundPersistRead := false
	for _, eff := range effects {
		switch e := eff.(type) {
		case ArchiveNotificationEffect:
			foundArchive = true
			archiveOpID = e.OpID
			if e.ThreadID != "42" {
				t.Fatalf("expected archive thread id 42, got %q", e.ThreadID)
			}
		case PersistReadStateEffect:
			if e.Ref == "o/r#1" && e.Read {
				foundPersistRead = true
			}
		}
	}
	if !foundArchive {
		t.Fatalf("expected ArchiveNotificationEffect")
	}
	if !foundPersistRead {
		t.Fatalf("expected PersistReadStateEffect")
	}

	next, _ = Reduce(next, ArchiveNotificationSucceededEvent{OpID: archiveOpID})
	if len(next.Notifications) != 0 {
		t.Fatalf("expected notification removed after archive success")
	}
	if next.Focus != focusNotifications {
		t.Fatalf("expected focus to return to notifications, got %v", next.Focus)
	}
}
