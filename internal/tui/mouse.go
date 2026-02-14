package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type mouseButton string

const (
	mouseButtonLeft mouseButton = "left"
)

type mousePane int

const (
	mousePaneNone mousePane = iota
	mousePaneNotifications
	mousePaneTimeline
	mousePaneThread
	mousePaneDetail
)

type mouseHitType int

const (
	mouseHitNone mouseHitType = iota
	mouseHitRow
	mouseHitTab
)

type mouseRect struct {
	x0 int
	x1 int
}

type mouseLayout struct {
	notifications mouseRect
	timeline      mouseRect
	detail        mouseRect
	height        int
}

type mouseHit struct {
	pane mousePane
	kind mouseHitType
	row  int
	tab  int
}

func mouseLayoutForState(state AppState) mouseLayout {
	mode := state.currentPaneMode()
	leftW, midW, rightW := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)

	layout := mouseLayout{
		height: paneInnerHeight(state),
	}

	switch mode {
	case paneModeNotificationsTimeline:
		layout.notifications = mouseRect{x0: 0, x1: leftW}
		layout.timeline = mouseRect{x0: leftW + 1, x1: leftW + 1 + midW}
	default:
		switch midPaneContent(mode) {
		case paneContentThread:
			layout.timeline = mouseRect{}
		default:
			layout.timeline = mouseRect{x0: 0, x1: midW}
		}
		switch rightPaneContent(mode) {
		case paneContentThread:
			layout.detail = mouseRect{}
		default:
			layout.detail = mouseRect{x0: midW + 1, x1: midW + 1 + rightW}
		}
		if midPaneContent(mode) == paneContentThread {
			layout.notifications = mouseRect{}
			layout.timeline = mouseRect{x0: 0, x1: midW}
		}
		if rightPaneContent(mode) == paneContentThread {
			layout.notifications = mouseRect{}
			layout.detail = mouseRect{x0: midW + 1, x1: midW + 1 + rightW}
		}
	}
	if mode == paneModeThreadDetail {
		// Reuse timeline rect slot for thread rows, detail stays detail.
		layout.timeline = mouseRect{x0: 0, x1: midW}
		layout.detail = mouseRect{x0: midW + 1, x1: midW + 1 + rightW}
	}
	if mode == paneModeTimelineThread {
		layout.timeline = mouseRect{x0: 0, x1: midW}
		layout.detail = mouseRect{x0: midW + 1, x1: midW + 1 + rightW}
	}
	if mode == paneModeTimelineDetail {
		layout.timeline = mouseRect{x0: 0, x1: midW}
		layout.detail = mouseRect{x0: midW + 1, x1: midW + 1 + rightW}
	}
	return layout
}

func (r mouseRect) contains(x int) bool {
	return x >= r.x0 && x < r.x1
}

func mouseHitFromCoordinates(state AppState, x, y int) mouseHit {
	layout := mouseLayoutForState(state)
	if x < 0 || y < 0 || y >= layout.height {
		return mouseHit{pane: mousePaneNone, kind: mouseHitNone, row: -1, tab: -1}
	}

	if layout.notifications.contains(x) {
		if y == 0 {
			if idx, ok := notificationTabAtX(state, x-layout.notifications.x0); ok {
				return mouseHit{pane: mousePaneNotifications, kind: mouseHitTab, tab: idx, row: -1}
			}
		}
		row := notificationRowAtY(state, y, layout.notifications.x1-layout.notifications.x0)
		return mouseHit{pane: mousePaneNotifications, kind: kindForMouseRow(row), row: row}
	}

	if layout.timeline.contains(x) {
		mode := state.currentPaneMode()
		if mode == paneModeThreadDetail {
			row := threadRowAtY(state, y, layout.timeline.x1-layout.timeline.x0)
			return mouseHit{pane: mousePaneThread, kind: kindForMouseRow(row), row: row}
		}
		row := timelineRowAtY(state, y, layout.timeline.x1-layout.timeline.x0)
		return mouseHit{pane: mousePaneTimeline, kind: kindForMouseRow(row), row: row}
	}

	if layout.detail.contains(x) {
		if state.currentPaneMode() == paneModeTimelineThread {
			row := threadRowAtY(state, y, layout.detail.x1-layout.detail.x0)
			return mouseHit{pane: mousePaneThread, kind: kindForMouseRow(row), row: row}
		}
		return mouseHit{pane: mousePaneDetail, kind: mouseHitNone, row: -1}
	}

	return mouseHit{pane: mousePaneNone, kind: mouseHitNone, row: -1, tab: -1}
}

