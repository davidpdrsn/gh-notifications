package tui

import (
	"gh-pr/ghpr"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Event interface{ isEvent() }

type InitEvent struct{}
type WindowSizeEvent struct{ Width, Height int }
type KeyEvent struct{ Key string }

type NotificationsArrivedEvent struct {
	Generation int
	Item       ghpr.NotificationEvent
}
type NotificationsDoneEvent struct{ Generation int }
type NotificationsErrEvent struct {
	Generation int
	Err        string
}

type TimelineArrivedEvent struct {
	Generation int
	Ref        string
	Event      ghpr.TimelineEvent
}
type TimelineWarnEvent struct {
	Generation int
	Ref        string
	Message    string
}
type TimelineDoneEvent struct {
	Generation int
	Ref        string
}
type TimelineErrEvent struct {
	Generation int
	Ref        string
	Err        string
}
type CommitDiffLoadedEvent struct {
	Ref     string
	EventID string
	Diff    string
}
type CommitDiffErrEvent struct {
	Ref     string
	EventID string
	Err     string
}
type ForcePushInterdiffLoadedEvent struct {
	Ref        string
	EventID    string
	BeforeSHA  string
	AfterSHA   string
	CompareURL string
	Diff       string
}
type ForcePushInterdiffErrEvent struct {
	Ref     string
	EventID string
	Err     string
}

type ClipboardCopiedEvent struct{ Column string }
type ClipboardCopyFailedEvent struct {
	Column string
	Err    string
}

type MouseClickEvent struct {
	X      int
	Y      int
	Button mouseButton
}

type MouseWheelEvent struct {
	X     int
	Y     int
	Delta int
}

func (InitEvent) isEvent()                 {}
func (WindowSizeEvent) isEvent()           {}
func (KeyEvent) isEvent()                  {}
func (NotificationsArrivedEvent) isEvent() {}
func (NotificationsDoneEvent) isEvent()    {}
func (NotificationsErrEvent) isEvent()     {}
func (TimelineArrivedEvent) isEvent()      {}
func (TimelineWarnEvent) isEvent()         {}
func (TimelineDoneEvent) isEvent()         {}
func (TimelineErrEvent) isEvent()          {}
func (CommitDiffLoadedEvent) isEvent()     {}
func (CommitDiffErrEvent) isEvent()        {}
func (ForcePushInterdiffLoadedEvent) isEvent() {
}
func (ForcePushInterdiffErrEvent) isEvent() {}
func (ClipboardCopiedEvent) isEvent()       {}
func (ClipboardCopyFailedEvent) isEvent()   {}
func (MouseClickEvent) isEvent()            {}
func (MouseWheelEvent) isEvent()            {}

type Effect interface{ isEffect() }

type StartNotificationsEffect struct{ Generation int }
type StartTimelineEffect struct {
	Generation int
	Ref        string
}
type CancelTimelineEffect struct{}
type StartCommitDiffEffect struct {
	Ref     string
	EventID string
	DiffURL string
}
type StartForcePushInterdiffEffect struct {
	Ref     string
	EventID string
}
type CopyColumnEffect struct {
	Column string
	Text   string
}

func (StartNotificationsEffect) isEffect() {}
func (StartTimelineEffect) isEffect()      {}
func (CancelTimelineEffect) isEffect()     {}
func (StartCommitDiffEffect) isEffect()    {}
func (StartForcePushInterdiffEffect) isEffect() {
}
func (CopyColumnEffect) isEffect() {}

func Reduce(state AppState, ev Event) (AppState, []Effect) {
	effects := make([]Effect, 0)

	switch e := ev.(type) {
	case InitEvent:
		effects = append(effects, StartNotificationsEffect{Generation: state.NotifGen})
	case WindowSizeEvent:
		state.Width = e.Width
		state.Height = e.Height
	case MouseWheelEvent:
		return handleMouseWheel(&state, e), effects
	case MouseClickEvent:
		return handleMouseClick(&state, &effects, e), effects
	case KeyEvent:
		switch e.Key {
		case "ctrl+c", "q":
			state.Quit = true
			effects = append(effects, CancelTimelineEffect{})
		case "tab":
			if state.Focus == focusNotifications {
				cycleNotificationTab(&state, &effects, +1)
			}
		case "shift+tab":
			if state.Focus == focusNotifications {
				cycleNotificationTab(&state, &effects, -1)
			}
		case "ctrl+n":
			state.DetailScroll++
		case "ctrl+p":
			if state.DetailScroll > 0 {
				state.DetailScroll--
			}
		case "ctrl+d":
			state.DetailScroll += 10
		case "ctrl+u":
			state.DetailScroll -= 10
			if state.DetailScroll < 0 {
				state.DetailScroll = 0
			}
		case "C", "shift+c":
			column, text := columnCopyText(state)
			if strings.TrimSpace(text) == "" {
				state.Status = "nothing to copy"
				break
			}
			effects = append(effects, CopyColumnEffect{Column: column, Text: text})
		case "down", "j":
			moveDown(&state, &effects)
		case "up", "k":
			moveUp(&state, &effects)
		case "right", "l", "enter":
			drillIn(&state)
		case "left", "h", "backspace":
			backOut(&state)
		}
	case NotificationsArrivedEvent:
		if e.Generation == state.NotifGen {
			refChanged := state.insertNotification(e.Item)
			if refChanged && state.CurrentRef != "" {
				state.TimelineGen++
				effects = append(effects,
					CancelTimelineEffect{},
					StartTimelineEffect{Generation: state.TimelineGen, Ref: state.CurrentRef},
				)
				ensureTimelineState(&state, state.CurrentRef)
				ts := state.TimelineByRef[state.CurrentRef]
				ts.loading = true
				ts.done = false
				ts.err = ""
			}
		}
	case NotificationsDoneEvent:
		if e.Generation == state.NotifGen {
			state.NotifLoading = false
			state.NotifDone = true
		}
	case NotificationsErrEvent:
		if e.Generation == state.NotifGen {
			state.NotifLoading = false
			state.NotifErr = e.Err
		}
	case TimelineArrivedEvent:
		if e.Generation == state.TimelineGen && e.Ref == state.CurrentRef {
			ts := state.TimelineByRef[e.Ref]
			if ts != nil {
				ts.insertTimelineEvent(e.Event)
			}
		}
	case TimelineWarnEvent:
		if e.Generation == state.TimelineGen && e.Ref == state.CurrentRef {
			state.Status = e.Message
		}
	case TimelineDoneEvent:
		if e.Generation == state.TimelineGen && e.Ref == state.CurrentRef {
			ts := state.TimelineByRef[e.Ref]
			if ts != nil {
				ts.loading = false
				ts.done = true
			}
		}
	case TimelineErrEvent:
		if e.Generation == state.TimelineGen && e.Ref == state.CurrentRef {
			ts := state.TimelineByRef[e.Ref]
			if ts != nil {
				ts.loading = false
				ts.err = e.Err
			}
		}
	case CommitDiffLoadedEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			if ts.commitDiffByID == nil {
				ts.commitDiffByID = make(map[string]commitDiffState)
			}
			ts.commitDiffByID[e.EventID] = commitDiffState{body: e.Diff}
		}
	case CommitDiffErrEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			if ts.commitDiffByID == nil {
				ts.commitDiffByID = make(map[string]commitDiffState)
			}
			ts.commitDiffByID[e.EventID] = commitDiffState{err: e.Err}
			if e.Ref == state.CurrentRef {
				state.Status = "failed to load commit diff"
			}
		}
	case ForcePushInterdiffLoadedEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			if ts.forcePushByID == nil {
				ts.forcePushByID = make(map[string]forcePushDiffState)
			}
			ts.forcePushByID[e.EventID] = forcePushDiffState{
				beforeSHA:  e.BeforeSHA,
				afterSHA:   e.AfterSHA,
				compareURL: e.CompareURL,
				body:       e.Diff,
			}
		}
	case ForcePushInterdiffErrEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			if ts.forcePushByID == nil {
				ts.forcePushByID = make(map[string]forcePushDiffState)
			}
			ts.forcePushByID[e.EventID] = forcePushDiffState{err: e.Err}
			if e.Ref == state.CurrentRef {
				state.Status = "failed to load force-push interdiff"
			}
		}
	case ClipboardCopiedEvent:
		state.Status = "copied " + e.Column + " column"
	case ClipboardCopyFailedEvent:
		state.Status = "copy failed (" + e.Column + "): " + e.Err
	}

	queueSelectedDetailDiff(&state, &effects)

	normalizeState(&state)

	return state, effects
}

