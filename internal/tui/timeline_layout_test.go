package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gh-pr/ghpr"
)

func TestBuildTimelineViewportPlanNormalizesStaleOffset(t *testing.T) {
	state := NewState()
	state.Width = 100
	state.Height = 20
	state.Focus = focusTimeline
	state.CurrentRef = "owner/repo#1"
	ts := &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
	}
	state.TimelineByRef[state.CurrentRef] = ts

	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e1", Type: "pr.opened", OccurredAt: time.Now().Add(-time.Minute), Pr: &ghpr.PROpenedData{Title: "Open"}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.committed", OccurredAt: time.Now(), Commit: &ghpr.CommitContext{SHA: ptrBody("abc123")}})
	ts.insertTimelineEvent(ghpr.TimelineEvent{ID: "e3", Type: "github.timeline.mentioned", OccurredAt: time.Now().Add(time.Minute)})
	ts.selectedID = eventRowID("e2")
	ts.scrollOffset = 2

	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	plan := buildTimelineViewportPlan(ts, midW, timelineViewportRows(state), false, true)
	if plan.start >= 2 {
		t.Fatalf("expected stale start to normalize, got %d", plan.start)
	}
	foundSelected := false
	for _, row := range plan.rows {
		if row.selected {
			foundSelected = true
			break
		}
	}
	if !foundSelected {
		t.Fatalf("expected selected row to be present in viewport plan")
	}
}

func TestBuildTimelineViewportPlanKeepsWrappedSelectedVisible(t *testing.T) {
	state := NewState()
	state.Width = 60
	state.Height = 8
	state.Focus = focusTimeline
	state.CurrentRef = "owner/repo#1"
	ts := &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
	}
	state.TimelineByRef[state.CurrentRef] = ts

	long := strings.Repeat("wrapped content ", 40)
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("e%d", i+1)
		body := long
		if i == 6 {
			body = "selected-marker"
		}
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &body},
		})
	}
	ts.selectedID = eventRowID("e7")
	ts.scrollOffset = 0

	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	plan := buildTimelineViewportPlan(ts, midW, timelineViewportRows(state), false, true)
	if plan.start == 0 {
		t.Fatalf("expected start to advance for wrapped rows")
	}
	found := false
	for _, row := range plan.rows {
		if row.selected && strings.Contains(strings.Join(row.lines, "\n"), "selected-marker") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected selected wrapped row in viewport plan")
	}
}
