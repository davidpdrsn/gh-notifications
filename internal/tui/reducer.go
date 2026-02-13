package tui

import (
	"gh-pr/ghpr"
	"strconv"
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
type ReadStateLoadedEvent struct {
	Ref      string
	EventIDs []string
	ReadIDs  []string
}
type ReadStateLoadFailedEvent struct {
	Ref      string
	EventIDs []string
	Err      string
}
type ReadStatePersistedEvent struct {
	OpID int64
}
type ReadStatePersistFailedEvent struct {
	OpID int64
	Err  string
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

func (InitEvent) isEvent()                   {}
func (WindowSizeEvent) isEvent()             {}
func (KeyEvent) isEvent()                    {}
func (NotificationsArrivedEvent) isEvent()   {}
func (NotificationsDoneEvent) isEvent()      {}
func (NotificationsErrEvent) isEvent()       {}
func (TimelineArrivedEvent) isEvent()        {}
func (TimelineWarnEvent) isEvent()           {}
func (TimelineDoneEvent) isEvent()           {}
func (TimelineErrEvent) isEvent()            {}
func (ReadStateLoadedEvent) isEvent()        {}
func (ReadStateLoadFailedEvent) isEvent()    {}
func (ReadStatePersistedEvent) isEvent()     {}
func (ReadStatePersistFailedEvent) isEvent() {}
func (CommitDiffLoadedEvent) isEvent()       {}
func (CommitDiffErrEvent) isEvent()          {}
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
type LoadReadStateEffect struct {
	Ref      string
	EventIDs []string
}
type PersistReadStateEffect struct {
	OpID     int64
	Ref      string
	EventIDs []string
	Read     bool
}

func (StartNotificationsEffect) isEffect() {}
func (StartTimelineEffect) isEffect()      {}
func (CancelTimelineEffect) isEffect()     {}
func (StartCommitDiffEffect) isEffect()    {}
func (StartForcePushInterdiffEffect) isEffect() {
}
func (CopyColumnEffect) isEffect()       {}
func (LoadReadStateEffect) isEffect()    {}
func (PersistReadStateEffect) isEffect() {}

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
		if appendMotionCount(&state, e.Key) {
			break
		}
		count := consumeMotionCount(&state)
		switch e.Key {
		case "ctrl+c", "q":
			clearMotionCount(&state)
			state.Quit = true
			effects = append(effects, CancelTimelineEffect{})
		case "tab":
			clearMotionCount(&state)
			if state.Focus == focusNotifications {
				cycleNotificationTab(&state, &effects, +1)
			}
		case "shift+tab":
			clearMotionCount(&state)
			if state.Focus == focusNotifications {
				cycleNotificationTab(&state, &effects, -1)
			}
		case "ctrl+n":
			clearMotionCount(&state)
			state.DetailScroll++
		case "ctrl+p":
			clearMotionCount(&state)
			if state.DetailScroll > 0 {
				state.DetailScroll--
			}
		case "ctrl+d":
			clearMotionCount(&state)
			state.DetailScroll += 10
		case "ctrl+u":
			clearMotionCount(&state)
			state.DetailScroll -= 10
			if state.DetailScroll < 0 {
				state.DetailScroll = 0
			}
		case "C", "shift+c":
			clearMotionCount(&state)
			column, text := columnCopyText(state)
			if strings.TrimSpace(text) == "" {
				state.Status = "nothing to copy"
				break
			}
			effects = append(effects, CopyColumnEffect{Column: column, Text: text})
		case "down", "j":
			moveDownN(&state, &effects, count)
		case "up", "k":
			moveUpN(&state, &effects, count)
		case "right", "l", "enter":
			clearMotionCount(&state)
			drillIn(&state)
		case "left", "h", "backspace":
			clearMotionCount(&state)
			backOut(&state)
		case "H":
			clearMotionCount(&state)
			state.HideRead = !state.HideRead
		case "r":
			clearMotionCount(&state)
			toggleSelectedRead(&state, &effects)
		default:
			clearMotionCount(&state)
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
				ensureReadStateMaps(ts)
				ts.insertTimelineEvent(e.Event)
				if e.Event.ID != "" && !ts.readKnownByEventID[e.Event.ID] && !ts.readLoadInFlight[e.Event.ID] {
					ts.readLoadInFlight[e.Event.ID] = true
					effects = append(effects, LoadReadStateEffect{Ref: e.Ref, EventIDs: []string{e.Event.ID}})
				}
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
	case ReadStateLoadedEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			ensureReadStateMaps(ts)
			readSet := make(map[string]bool, len(e.ReadIDs))
			for _, id := range e.ReadIDs {
				readSet[id] = true
			}
			for _, id := range e.EventIDs {
				if id == "" {
					continue
				}
				delete(ts.readLoadInFlight, id)
				ts.readKnownByEventID[id] = true
				ts.readByEventID[id] = readSet[id]
			}
		}
	case ReadStateLoadFailedEvent:
		if ts := state.TimelineByRef[e.Ref]; ts != nil {
			ensureReadStateMaps(ts)
			for _, id := range e.EventIDs {
				if id == "" {
					continue
				}
				delete(ts.readLoadInFlight, id)
			}
		}
		if e.Ref == state.CurrentRef {
			state.Status = "failed to load read state: " + e.Err
		}
	case ReadStatePersistedEvent:
		delete(state.PendingRead, e.OpID)
	case ReadStatePersistFailedEvent:
		pending, ok := state.PendingRead[e.OpID]
		if !ok {
			break
		}
		delete(state.PendingRead, e.OpID)
		if ts := state.TimelineByRef[pending.ref]; ts != nil {
			ensureReadStateMaps(ts)
			for _, id := range pending.eventIDs {
				if id == "" {
					continue
				}
				ts.readByEventID[id] = pending.prevRead[id]
				ts.readKnownByEventID[id] = pending.prevKnown[id]
			}
		}
		if pending.ref == state.CurrentRef {
			state.Status = "failed to persist read state: " + e.Err
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
			clampTimelineScroll(ts, state.HideRead)
		}
	case mousePaneThread:
		ts := state.currentTimeline()
		if ts != nil && ts.activeThreadID != "" {
			ts.threadScrollOffset += e.Delta
			rows := ts.threadRows(ts.activeThreadID, state.HideRead)
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
				rows := ts.displayRows(state.HideRead)
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
			rows := ts.threadRows(ts.activeThreadID, state.HideRead)
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
		ref:                ref,
		rowIndexByID:       make(map[string]int),
		threadByID:         make(map[string]*threadGroup),
		expandedThreads:    make(map[string]bool),
		readByEventID:      make(map[string]bool),
		readKnownByEventID: make(map[string]bool),
		readLoadInFlight:   make(map[string]bool),
		commitDiffByID:     make(map[string]commitDiffState),
		forcePushByID:      make(map[string]forcePushDiffState),
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

func toggleSelectedRead(state *AppState, effects *[]Effect) {
	ts := state.currentTimeline()
	if ts == nil {
		return
	}

	targetRef := state.CurrentRef
	targetTS := ts

	var (
		eventIDs    []string
		currentRead bool
		nextID      string
		inThread    bool
	)

	switch state.Focus {
	case focusNotifications:
		n := state.selectedNotification()
		if n == nil {
			return
		}
		targetRef = n.ref
		targetTS = state.TimelineByRef[targetRef]
		if targetTS == nil {
			return
		}
		eventIDs = targetTS.allEventIDs()
		if len(eventIDs) == 0 {
			return
		}
		currentRead = true
		for _, id := range eventIDs {
			if id == "" {
				continue
			}
			if !targetTS.readByEventID[id] {
				currentRead = false
				break
			}
		}
	case focusTimeline:
		rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < 0 || idx >= len(rows) {
			return
		}
		row := rows[idx]
		eventIDs = ts.rowLeafEventIDs(row)
		currentRead = ts.rowRead(row)
		if idx < len(rows)-1 {
			nextID = rows[idx+1].id
		}
	case focusThread:
		if ts.activeThreadID == "" {
			return
		}
		inThread = true
		rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
		if len(rows) == 0 {
			return
		}
		idx := indexOfThreadSelection(rows, ts.threadSelectedID)
		if idx < 0 || idx >= len(rows) {
			return
		}
		row := rows[idx]
		eventIDs = ts.rowLeafEventIDs(row)
		currentRead = ts.rowRead(row)
		if idx < len(rows)-1 {
			nextID = rows[idx+1].id
		}
	case focusDetail:
		if ts.activeThreadID != "" {
			inThread = true
			rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
			if len(rows) == 0 {
				return
			}
			idx := indexOfThreadSelection(rows, ts.threadSelectedID)
			if idx < 0 || idx >= len(rows) {
				return
			}
			row := rows[idx]
			eventIDs = ts.rowLeafEventIDs(row)
			currentRead = ts.rowRead(row)
			if idx < len(rows)-1 {
				nextID = rows[idx+1].id
			}
			break
		}
		rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
		if len(rows) == 0 {
			return
		}
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < 0 || idx >= len(rows) {
			return
		}
		row := rows[idx]
		eventIDs = ts.rowLeafEventIDs(row)
		currentRead = ts.rowRead(row)
		if idx < len(rows)-1 {
			nextID = rows[idx+1].id
		}
	default:
		return
	}

	ensureReadStateMaps(targetTS)

	if len(eventIDs) == 0 {
		return
	}
	desiredRead := !currentRead

	state.NextReadOpID++
	opID := state.NextReadOpID
	if state.PendingRead == nil {
		state.PendingRead = make(map[int64]pendingReadOp)
	}
	prevRead := make(map[string]bool, len(eventIDs))
	prevKnown := make(map[string]bool, len(eventIDs))
	for _, id := range eventIDs {
		if id == "" {
			continue
		}
		prevRead[id] = targetTS.readByEventID[id]
		prevKnown[id] = targetTS.readKnownByEventID[id]
		targetTS.readByEventID[id] = desiredRead
		targetTS.readKnownByEventID[id] = true
	}
	state.PendingRead[opID] = pendingReadOp{
		ref:       targetRef,
		eventIDs:  append([]string(nil), eventIDs...),
		read:      desiredRead,
		prevRead:  prevRead,
		prevKnown: prevKnown,
	}
	*effects = append(*effects, PersistReadStateEffect{
		OpID:     opID,
		Ref:      targetRef,
		EventIDs: append([]string(nil), eventIDs...),
		Read:     desiredRead,
	})

	if nextID != "" {
		if inThread {
			ts.threadSelectedID = nextID
			ts.threadSelectedIndex = -1
			ensureThreadSelectionVisible(state, ts)
		} else {
			ts.selectedID = nextID
			ts.selectedIndex = -1
			ensureTimelineSelectionVisible(state, ts)
		}
		state.DetailScroll = 0
	}
}

func appendMotionCount(state *AppState, key string) bool {
	if len(key) != 1 || key[0] < '0' || key[0] > '9' {
		return false
	}
	if key == "0" && state.MotionCount == "" {
		return false
	}
	state.MotionCount += key
	return true
}

func consumeMotionCount(state *AppState) int {
	if state.MotionCount == "" {
		return 1
	}
	v, err := strconv.Atoi(state.MotionCount)
	state.MotionCount = ""
	if err != nil || v < 1 {
		return 1
	}
	if v > 10000 {
		return 10000
	}
	return v
}

func clearMotionCount(state *AppState) {
	state.MotionCount = ""
}

func moveDownN(state *AppState, effects *[]Effect, n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		moveDown(state, effects)
	}
}

func moveUpN(state *AppState, effects *[]Effect, n int) {
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		moveUp(state, effects)
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
		rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
		rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
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
			rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
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
		rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
		rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
		rows := ts.threadRows(ts.activeThreadID, state.HideRead)
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
			rows := ts.threadRows(ts.activeThreadID, state.HideRead)
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
		rows := ts.displayRows(state.HideRead)
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
		rows := ts.displayRows(state.HideRead)
		idx := indexOfTimelineSelection(rows, ts.selectedID)
		if idx < 0 || idx >= len(rows) {
			return
		}
		row := rows[idx]
		if row.isThreadHeader {
			ts.activeThreadID = row.threadID
			ts.threadScrollOffset = 0
			threadRows := ts.rowsReadyForDisplay(ts.threadRows(row.threadID, state.HideRead))
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
		if len(ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))) == 0 {
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
	avail := paneContentWidthWithRelativeNumbers(leftW, viewport)
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
	rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
	rows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
	plan := buildTimelineViewportPlan(ts, midW, viewport, state.HideRead)
	ts.scrollOffset = plan.start
}

func normalizeThread(state *AppState, ts *timelineState) {
	if ts.activeThreadID == "" {
		if state.Focus == focusThread {
			state.Focus = focusTimeline
		}
		return
	}
	rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
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
	timelineRows := ts.rowsReadyForDisplay(ts.displayRows(state.HideRead))
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
	rows := ts.rowsReadyForDisplay(ts.threadRows(ts.activeThreadID, state.HideRead))
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
	avail := paneContentWidthWithRelativeNumbers(midW, viewport)
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

func clampTimelineScroll(ts *timelineState, hideRead bool) {
	maxScroll := len(ts.rowsReadyForDisplay(ts.displayRows(hideRead))) - 1
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
	queueReadStateLoadsForUnknown(ts, state.CurrentRef, effects)
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

func queueReadStateLoadsForUnknown(ts *timelineState, ref string, effects *[]Effect) {
	if ts == nil {
		return
	}
	ensureReadStateMaps(ts)
	ids := ts.allEventIDs()
	toLoad := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if ts.readKnownByEventID[id] || ts.readLoadInFlight[id] {
			continue
		}
		ts.readLoadInFlight[id] = true
		toLoad = append(toLoad, id)
	}
	if len(toLoad) == 0 {
		return
	}
	*effects = append(*effects, LoadReadStateEffect{Ref: ref, EventIDs: toLoad})
}

func ensureReadStateMaps(ts *timelineState) {
	if ts.readByEventID == nil {
		ts.readByEventID = make(map[string]bool)
	}
	if ts.readKnownByEventID == nil {
		ts.readKnownByEventID = make(map[string]bool)
	}
	if ts.readLoadInFlight == nil {
		ts.readLoadInFlight = make(map[string]bool)
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
