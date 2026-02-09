package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestViewFitsWithinReportedTerminalHeight(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 90
	m.state.Height = 24

	out := m.View()
	lines := strings.Count(out, "\n") + 1
	if lines > m.state.Height {
		t.Fatalf("expected view height <= %d lines, got %d", m.state.Height, lines)
	}
}

func TestViewFitsWithLongWrappedNotificationContent(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 80
	m.state.Height = 20
	m.state.Notifications = []notifRow{
		{
			id:        "n1",
			updatedAt: time.Now().UTC(),
			repo:      "owner/repo",
			title:     "This is a very long notification title that would previously wrap into multiple rows and push the TUI beyond terminal bounds",
		},
	}
	m.state.rebuildNotifIndex()
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0

	out := m.View()
	lines := strings.Count(out, "\n") + 1
	if lines > m.state.Height {
		t.Fatalf("expected wrapped view height <= %d lines, got %d", m.state.Height, lines)
	}
}

func TestPaneWidthsTrackFocusedColumn(t *testing.T) {
	total := 120
	nL, nM, nR := paneWidths(total, focusNotifications, paneModeNotificationsTimeline)
	if nR != 0 {
		t.Fatalf("expected detail pane hidden in notifications focus, got %d/%d/%d", nL, nM, nR)
	}
	if nL <= 0 || nM <= 0 {
		t.Fatalf("expected notifications and timeline panes visible, got %d/%d/%d", nL, nM, nR)
	}

	tL, tM, tR := paneWidths(total, focusTimeline, paneModeTimelineDetail)
	if tL != 0 {
		t.Fatalf("expected notifications pane hidden in timeline focus, got %d/%d/%d", tL, tM, tR)
	}
	if tM <= 0 || tR <= 0 {
		t.Fatalf("expected timeline and detail panes visible, got %d/%d/%d", tL, tM, tR)
	}

	dL, dM, dR := paneWidths(total, focusDetail, paneModeTimelineDetail)
	if dL != 0 {
		t.Fatalf("expected notifications pane hidden in detail focus, got %d/%d/%d", dL, dM, dR)
	}
	if dM <= 0 || dR <= 0 {
		t.Fatalf("expected timeline and detail panes visible, got %d/%d/%d", dL, dM, dR)
	}

	if nL+nM+nR != total || tL+tM+tR != total || dL+dM+dR != total {
		t.Fatalf("expected pane widths to sum to terminal width")
	}
}

func TestPaneWidthsShowExactlyTwoColumns(t *testing.T) {
	l, m, r := paneWidths(42, focusTimeline, paneModeTimelineDetail)
	if l != 0 || m < 1 || r < 1 {
		t.Fatalf("expected only timeline/detail visible, got %d/%d/%d", l, m, r)
	}
}

type loopModel struct {
	inner *model
}

func (l *loopModel) Init() tea.Cmd {
	return nil
}

func (l *loopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := l.inner.Update(msg)
	return l, cmd
}

func (l *loopModel) View() string {
	return l.inner.View()
}

func TestTeaProgramLoopChangesFocusedPaneWidth(t *testing.T) {
	base := newModel(context.Background(), nil)
	base.state.Width = 96
	base.state.Height = 24
	base.state.Notifications = []notifRow{{id: "n1", ref: "o/r#1", title: "one"}}
	base.state.rebuildNotifIndex()
	base.state.SelectedNotif = "n1"
	base.state.NotifSelected = 0
	base.state.CurrentRef = "o/r#1"

	wrapped := &loopModel{inner: base}
	p := tea.NewProgram(wrapped, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not exit in time")
	}

	if wrapped.inner.state.Focus != focusTimeline {
		t.Fatalf("expected focus timeline after key navigation, got %v", wrapped.inner.state.Focus)
	}

	mode := wrapped.inner.state.currentPaneMode()
	l, m, r := paneWidths(wrapped.inner.state.Width, wrapped.inner.state.Focus, mode)
	if l != 0 || m <= r {
		t.Fatalf("expected timeline/detail layout with wider timeline pane, got %d/%d/%d", l, m, r)
	}
}

