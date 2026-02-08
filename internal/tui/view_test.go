package tui

import (
	"context"
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
	nL, nM, nR := paneWidths(total, focusNotifications)
	if nL <= nM || nL <= nR {
		t.Fatalf("expected notifications pane widest, got %d/%d/%d", nL, nM, nR)
	}

	tL, tM, tR := paneWidths(total, focusTimeline)
	if tM <= tL || tM <= tR {
		t.Fatalf("expected timeline pane widest, got %d/%d/%d", tL, tM, tR)
	}

	dL, dM, dR := paneWidths(total, focusDetail)
	if dR <= dL || dR <= dM {
		t.Fatalf("expected detail pane widest, got %d/%d/%d", dL, dM, dR)
	}

	if nL+nM+nR != total || tL+tM+tR != total || dL+dM+dR != total {
		t.Fatalf("expected pane widths to sum to terminal width")
	}
}

func TestPaneWidthsKeepSidePanesVisible(t *testing.T) {
	l, m, r := paneWidths(42, focusTimeline)
	if l < 1 || m < 1 || r < 1 {
		t.Fatalf("expected all panes visible, got %d/%d/%d", l, m, r)
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

	l, m, r := paneWidths(wrapped.inner.state.Width, wrapped.inner.state.Focus)
	if m <= l || m <= r {
		t.Fatalf("expected focused timeline pane to be widest after full loop, got %d/%d/%d", l, m, r)
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