func handleMouseWheel(state *AppState, e MouseWheelEvent) AppState {
	hit := mouseHitFromCoordinates(*state, e.X, e.Y)
	switch hit.pane {
	case mousePaneNotifications:
		state.NotifScroll += e.Delta
		clampNotificationScroll(state)
	case mousePaneTimeline:
		ts := state.currentTimeline()
		if ts != nil {
			ts.scrollOffset += e.Delta
			clampTimelineScroll(ts)
		}
	case mousePaneThread:
		ts := state.currentTimeline()
		if ts != nil && ts.activeThreadID != "" {
			ts.threadScrollOffset += e.Delta
			rows := ts.threadRows(ts.activeThreadID)
			maxScroll := len(rows) - 1
			if maxScroll < 0 {
				ts.threadScrollOffset = 0
			} else {
				if ts.threadScrollOffset < 0 {
					ts.threadScrollOffset = 0
				}
				if ts.threadScrollOffset > maxScroll {
					ts.threadScrollOffset = maxScroll
				}
			}
		}
	case mousePaneDetail:
		state.DetailScroll += e.Delta
		normalizeDetail(state)
	}
	return *state
}

func handleMouseClick(state *AppState, effects *[]Effect, event MouseClickEvent) AppState {
	if event.Button != mouseButtonLeft {
		return *state
	}

	hit := mouseHitFromCoordinates(*state, event.X, event.Y)
	switch hit.pane {
	case mousePaneDetail:
		state.Focus = focusDetail
	case mousePaneNotifications:
		switch hit.kind {
		case mouseHitTab:
			clampAndSelectNotificationTab(state, effects, hit.tab)
		case mouseHitRow:
			state.Focus = focusNotifications
			visible := state.visibleNotifications()
			if hit.row >= 0 && hit.row < len(visible) {
				selectNotificationByID(state, effects, visible[hit.row].id)
			}
		}
	case mousePaneTimeline:
		if hit.kind == mouseHitRow {
			state.Focus = focusTimeline
			ts := state.currentTimeline()
			if ts != nil {
				rows := ts.displayRows()
				if hit.row >= 0 && hit.row < len(rows) {
					ts.selectedID = rows[hit.row].id
					ts.selectedIndex = hit.row
					state.DetailScroll = 0
					ensureTimelineSelectionVisible(state, ts)
				}
			}
		}
	case mousePaneThread:
		if hit.kind == mouseHitRow {
			state.Focus = focusThread
			ts := state.currentTimeline()
			if ts == nil || ts.activeThreadID == "" {
				return *state
			}
			rows := ts.threadRows(ts.activeThreadID)
			if hit.row >= 0 && hit.row < len(rows) {
				ts.threadSelectedID = rows[hit.row].id
				ts.threadSelectedIndex = hit.row
				state.DetailScroll = 0
				ensureThreadSelectionVisible(state, ts)
			}
		}
	}
	return *state
}