func threadRowAtY(state AppState, y, width int) int {
	ts := state.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		return -1
	}
	rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
	if len(rows) == 0 || width <= 0 || y < 0 {
		return -1
	}
	avail := paneContentWidthWithRelativeNumbers(width, paneInnerHeight(state))
	if avail < 1 {
		avail = 1
	}
	actorWidth := timelineActorColumnWidth(rows)
	line := y
	start := ts.threadScrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(rows) {
		start = len(rows) - 1
	}
	for i := start; i < len(rows); i++ {
		h := len(wrapThreadRow(rows[i], ts, avail, actorWidth))
		if h < 1 {
			h = 1
		}
		if line < h {
			return i
		}
		line -= h
	}
	return -1
}

func kindForMouseRow(row int) mouseHitType {
	if row >= 0 {
		return mouseHitRow
	}
	return mouseHitNone
}

func notificationTabAtX(state AppState, x int) (int, bool) {
	if x < 0 {
		return 0, false
	}

	tabs := state.notificationTabs()
	offset := 0
	for i, tab := range tabs {
		label := " " + tab + " "
		start := offset
		end := start + lipgloss.Width(label)
		if x >= start && x < end {
			return i, true
		}
		offset = end + 1
	}

	return 0, false
}

func notificationRowAtY(state AppState, y, width int) int {
	visible := state.visibleNotifications()
	if len(visible) == 0 || width <= 0 {
		return -1
	}
	if y <= 0 {
		return -1
	}

	avail := paneContentWidthWithRelativeNumbers(width, paneInnerHeight(state))
	if avail < 1 {
		avail = 1
	}
	timeColWidth := notificationTimeColumnWidth(visible)
	repoColWidth := notificationRepoColumnWidth(visible)
	line := y - 1

	for i := state.NotifScroll; i < len(visible); i++ {
		n := visible[i]
		prefix := state.notificationUnreadMarker(n) + padToDisplayWidth(timeAgo(n.updatedAt), timeColWidth) + " "
		repo := padToDisplayWidth(clampDisplayWidth(oneLine(n.repo), repoColWidth), repoColWidth)
		label := prefix + repo + "  " + oneLine(n.title)
		titleIndent := strings.Repeat(" ", lipgloss.Width(prefix)+repoColWidth+2)
		wrapped := wrapDisplayWidth(label, avail, titleIndent)
		h := len(wrapped)
		if h < 1 {
			h = 1
		}
		if line < h {
			return i
		}
		line -= h
	}

	return -1
}

func timelineRowAtY(state AppState, y, width int) int {
	ts := state.currentTimeline()
	if ts == nil {
		return -1
	}
	rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
	if len(rows) == 0 || width <= 0 {
		return -1
	}
	if y < 0 {
		return -1
	}

	avail := paneContentWidthWithRelativeNumbers(width, paneInnerHeight(state))
	if avail < 1 {
		avail = 1
	}
	kindWidth := timelineKindColumnWidth(rows)
	timeWidth := timelineTimeColumnWidth(rows)
	actorWidth := timelineActorColumnWidth(rows)
	line := y

	if ts.loading {
		line--
	}
	if ts.err != "" {
		line--
	}
	if line < 0 {
		return -1
	}

	start := ts.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(rows) {
		start = len(rows) - 1
	}
	if start < 0 {
		start = 0
	}

	for i := start; i < len(rows); i++ {
		h := len(wrapTimelineRow(rows[i], ts, avail, timeWidth, kindWidth, actorWidth))
		if h < 1 {
			h = 1
		}
		if line < h {
			return i
		}
		line -= h
	}

	return -1
}

func clampAndSelectNotificationTab(state *AppState, effects *[]Effect, targetIdx int) {
	tabs := state.notificationTabs()
	if targetIdx < 0 || targetIdx >= len(tabs) {
		return
	}
	target := tabs[targetIdx]
	if target == state.NotifTab {
		return
	}

	state.NotifTab = target
	state.NotifScroll = 0

	visible := state.visibleNotifications()
	if len(visible) == 0 {
		state.SelectedNotif = ""
		state.NotifSelected = 0
		if state.CurrentRef != "" {
			state.CurrentRef = ""
			*effects = append(*effects, CancelTimelineEffect{})
		}
		return
	}

	targetID := state.SelectedNotif
	idx := indexOfNotificationByID(visible, targetID)
	if idx < 0 {
		idx = 0
		targetID = visible[0].id
	}

	state.NotifSelected = idx
	if targetID == state.SelectedNotif {
		return
	}
	selectNotificationByID(state, effects, targetID)
}