func TestTeaProgramLoopMouseClickSelectsTimeline(t *testing.T) {
	base := newModel(context.Background(), nil)
	base.state.Width = 96
	base.state.Height = 24
	base.state.NotifLoading = false
	base.state.NotifDone = true
	base.state.Notifications = []notifRow{
		{id: "n1", ref: "o/r#1", title: "first"},
		{id: "n2", ref: "o/r#2", title: "second"},
	}
	base.state.rebuildNotifIndex()
	base.state.SelectedNotif = "n1"
	base.state.NotifSelected = 0
	base.state.CurrentRef = "o/r#1"
	base.state.TimelineByRef[base.state.CurrentRef] = &timelineState{
		ref:             base.state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
	}
	ts := base.state.TimelineByRef[base.state.CurrentRef]
	body := "first"
	ts.insertTimelineEvent(ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now(),
		Comment:    &ghpr.CommentContext{Body: &body},
	})

	wrapped := &loopModel{inner: base}
	p := tea.NewProgram(wrapped, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	mode := base.state.currentPaneMode()
	leftW, _, _ := paneWidths(panesTotalWidth(base.state.Width, base.state.Focus, mode), base.state.Focus, mode)
	p.Send(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: leftW + 2, Y: 0})
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not exit in time")
	}

	if wrapped.inner.state.Focus != focusTimeline {
		t.Fatalf("expected focus timeline after mouse click, got %v", wrapped.inner.state.Focus)
	}
}

func TestTeaProgramLoopJKKeepsTimelineSelectionVisible(t *testing.T) {
	base := newModel(context.Background(), nil)
	base.state.Width = 60
	base.state.Height = 8
	base.state.Focus = focusTimeline
	base.state.CurrentRef = "o/r#1"
	base.state.TimelineByRef[base.state.CurrentRef] = &timelineState{
		ref:             base.state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
	}
	ts := base.state.TimelineByRef[base.state.CurrentRef]
	long := strings.Repeat("wrapped timeline row content ", 30)
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("e%d", i)
		body := fmt.Sprintf("marker-%d %s", i, long)
		ts.insertTimelineEvent(ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &body},
		})
	}
	ts.selectedID = eventRowID("e0")

	wrapped := &loopModel{inner: base}
	p := tea.NewProgram(wrapped, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())

	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	for i := 0; i < 8; i++ {
		p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("program run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not exit in time")
	}

	finalTS := wrapped.inner.state.currentTimeline()
	if finalTS == nil {
		t.Fatalf("expected timeline state")
	}
	if finalTS.selectedIndex < 8 {
		t.Fatalf("expected j navigation to move selection, got %d", finalTS.selectedIndex)
	}
	if finalTS.scrollOffset <= 0 {
		t.Fatalf("expected j navigation to advance timeline scroll, got %d", finalTS.scrollOffset)
	}
	mode := wrapped.inner.state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(wrapped.inner.state.Width, wrapped.inner.state.Focus, mode), wrapped.inner.state.Focus, mode)
	out := wrapped.inner.renderTimeline(midW, paneInnerHeight(wrapped.inner.state))
	if !strings.Contains(out, "marker-8") {
		t.Fatalf("expected selected marker visible after j navigation, got %q", out)
	}
}

func TestViewLinesDoNotExceedTerminalWidth(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 90
	m.state.Height = 20
	m.state.Focus = focusTimeline
	m.state.Notifications = []notifRow{{
		id:        "n1",
		updatedAt: time.Now().UTC(),
		repo:      "owner/repo",
		title:     "This is an intentionally long notification title to stress selected row rendering width",
		ref:       "owner/repo#1",
	}}
	m.state.rebuildNotifIndex()
	m.state.NotifLoading = false
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts
	body := "A very long timeline comment body that should be truncated before selection marker styling to prevent visual spill into adjacent columns"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}}
	ts.insertTimelineEvent(ev)

	out := m.View()
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > m.state.Width {
			t.Fatalf("line %d width %d exceeds terminal width %d", i+1, w, m.state.Width)
		}
	}
}