func ensureTimelineState(state *AppState, ref string) {
	if _, ok := state.TimelineByRef[ref]; ok {
		return
	}
	state.TimelineByRef[ref] = &timelineState{
		ref:             ref,
		rowIndexByID:    make(map[string]int),
		threadByID:      make(map[string]*threadGroup),
		expandedThreads: make(map[string]bool),
		commitDiffByID:  make(map[string]commitDiffState),
		forcePushByID:   make(map[string]forcePushDiffState),
	}
}

func queueSelectedDetailDiff(state *AppState, effects *[]Effect) {
	ts := state.currentTimeline()
	if ts == nil {
		return
	}
	ev := state.selectedDetailEvent()
	if ev == nil {
		return
	}
	if ev.ID == "" {
		return
	}
	if ev.Type == "github.timeline.committed" {
		if ev.DiffURL == nil || strings.TrimSpace(*ev.DiffURL) == "" {
			return
		}
		if ts.commitDiffByID == nil {
			ts.commitDiffByID = make(map[string]commitDiffState)
		}
		existing, ok := ts.commitDiffByID[ev.ID]
		if ok && (existing.loading || existing.body != "" || existing.err != "") {
			return
		}
		ts.commitDiffByID[ev.ID] = commitDiffState{loading: true}
		*effects = append(*effects, StartCommitDiffEffect{Ref: state.CurrentRef, EventID: ev.ID, DiffURL: *ev.DiffURL})
		return
	}
	if ev.Type == "github.timeline.head_ref_force_pushed" {
		if ts.forcePushByID == nil {
			ts.forcePushByID = make(map[string]forcePushDiffState)
		}
		existing, ok := ts.forcePushByID[ev.ID]
		if ok && (existing.loading || existing.body != "" || existing.err != "") {
			return
		}
		ts.forcePushByID[ev.ID] = forcePushDiffState{loading: true}
		*effects = append(*effects, StartForcePushInterdiffEffect{Ref: state.CurrentRef, EventID: ev.ID})
		return
	}
}

