package tui

import (
	"strings"

	"gh-pr/ghpr"
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

type Effect interface{ isEffect() }

type StartNotificationsEffect struct{ Generation int }
type StartTimelineEffect struct {
	Generation int
	Ref        string
}
type CancelTimelineEffect struct{}

func (StartNotificationsEffect) isEffect() {}
func (StartTimelineEffect) isEffect()      {}
func (CancelTimelineEffect) isEffect()     {}

func Reduce(state AppState, ev Event) (AppState, []Effect) {
	effects := make([]Effect, 0)

	switch e := ev.(type) {
	case InitEvent:
		effects = append(effects, StartNotificationsEffect{Generation: state.NotifGen})
	case WindowSizeEvent:
		state.Width = e.Width
		state.Height = e.Height
	case KeyEvent:
		switch e.Key {
		case "ctrl+c", "q":
			state.Quit = true
			effects = append(effects, CancelTimelineEffect{})
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
	}

	normalizeState(&state)

	return state, effects
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
	}
}

func moveDown(state *AppState, effects *[]Effect) {
	switch state.Focus {
	case focusNotifications:
		if len(state.Notifications) == 0 || state.SelectedNotif == "" {
			return
		}
		idx := state.NotifIndexByID[state.SelectedNotif]
		if idx < len(state.Notifications)-1 {
			prevRef := state.CurrentRef
			state.SelectedNotif = state.Notifications[idx+1].id
			state.NotifSelected = idx + 1
			state.setCurrentRefFromSelectedNotification()
			if state.CurrentRef != prevRef {
				state.TimelineGen++
				*effects = append(*effects, CancelTimelineEffect{})
			}
			if state.CurrentRef != "" {
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
		}
	case focusTimeline, focusDetail:
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
		}
	}
}

func moveUp(state *AppState, effects *[]Effect) {
	switch state.Focus {
	case focusNotifications:
		if len(state.Notifications) == 0 || state.SelectedNotif == "" {
			return
		}
		idx := state.NotifIndexByID[state.SelectedNotif]
		if idx > 0 {
			prevRef := state.CurrentRef
			state.SelectedNotif = state.Notifications[idx-1].id
			state.NotifSelected = idx - 1
			state.setCurrentRefFromSelectedNotification()
			if state.CurrentRef != prevRef {
				state.TimelineGen++
				*effects = append(*effects, CancelTimelineEffect{})
			}
			if state.CurrentRef != "" {
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
		}
	case focusTimeline, focusDetail:
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
			ts.expandedThreads[row.threadID] = !ts.expandedThreads[row.threadID]
			return
		}
		state.Focus = focusDetail
	}
}

func backOut(state *AppState) {
	switch state.Focus {
	case focusDetail:
		state.Focus = focusTimeline
	case focusTimeline:
		ts := state.currentTimeline()
		if ts != nil {
			rows := ts.displayRows()
			idx := indexOfTimelineSelection(rows, ts.selectedID)
			if idx >= 0 && idx < len(rows) {
				row := rows[idx]
				threadID := ""
				if row.isThreadHeader {
					threadID = row.threadID
				} else if strings.HasPrefix(row.id, "thread:") {
					threadID = row.threadID
				}
				if threadID != "" && ts.expandedThreads[threadID] {
					ts.expandedThreads[threadID] = false
					ts.selectedID = threadHeaderID(threadID)
					return
				}
			}
		}
		state.Focus = focusNotifications
	}
}

func normalizeState(state *AppState) {
	normalizeNotifications(state)
	if ts := state.currentTimeline(); ts != nil {
		normalizeTimeline(state, ts)
	}
}

func normalizeNotifications(state *AppState) {
	if len(state.Notifications) == 0 {
		state.SelectedNotif = ""
		state.NotifSelected = 0
		state.NotifScroll = 0
		return
	}
	if state.SelectedNotif != "" {
		if idx, ok := state.NotifIndexByID[state.SelectedNotif]; ok {
			state.NotifSelected = idx
		} else {
			state.SelectedNotif = ""
		}
	}
	if state.SelectedNotif == "" {
		if state.NotifSelected < 0 {
			state.NotifSelected = 0
		}
		if state.NotifSelected >= len(state.Notifications) {
			state.NotifSelected = len(state.Notifications) - 1
		}
		state.SelectedNotif = state.Notifications[state.NotifSelected].id
	}
	state.NotifScroll = clampScroll(state.NotifScroll, state.NotifSelected, len(state.Notifications), notificationViewportRows(*state))
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
	ts.scrollOffset = clampScroll(ts.scrollOffset, ts.selectedIndex, len(rows), timelineViewportRows(*state))
}

func clampScroll(scroll, selected, length, viewport int) int {
	if viewport <= 0 {
		viewport = 1
	}
	if length <= viewport {
		return 0
	}
	if selected < scroll {
		scroll = selected
	}
	if selected >= scroll+viewport {
		scroll = selected - viewport + 1
	}
	maxScroll := length - viewport
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}

func notificationViewportRows(state AppState) int {
	rows := paneInnerHeight(state) - 2
	if rows < 1 {
		return 1
	}
	return rows
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
	panesInnerHeight := panesOuterHeight - 2
	if panesInnerHeight < 1 {
		panesInnerHeight = 1
	}
	return panesInnerHeight
}