func TestViewLinesDoNotOverflowWithWideUnicode(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 72
	m.state.Height = 18
	m.state.Focus = focusDetail
	m.state.Notifications = []notifRow{{
		id:        "n1",
		updatedAt: time.Now().UTC(),
		repo:      "owner/repo",
		title:     "unicode width check",
		ref:       "owner/repo#1",
	}}
	m.state.rebuildNotifIndex()
	m.state.NotifLoading = false
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts
	body := "<summary>📝 Walkthrough</summary> ✅" + strings.Repeat(" 文字", 12)
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}}
	ts.insertTimelineEvent(ev)

	out := m.View()
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > m.state.Width {
			t.Fatalf("line %d width %d exceeds terminal width %d", i+1, w, m.state.Width)
		}
	}
}

func TestViewSanitizesCarriageReturnAndTabs(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 88
	m.state.Height = 20
	m.state.Focus = focusDetail
	m.state.Notifications = []notifRow{{
		id:        "n1",
		updatedAt: time.Now().UTC(),
		repo:      "owner/repo",
		title:     "render sanitize",
		ref:       "owner/repo#1",
	}}
	m.state.rebuildNotifIndex()
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts
	body := "Line A\rReposting from #493\twith tab"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}}
	ts.insertTimelineEvent(ev)

	out := m.View()
	if strings.ContainsRune(out, '\r') {
		t.Fatal("expected rendered output to not contain carriage returns")
	}
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > m.state.Width {
			t.Fatalf("line %d width %d exceeds terminal width %d", i+1, w, m.state.Width)
		}
	}
}

func TestWrapDisplayWidthUsesHangingIndent(t *testing.T) {
	got := wrapDisplayWidth("1h  repo/name  some wrapped title", 14, "    ")
	if len(got) < 2 {
		t.Fatalf("expected wrapped lines, got %v", got)
	}
	if !strings.HasPrefix(got[1], "    ") {
		t.Fatalf("expected continuation line to use hanging indent, got %q", got[1])
	}
}

func TestWrapDisplayWidthPrefersLogicalBreaks(t *testing.T) {
	got := wrapDisplayWidth("alpha beta-gamma/delta", 11, "")
	if len(got) < 2 {
		t.Fatalf("expected wrapped lines, got %v", got)
	}
	if strings.HasSuffix(got[0], "a") && strings.HasPrefix(got[1], "mma") {
		t.Fatalf("expected logical break point, got %v", got)
	}
}

func TestNotificationTimePrefixesAlignAcrossRows(t *testing.T) {
	now := time.Now()
	rows := []notifRow{
		{updatedAt: now.Add(-8 * time.Hour)},
		{updatedAt: now.Add(-12 * time.Hour)},
	}
	w := notificationTimeColumnWidth(rows)
	p1 := padToDisplayWidth(timeAgo(rows[0].updatedAt), w) + " "
	p2 := padToDisplayWidth(timeAgo(rows[1].updatedAt), w) + " "

	if lipgloss.Width(p1) != lipgloss.Width(p2) {
		t.Fatalf("expected aligned prefixes, got %q (%d) and %q (%d)", p1, lipgloss.Width(p1), p2, lipgloss.Width(p2))
	}
}

func TestRenderNotificationTimestampStylesPrefix(t *testing.T) {
	m := newModel(context.Background(), nil)
	line := "2h owner/repo  title"
	got := renderNotificationTimestamp(line, 3, m.styles.muted)

	if !strings.Contains(got, m.styles.muted.Render("2h ")) {
		t.Fatalf("expected muted timestamp prefix, got %q", got)
	}
	if !strings.Contains(got, "owner/repo  title") {
		t.Fatalf("expected remainder of line unchanged, got %q", got)
	}
}