func moveDown(state *AppState, effects *[]Effect) {
	switch state.Focus {
	case focusNotifications:
		visible := state.visibleNotifications()
		if len(visible) == 0 || state.SelectedNotif == "" {
			return
		}
		idx := indexOfNotificationByID(visible, state.SelectedNotif)
		if idx >= 0 && idx < len(visible)-1 {
			state.NotifSelected = idx + 1
			selectNotificationByID(state, effects, visible[idx+1].id)
		}
	case focusTimeline:
		ts := state.currentTimeline()
		if ts == nil {
			return
		}
		rows := ts.displayRows()
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < len(rows)-1 {
			ts.selectedID = rows[idx+1].id
			ts.selectedIndex = idx + 1
			ensureTimelineSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	case focusThread:
		ts := state.currentTimeline()
		if ts == nil || ts.activeThreadID == "" {
			return
		}
		rows := ts.threadRows(ts.activeThreadID)
		if len(rows) == 0 {
			return
		}
		idx := indexOfThreadSelection(rows, ts.threadSelectedID)
		if idx < 0 {
			idx = 0
		}
		if idx < len(rows)-1 {
			ts.threadSelectedID = rows[idx+1].id
			ts.threadSelectedIndex = idx + 1
			ensureThreadSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	case focusDetail:
		ts := state.currentTimeline()
		if ts == nil {
			return
		}
		if ts.activeThreadID != "" {
			rows := ts.threadRows(ts.activeThreadID)
			if len(rows) == 0 {
				return
			}
			idx := indexOfThreadSelection(rows, ts.threadSelectedID)
			if idx < 0 {
				idx = 0
			}
			if idx < len(rows)-1 {
				ts.threadSelectedID = rows[idx+1].id
				ts.threadSelectedIndex = idx + 1
				ensureThreadSelectionVisible(state, ts)
				state.DetailScroll = 0
			}
			return
		}
		rows := ts.displayRows()
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < len(rows)-1 {
			ts.selectedID = rows[idx+1].id
			ts.selectedIndex = idx + 1
			ensureTimelineSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	}
}

func moveUp(state *AppState, effects *[]Effect) {
	switch state.Focus {
	case focusNotifications:
		visible := state.visibleNotifications()
		if len(visible) == 0 || state.SelectedNotif == "" {
			return
		}
		idx := indexOfNotificationByID(visible, state.SelectedNotif)
		if idx > 0 {
			state.NotifSelected = idx - 1
			selectNotificationByID(state, effects, visible[idx-1].id)
		}
	case focusTimeline:
		ts := state.currentTimeline()
		if ts == nil {
			return
		}
		rows := ts.displayRows()
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx > 0 {
			ts.selectedID = rows[idx-1].id
			ts.selectedIndex = idx - 1
			ensureTimelineSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	case focusThread:
		ts := state.currentTimeline()
		if ts == nil || ts.activeThreadID == "" {
			return
		}
		rows := ts.threadRows(ts.activeThreadID)
		if len(rows) == 0 {
			return
		}
		idx := indexOfThreadSelection(rows, ts.threadSelectedID)
		if idx > 0 {
			ts.threadSelectedID = rows[idx-1].id
			ts.threadSelectedIndex = idx - 1
			ensureThreadSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	case focusDetail:
		ts := state.currentTimeline()
		if ts == nil {
			return
		}
		if ts.activeThreadID != "" {
			rows := ts.threadRows(ts.activeThreadID)
			if len(rows) == 0 {
				return
			}
			idx := indexOfThreadSelection(rows, ts.threadSelectedID)
			if idx > 0 {
				ts.threadSelectedID = rows[idx-1].id
				ts.threadSelectedIndex = idx - 1
				ensureThreadSelectionVisible(state, ts)
				state.DetailScroll = 0
			}
			return
		}
		rows := ts.displayRows()
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx > 0 {
			ts.selectedID = rows[idx-1].id
			ts.selectedIndex = idx - 1
			ensureTimelineSelectionVisible(state, ts)
			state.DetailScroll = 0
		}
	}
}

func drillIn(state *AppState) {
	switch state.Focus {
	case focusNotifications:
		state.Focus = focusTimeline
	case focusTimeline:
		ts := state.currentTimeline()
		if ts == nil {
			return
		}
		rows := ts.displayRows()
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < 0 || idx >= len(rows) {
			return
		}
		row := rows[idx]
		if row.isThreadHeader {
			ts.activeThreadID = row.threadID
			ts.threadScrollOffset = 0
			threadRows := ts.threadRows(row.threadID)
			if len(threadRows) > 0 {
				ts.threadSelectedID = threadRows[0].id
				ts.threadSelectedIndex = 0
			} else {
				ts.threadSelectedID = ""
				ts.threadSelectedIndex = 0
			}
			state.Focus = focusThread
			return
		}
		ts.activeThreadID = ""
		ts.threadSelectedID = ""
		state.Focus = focusDetail
	case focusThread:
		ts := state.currentTimeline()
		if ts == nil || ts.activeThreadID == "" {
			return
		}
		if len(ts.threadRows(ts.activeThreadID)) == 0 {
			return
		}
		state.Focus = focusDetail
	}
}

func backOut(state *AppState) {
	switch state.Focus {
	case focusDetail:
		ts := state.currentTimeline()
		if ts != nil && ts.activeThreadID != "" {
			state.Focus = focusThread
			return
		}
		state.Focus = focusTimeline
	case focusThread:
		ts := state.currentTimeline()
		if ts != nil {
			ts.activeThreadID = ""
			ts.threadSelectedID = ""
			ts.threadSelectedIndex = 0
			ts.threadScrollOffset = 0
		}
		state.Focus = focusTimeline
	case focusTimeline:
		ts := state.currentTimeline()
		if ts != nil && ts.activeThreadID != "" {
			ts.activeThreadID = ""
			ts.threadSelectedID = ""
			ts.threadSelectedIndex = 0
			ts.threadScrollOffset = 0
			return
		}
		state.Focus = focusNotifications
	}
}

func normalizeState(state *AppState) {
	normalizeNotifications(state)
	if ts := state.currentTimeline(); ts != nil {
		normalizeTimeline(state, ts)
		normalizeThread(state, ts)
	}
	normalizeDetail(state)
}

func normalizeNotifications(state *AppState) {
	visible := state.visibleNotifications()
	if len(visible) == 0 {
		state.SelectedNotif = ""
		state.NotifSelected = 0
		state.NotifScroll = 0
		return
	}
	if idx := indexOfNotificationByID(visible, state.SelectedNotif); idx >= 0 {
		state.NotifSelected = idx
	} else {
		state.SelectedNotif = ""
	}
	if state.SelectedNotif == "" {
		if state.NotifSelected < 0 {
			state.NotifSelected = 0
		}
		if state.NotifSelected >= len(visible) {
			state.NotifSelected = len(visible) - 1
		}
		state.SelectedNotif = visible[state.NotifSelected].id
	}
	state.setCurrentRefFromSelectedNotification()
	maxScroll := len(visible) - 1
	if maxScroll < 0 {
		state.NotifScroll = 0
		return
	}
	if state.NotifScroll < 0 {
		state.NotifScroll = 0
		return
	}
	if state.NotifScroll > maxScroll {
		state.NotifScroll = maxScroll
	}
}

func ensureNotificationSelectionVisible(state *AppState) {
	visible := state.visibleNotifications()
	viewport := notificationViewportRows(*state)
	mode := state.currentPaneMode()
	leftW, _, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	avail := contentWidth(leftW)
	if avail < 1 {
		avail = 1
	}
	timeColWidth := notificationTimeColumnWidth(visible)
	repoColWidth := notificationRepoColumnWidth(visible)
	state.NotifScroll = clampWrappedScroll(state.NotifScroll, state.NotifSelected, len(visible), viewport, func(i int) int {
		if i < 0 || i >= len(visible) {
			return 1
		}
		prefix := padToDisplayWidth(timeAgo(visible[i].updatedAt), timeColWidth) + " "
		repo := padToDisplayWidth(clampDisplayWidth(oneLine(visible[i].repo), repoColWidth), repoColWidth)
		label := prefix + repo + "  " + oneLine(visible[i].title)
		titleIndent := strings.Repeat(" ", lipgloss.Width(prefix)+repoColWidth+2)
		return len(wrapDisplayWidth(label, avail, titleIndent))
	})
}

func clampNotificationScroll(state *AppState) {
	visible := state.visibleNotifications()
	maxScroll := len(visible) - 1
	if maxScroll < 0 {
		state.NotifScroll = 0
		return
	}
	if state.NotifScroll < 0 {
		state.NotifScroll = 0
		return
	}
	if state.NotifScroll > maxScroll {
		state.NotifScroll = maxScroll
	}
}

func normalizeTimeline(state *AppState, ts *timelineState) {
	rows := ts.displayRows()
	if len(rows) == 0 {
		ts.selectedID = ""
		ts.selectedIndex = 0
		ts.scrollOffset = 0
		return
	}
	idx := indexOfTimelineSelection(rows, ts.selectedID)
	if idx < 0 {
		idx = 0
	}
	ts.selectedIndex = idx
	ts.selectedID = rows[idx].id
	maxScroll := len(rows) - 1
	if maxScroll < 0 {
		ts.scrollOffset = 0
		return
	}
	if ts.scrollOffset < 0 {
		ts.scrollOffset = 0
		return
	}
	if ts.scrollOffset > maxScroll {
		ts.scrollOffset = maxScroll
	}
}

func ensureTimelineSelectionVisible(state *AppState, ts *timelineState) {
	rows := ts.displayRows()
	if len(rows) == 0 {
		return
	}
	if ts.selectedIndex < 0 {
		ts.selectedIndex = indexOfTimelineSelection(rows, ts.selectedID)
	}
	if ts.selectedIndex < 0 {
		return
	}
	viewport := timelineViewportRows(*state)
	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	plan := buildTimelineViewportPlan(ts, midW, viewport)
	ts.scrollOffset = plan.start
}

func normalizeThread(state *AppState, ts *timelineState) {
	if ts.activeThreadID == "" {
		if state.Focus == focusThread {
			state.Focus = focusTimeline
		}
		return
	}
	rows := ts.threadRows(ts.activeThreadID)
	if len(rows) == 0 {
		ts.threadSelectedID = ""
		ts.threadSelectedIndex = 0
		ts.threadScrollOffset = 0
		if state.Focus == focusThread || state.Focus == focusDetail {
			// Keep the active thread view available for empty thread panes.
			return
		}
		return
	}
	idx := indexOfThreadSelection(rows, ts.threadSelectedID)
	if idx < 0 {
		if ts.threadSelectedIndex < 0 {
			ts.threadSelectedIndex = 0
		}
		if ts.threadSelectedIndex >= len(rows) {
			ts.threadSelectedIndex = len(rows) - 1
		}
		idx = ts.threadSelectedIndex
	}
	ts.threadSelectedIndex = idx
	ts.threadSelectedID = rows[idx].id
	if ts.threadScrollOffset < 0 {
		ts.threadScrollOffset = 0
	}
	if ts.threadScrollOffset >= len(rows) {
		ts.threadScrollOffset = len(rows) - 1
	}
	ensureThreadSelectionVisible(state, ts)

	if state.Focus != focusThread && state.Focus != focusDetail {
		return
	}
	// If the timeline selection is no longer this active thread root, pop thread drill mode.
	timelineRows := ts.displayRows()
	tIdx := indexOfTimelineSelection(timelineRows, ts.selectedID)
	if tIdx < 0 || tIdx >= len(timelineRows) {
		ts.activeThreadID = ""
		ts.threadSelectedID = ""
		ts.threadSelectedIndex = 0
		ts.threadScrollOffset = 0
		if state.Focus == focusThread {
			state.Focus = focusTimeline
		}
		return
	}
	row := timelineRows[tIdx]
	if !row.isThreadHeader || row.threadID != ts.activeThreadID {
		ts.activeThreadID = ""
		ts.threadSelectedID = ""
		ts.threadSelectedIndex = 0
		ts.threadScrollOffset = 0
		if state.Focus == focusThread {
			state.Focus = focusTimeline
		}
	}
}

func ensureThreadSelectionVisible(state *AppState, ts *timelineState) {
	if ts == nil || ts.activeThreadID == "" {
		return
	}
	rows := ts.threadRows(ts.activeThreadID)
	if len(rows) == 0 {
		ts.threadScrollOffset = 0
		return
	}
	if ts.threadSelectedIndex < 0 {
		ts.threadSelectedIndex = indexOfThreadSelection(rows, ts.threadSelectedID)
	}
	if ts.threadSelectedIndex < 0 {
		ts.threadSelectedIndex = 0
	}
	viewport := timelineViewportRows(*state)
	mode := state.currentPaneMode()
	_, midW, _ := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	avail := contentWidth(midW)
	if avail < 1 {
		avail = 1
	}
	actorWidth := timelineActorColumnWidth(rows)
	ts.threadScrollOffset = clampWrappedScroll(ts.threadScrollOffset, ts.threadSelectedIndex, len(rows), viewport, func(i int) int {
		if i < 0 || i >= len(rows) {
			return 1
		}
		return len(wrapThreadRow(rows[i], ts, avail, actorWidth))
	})
}

func clampTimelineScroll(ts *timelineState) {
	maxScroll := len(ts.displayRows()) - 1
	if maxScroll < 0 {
		ts.scrollOffset = 0
		return
	}
	if ts.scrollOffset < 0 {
		ts.scrollOffset = 0
		return
	}
	if ts.scrollOffset > maxScroll {
		ts.scrollOffset = maxScroll
	}
}

func clampWrappedScroll(scroll, selected, length, viewport int, lineCount func(int) int) int {
	if viewport <= 0 {
		viewport = 1
	}
	if length <= 0 {
		return 0
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= length {
		selected = length - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= length {
		scroll = length - 1
	}

	if selected < scroll {
		scroll = selected
	}

	for scroll < selected {
		offset := 0
		for i := scroll; i < selected; i++ {
			c := lineCount(i)
			if c < 1 {
				c = 1
			}
			offset += c
			if offset >= viewport {
				break
			}
		}
		if offset < viewport {
			break
		}
		scroll++
	}

	maxScroll := length - 1
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func notificationViewportRows(state AppState) int {
	rows := paneInnerHeight(state) - 1
	if rows < 1 {
		return 1
	}
	return rows
}

func cycleNotificationTab(state *AppState, effects *[]Effect, direction int) {
	tabs := state.notificationTabs()
	if len(tabs) <= 1 {
		return
	}
	if direction == 0 {
		direction = +1
	}
	current := state.activeNotificationTab()
	idx := 0
	for i := range tabs {
		if tabs[i] == current {
			idx = i
			break
		}
	}
	nextIdx := idx + direction
	for nextIdx < 0 {
		nextIdx += len(tabs)
	}
	next := tabs[nextIdx%len(tabs)]
	state.NotifTab = next
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
	idx = indexOfNotificationByID(visible, targetID)
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

func selectNotificationByID(state *AppState, effects *[]Effect, id string) {
	if id == "" {
		return
	}
	prevRef := state.CurrentRef
	visible := state.visibleNotifications()
	state.NotifSelected = indexOfNotificationByID(visible, id)
	state.SelectedNotif = id
	state.setCurrentRefFromSelectedNotification()
	ensureNotificationSelectionVisible(state)
	if state.CurrentRef != prevRef {
		state.TimelineGen++
		*effects = append(*effects, CancelTimelineEffect{})
	}
	if state.CurrentRef == "" {
		return
	}
	ensureTimelineState(state, state.CurrentRef)
	ts := state.TimelineByRef[state.CurrentRef]
	if !ts.done {
		if state.CurrentRef == prevRef {
			state.TimelineGen++
		}
		*effects = append(*effects, StartTimelineEffect{Generation: state.TimelineGen, Ref: state.CurrentRef})
		ts.loading = true
		ts.done = false
		ts.err = ""
	}
}

func timelineViewportRows(state AppState) int {
	rows := paneInnerHeight(state) - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func paneInnerHeight(state AppState) int {
	statusHeight := 1
	panesOuterHeight := state.Height - statusHeight
	if panesOuterHeight < 1 {
		panesOuterHeight = 1
	}
	panesInnerHeight := panesOuterHeight
	if panesInnerHeight < 1 {
		panesInnerHeight = 1
	}
	return panesInnerHeight
}

func normalizeDetail(state *AppState) {
	if state.DetailScroll < 0 {
		state.DetailScroll = 0
	}
	mode := state.currentPaneMode()
	_, _, rightW := paneWidths(panesTotalWidth(state.Width, state.Focus, mode), state.Focus, mode)
	if rightW < 1 {
		return
	}
	avail := contentWidth(rightW)
	if avail < 1 {
		avail = 1
	}
	viewport := paneInnerHeight(*state)
	if viewport < 1 {
		viewport = 1
	}
	total := detailWrappedLineCount(*state, avail)
	maxScroll := total - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.DetailScroll > maxScroll {
		state.DetailScroll = maxScroll
	}
}

func detailWrappedLineCount(state AppState, avail int) int {
	count := 0
	for _, line := range detailLines(state) {
		safe := sanitizeForRender(line)
		for _, part := range strings.Split(safe, "\n") {
			wrapped := wrapDisplayWidth(part, avail, "")
			count += len(wrapped)
		}
	}
	if count < 1 {
		return 1
	}
	return count
}