func TestWrapNotificationsAlignsWrappedTitleUnderTitleColumn(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 86
	m.state.Height = 12
	m.state.Focus = focusNotifications
	m.state.Notifications = []notifRow{{
		id:        "n1",
		updatedAt: time.Now().Add(-time.Hour).UTC(),
		repo:      "lun-energy/room-by-room-ios",
		title:     "Fix TextEditor not expanding for pre-populated note descriptions and preserve line wrapping alignment",
		ref:       "lun-energy/room-by-room-ios#1",
	}}
	m.state.rebuildNotifIndex()
	m.state.NotifLoading = false
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0

	mode := m.state.currentPaneMode()
	leftW, _, _ := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	out := m.renderNotifications(leftW, paneInnerHeight(m.state))
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped notification to span multiple lines, got %q", out)
	}
	firstIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "Fix TextEditor") {
			firstIdx = i
			break
		}
	}
	if firstIdx < 0 || firstIdx+1 >= len(lines) {
		t.Fatalf("expected wrapped title lines, got %q", out)
	}

	first := strings.TrimRight(lines[firstIdx], " ")
	second := strings.TrimRight(lines[firstIdx+1], " ")
	firstTitleIdx := strings.Index(first, "Fix TextEditor")
	if firstTitleIdx < 0 {
		t.Fatalf("expected first line to contain title start, got %q", first)
	}
	if len(second) < firstTitleIdx {
		t.Fatalf("expected wrapped second line to reach title column, got %q", second)
	}
	if !strings.HasPrefix(second, strings.Repeat(" ", firstTitleIdx)) {
		t.Fatalf("expected wrapped second line to align under title column, got %q", second)
	}
}

func TestWrapTimelineRowThreadHeaderHasNoArrowIndent(t *testing.T) {
	ts := &timelineState{expandedThreads: map[string]bool{"tid": true}}
	row := displayTimelineRow{
		id:             threadHeaderID("tid"),
		threadID:       "tid",
		isThreadHeader: true,
		label:          "RoomByRoom/../RoomOverview/RoomViewModel.swift (1 comments)",
	}

	lines := wrapTimelineRow(row, ts, 26, 9, 10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped header lines, got %v", lines)
	}
	if strings.HasPrefix(lines[1], "  ") {
		t.Fatalf("expected continuation line without arrow indent, got %q", lines[1])
	}
}

func TestWrapThreadRowUsesHangingIndentForReplies(t *testing.T) {
	threadID := "tid"
	ts := &timelineState{
		expandedThreads: map[string]bool{},
		threadByID: map[string]*threadGroup{
			threadID: &threadGroup{
				id: threadID,
				items: []ghpr.TimelineEvent{
					{Actor: &ghpr.Actor{Login: "KaffeDiem"}},
					{Actor: &ghpr.Actor{Login: "Copilot"}},
				},
			},
		},
	}
	row := displayTimelineRow{
		id:       threadChildID(threadID, "c1"),
		threadID: threadID,
		label:    "KaffeDiem  This triggers a FB write every time",
		event: &ghpr.TimelineEvent{
			Type:       "github.review_comment",
			OccurredAt: time.Now().UTC(),
			Actor:      &ghpr.Actor{Login: "KaffeDiem"},
			Comment:    &ghpr.CommentContext{Body: ptrBody("This triggers a FB write every time")},
		},
	}

	lines := wrapThreadRow(row, ts, 38, 10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped child lines, got %v", lines)
	}
	expectedIndent := strings.Repeat(" ", 9+2)
	if !strings.HasPrefix(lines[1], expectedIndent) {
		t.Fatalf("expected continuation line to align under message column, got %q", lines[1])
	}
}

func TestWrapThreadRowDoesNotUseTreeRail(t *testing.T) {
	threadID := "tid"
	ts := &timelineState{
		expandedThreads: map[string]bool{},
		threadByID: map[string]*threadGroup{
			threadID: {
				id: threadID,
				items: []ghpr.TimelineEvent{
					{Actor: &ghpr.Actor{Login: "KaffeDiem"}},
					{Actor: &ghpr.Actor{Login: "Copilot"}},
				},
			},
		},
	}
	row := displayTimelineRow{
		id:       threadChildID(threadID, "c1"),
		threadID: threadID,
		label:    "KaffeDiem  This triggers a FB write every time",
		event: &ghpr.TimelineEvent{
			Type:       "github.review_comment",
			OccurredAt: time.Now().UTC(),
			Actor:      &ghpr.Actor{Login: "KaffeDiem"},
			Comment:    &ghpr.CommentContext{Body: ptrBody("This triggers a FB write every time")},
		},
	}

	lines := wrapThreadRow(row, ts, 38, 10)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped child lines, got %v", lines)
	}
	if strings.HasPrefix(lines[1], "  │  ") {
		t.Fatalf("expected no tree rail in thread pane wrap, got %q", lines[1])
	}
}

func ptrBody(s string) *string {
	return &s
}

func TestFormatTimelineColumnsAlignsKindColumn(t *testing.T) {
	a, _ := formatTimelineColumns(10, 10, "opened", "KaffeDiem", "Auto detect")
	b, _ := formatTimelineColumns(10, 10, "requested", "KaffeDiem", "")

	if !strings.HasPrefix(a, "opened    ") {
		t.Fatalf("expected padded kind column, got %q", a)
	}
	if !strings.HasPrefix(b, "requested ") {
		t.Fatalf("expected padded kind column, got %q", b)
	}
}

func TestFormatTimelineColumnsAlignsMessageColumn(t *testing.T) {
	first, offset := formatTimelineColumns(8, 10, "opened", "davidpdrsn", "NG-2918 long message")
	if offset <= 0 {
		t.Fatalf("expected positive message offset, got %d", offset)
	}
	if !strings.HasPrefix(first, "opened  ") {
		t.Fatalf("expected kind column prefix, got %q", first)
	}

	wrapped := wrapDisplayWidth(first, 34, strings.Repeat(" ", offset))
	if len(wrapped) < 2 {
		t.Fatalf("expected wrapped lines, got %v", wrapped)
	}
	if !strings.HasPrefix(wrapped[1], strings.Repeat(" ", offset)) {
		t.Fatalf("expected continuation to align under message column, got %q", wrapped[1])
	}
}

func TestTimelineViewportRemainsReadableWithManyLongEvents(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 92
	m.state.Height = 18
	m.state.Focus = focusTimeline
	m.state.Notifications = []notifRow{{id: "n1", ref: "owner/repo#1", title: "one"}}
	m.state.rebuildNotifIndex()
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts

	for i := 0; i < 40; i++ {
		body := "Long body " + strings.Repeat("segment ", 25) + "END_MARKER_DO_NOT_SHOW"
		ev := ghpr.TimelineEvent{
			ID:         "e" + string(rune('a'+(i%26))) + string(rune('a'+((i/26)%26))) + string(rune('a'+((i/676)%26))),
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().UTC().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &body},
		}
		ts.insertTimelineEvent(ev)
	}

	mode := m.state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	out := m.renderTimeline(midW, paneInnerHeight(m.state))

	lines := strings.Split(out, "\n")
	if len(lines) > paneInnerHeight(m.state) {
		t.Fatalf("expected timeline to fit viewport height %d, got %d", paneInnerHeight(m.state), len(lines))
	}
	if strings.Contains(out, "END_MARKER_DO_NOT_SHOW") {
		t.Fatalf("expected compact timeline labels to hide full long bodies")
	}
}

func TestRenderTimelineKeepsSelectedRowVisibleWithStaleScrollOffset(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 100
	m.state.Height = 20
	m.state.Focus = focusTimeline
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts

	opened := ghpr.TimelineEvent{ID: "e1", Type: "pr.opened", OccurredAt: time.Date(2024, 1, 2, 3, 0, 0, 0, time.UTC), Pr: &ghpr.PROpenedData{Title: "Open"}, Actor: &ghpr.Actor{Login: "alice"}}
	committed := ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.committed", OccurredAt: time.Date(2024, 1, 2, 3, 1, 0, 0, time.UTC), Commit: &ghpr.CommitContext{SHA: ptrBody("06c410f8717c")}}
	mentioned := ghpr.TimelineEvent{ID: "e3", Type: "github.timeline.mentioned", OccurredAt: time.Date(2024, 1, 2, 3, 2, 0, 0, time.UTC), Actor: &ghpr.Actor{Login: "bob"}}

	ts.insertTimelineEvent(opened)
	ts.insertTimelineEvent(committed)
	ts.insertTimelineEvent(mentioned)
	ts.selectedID = eventRowID("e2")
	ts.scrollOffset = 2

	mode := m.state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	out := m.renderTimeline(midW, paneInnerHeight(m.state))
	if !strings.Contains(out, "committed") {
		t.Fatalf("expected selected committed row to remain visible, got %q", out)
	}
}

func TestRenderTimelineKeepsWrappedSelectedRowVisible(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 60
	m.state.Height = 8
	m.state.Focus = focusTimeline
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts

	long := strings.Repeat("long wrapped content ", 25)
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("e%d", i+1)
		body := long
		if i == 5 {
			body = "selected sentinel"
		}
		ev := ghpr.TimelineEvent{
			ID:         id,
			Type:       "github.timeline.commented",
			OccurredAt: time.Now().UTC().Add(time.Duration(i) * time.Minute),
			Comment:    &ghpr.CommentContext{Body: &body},
		}
		ts.insertTimelineEvent(ev)
	}
	ts.selectedID = eventRowID("e6")
	ts.selectedIndex = 5
	ensureTimelineSelectionVisible(&m.state, ts)

	mode := m.state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	out := m.renderTimeline(midW, paneInnerHeight(m.state))
	if !strings.Contains(out, "selected sentinel") {
		t.Fatalf("expected wrapped selected row to be visible, got %q", out)
	}
}

func TestDiffLineKind(t *testing.T) {
	if got := diffLineKind("+added line"); got != "add" {
		t.Fatalf("expected add kind, got %q", got)
	}
	if got := diffLineKind("-removed line"); got != "del" {
		t.Fatalf("expected del kind, got %q", got)
	}
	if got := diffLineKind("@@ -1,2 +1,2 @@"); got != "hunk" {
		t.Fatalf("expected hunk kind, got %q", got)
	}
	if got := diffLineKind("diff --git a/x b/x"); got != "header" {
		t.Fatalf("expected header kind, got %q", got)
	}
}

func TestShouldHighlightDetailDiffForDiffEventsOnly(t *testing.T) {
	state := NewState()
	state.Focus = focusDetail
	state.CurrentRef = "owner/repo#1"
	state.TimelineByRef[state.CurrentRef] = &timelineState{
		ref:             state.CurrentRef,
		rowIndexByID:    map[string]int{},
		threadByID:      map[string]*threadGroup{},
		expandedThreads: map[string]bool{},
		commitDiffByID:  map[string]commitDiffState{},
	}
	ts := state.TimelineByRef[state.CurrentRef]

	nonCommitted := ghpr.TimelineEvent{ID: "e1", Type: "pr.opened", OccurredAt: time.Now().UTC(), Pr: &ghpr.PROpenedData{Title: "x"}}
	ts.insertTimelineEvent(nonCommitted)
	ts.selectedID = eventRowID("e1")
	if shouldHighlightDetailDiff(state) {
		t.Fatalf("expected non-committed event to not enable diff highlighting")
	}

	sha := "abc"
	committed := ghpr.TimelineEvent{ID: "e2", Type: "github.timeline.committed", OccurredAt: time.Now().UTC(), Commit: &ghpr.CommitContext{SHA: &sha}}
	ts.insertTimelineEvent(committed)
	ts.selectedID = eventRowID("e2")
	if !shouldHighlightDetailDiff(state) {
		t.Fatalf("expected committed event to enable diff highlighting")
	}

	forcePushed := ghpr.TimelineEvent{ID: "e3", Type: "github.timeline.head_ref_force_pushed", OccurredAt: time.Now().UTC()}
	ts.insertTimelineEvent(forcePushed)
	ts.selectedID = eventRowID("e3")
	if !shouldHighlightDetailDiff(state) {
		t.Fatalf("expected force-pushed event to enable diff highlighting")
	}

	threadID := "t1"
	body := "root"
	hunk := "@@ -1 +1 @@\n-old\n+new"
	root := ghpr.TimelineEvent{
		ID:         "c1",
		Type:       "github.review_comment",
		OccurredAt: time.Now().UTC(),
		Comment:    &ghpr.CommentContext{ThreadID: &threadID, Body: &body, DiffHunk: &hunk},
	}
	reply := ghpr.TimelineEvent{
		ID:         "c2",
		Type:       "github.review_comment",
		OccurredAt: time.Now().UTC().Add(time.Minute),
		Comment:    &ghpr.CommentContext{ThreadID: &threadID, Body: &body},
	}
	ts.insertTimelineEvent(root)
	ts.insertTimelineEvent(reply)
	ts.selectedID = threadHeaderID(threadID)
	ts.activeThreadID = threadID
	ts.threadSelectedID = threadChildID(threadID, "c1")
	ts.threadSelectedIndex = 0
	if !shouldHighlightDetailDiff(state) {
		t.Fatalf("expected thread root diff hunk to enable diff highlighting")
	}

	ts.threadSelectedID = threadChildID(threadID, "c2")
	ts.threadSelectedIndex = 1
	if shouldHighlightDetailDiff(state) {
		t.Fatalf("expected thread reply without diff hunk to not enable diff highlighting")
	}
}

func TestViewLinesDoNotOverflowWithColoredCommitDiff(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 90
	m.state.Height = 22
	m.state.Focus = focusDetail
	m.state.Notifications = []notifRow{{
		id:        "n1",
		updatedAt: time.Now().UTC(),
		repo:      "owner/repo",
		title:     "commit diff overflow",
		ref:       "owner/repo#1",
	}}
	m.state.rebuildNotifIndex()
	m.state.SelectedNotif = "n1"
	m.state.NotifSelected = 0
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts

	sha := "06c410f8717c3c575e3e4312ecb4e580c31bdaed"
	url := "https://github.com/lun-energy/integrations/commit/" + sha
	diffURL := "https://api.github.com/repos/lun-energy/integrations/commits/" + sha
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.committed", OccurredAt: time.Now().UTC(), Commit: &ghpr.CommitContext{SHA: &sha, URL: &url}, DiffURL: &diffURL}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")
	ts.commitDiffByID["e1"] = commitDiffState{body: "diff --git a/pkg/sdk/handler.go b/pkg/sdk/handler.go\n+\t\t\"github.com/lun-energy/integrations/pkg/logging\""}

	out := m.View()
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > m.state.Width {
			t.Fatalf("line %d width %d exceeds terminal width %d", i+1, w, m.state.Width)
		}
	}
}

func TestRenderDetailRespectsDetailScrollOffset(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 90
	m.state.Height = 12
	m.state.Focus = focusDetail
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}, commitDiffByID: map[string]commitDiffState{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts
	body := "line one\nline two\nline three\nline four"
	ev := ghpr.TimelineEvent{ID: "e1", Type: "github.timeline.commented", OccurredAt: time.Now().UTC(), Comment: &ghpr.CommentContext{Body: &body}}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	mode := m.state.currentPaneMode()
	_, _, rightW := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	first := m.renderDetail(rightW, paneInnerHeight(m.state))
	m.state.DetailScroll = 1
	scrolled := m.renderDetail(rightW, paneInnerHeight(m.state))

	if first == scrolled {
		t.Fatalf("expected detail view to change after scroll")
	}
}

func TestRenderDetailHighlightsMentionsForCommentEvents(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.state.Width = 90
	m.state.Height = 12
	m.state.Focus = focusDetail
	m.state.CurrentRef = "owner/repo#1"
	ts := &timelineState{ref: m.state.CurrentRef, rowIndexByID: map[string]int{}, threadByID: map[string]*threadGroup{}, expandedThreads: map[string]bool{}}
	m.state.TimelineByRef[m.state.CurrentRef] = ts

	body := "please review this @alice"
	ev := ghpr.TimelineEvent{
		ID:         "e1",
		Type:       "github.timeline.commented",
		OccurredAt: time.Now().UTC(),
		Event:      ptrBody("commented"),
		Comment:    &ghpr.CommentContext{Body: &body},
	}
	ts.insertTimelineEvent(ev)
	ts.selectedID = eventRowID("e1")

	mode := m.state.currentPaneMode()
	_, _, rightW := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)
	out := m.renderDetail(rightW, paneInnerHeight(m.state))

	if !strings.Contains(out, "[@alice]") {
		t.Fatalf("expected highlighted mention in detail output, got %q", out)
	}
	if !strings.Contains(out, "@alice") {
		t.Fatalf("expected mention text in detail output, got %q", out)
	}
}
