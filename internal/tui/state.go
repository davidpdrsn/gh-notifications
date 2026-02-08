package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gh-pr/ghpr"
)

type focusColumn int

const (
	focusNotifications focusColumn = iota
	focusTimeline
	focusDetail
)

type notifRow struct {
	id        string
	updatedAt time.Time
	title     string
	repo      string
	kind      string
	ref       string
}

type timelineRow struct {
	id     string
	sortAt time.Time
	pinned bool
	event  *ghpr.TimelineEvent
	thread *threadGroup
}

type threadGroup struct {
	id      string
	path    string
	firstAt time.Time
	lastAt  time.Time
	items   []ghpr.TimelineEvent
}

type timelineState struct {
	ref             string
	rows            []timelineRow
	rowIndexByID    map[string]int
	threadByID      map[string]*threadGroup
	expandedThreads map[string]bool
	selectedID      string
	selectedIndex   int
	scrollOffset    int
	loading         bool
	done            bool
	err             string
}

type displayTimelineRow struct {
	id             string
	label          string
	threadID       string
	isThreadHeader bool
	event          *ghpr.TimelineEvent
}

type AppState struct {
	Width  int
	Height int
	Focus  focusColumn

	NotifGen       int
	Notifications  []notifRow
	NotifIndexByID map[string]int
	SelectedNotif  string
	NotifSelected  int
	NotifScroll    int
	NotifLoading   bool
	NotifDone      bool
	NotifErr       string

	TimelineGen   int
	CurrentRef    string
	TimelineByRef map[string]*timelineState

	Status string
	Quit   bool
}

func NewState() AppState {
	return AppState{
		Focus:          focusNotifications,
		NotifGen:       1,
		NotifIndexByID: make(map[string]int),
		NotifLoading:   true,
		TimelineByRef:  make(map[string]*timelineState),
	}
}

func (s *AppState) currentTimeline() *timelineState {
	if s.CurrentRef == "" {
		return nil
	}
	ts, ok := s.TimelineByRef[s.CurrentRef]
	if !ok {
		ts = &timelineState{
			ref:             s.CurrentRef,
			rowIndexByID:    make(map[string]int),
			threadByID:      make(map[string]*threadGroup),
			expandedThreads: make(map[string]bool),
		}
		s.TimelineByRef[s.CurrentRef] = ts
	}
	return ts
}

func (s *AppState) selectedNotification() *notifRow {
	if s.SelectedNotif == "" {
		return nil
	}
	idx, ok := s.NotifIndexByID[s.SelectedNotif]
	if !ok || idx < 0 || idx >= len(s.Notifications) {
		return nil
	}
	n := s.Notifications[idx]
	return &n
}

func (s *AppState) selectedTimelineEvent() *ghpr.TimelineEvent {
	ts := s.currentTimeline()
	if ts == nil {
		return nil
	}
	rows := ts.displayRows()
	if len(rows) == 0 {
		return nil
	}
	idx := indexOfTimelineSelection(rows, ts.selectedID)
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	row := rows[idx]
	if row.event != nil {
		return row.event
	}
	if row.isThreadHeader {
		return &ghpr.TimelineEvent{Type: "thread", ID: row.threadID}
	}
	return nil
}

func (s *AppState) rebuildNotifIndex() {
	s.NotifIndexByID = make(map[string]int, len(s.Notifications))
	for i := range s.Notifications {
		s.NotifIndexByID[s.Notifications[i].id] = i
	}
}

func (ts *timelineState) rebuildRowIndex() {
	ts.rowIndexByID = make(map[string]int, len(ts.rows))
	for i := range ts.rows {
		ts.rowIndexByID[ts.rows[i].id] = i
	}
}

func (s *AppState) insertNotification(item ghpr.NotificationEvent) bool {
	row := notifRow{
		id:        item.ID,
		updatedAt: item.UpdatedAt,
		title:     item.Subject.Title,
		repo:      fmt.Sprintf("%s/%s", item.Repository.Owner, item.Repository.Repo),
		kind:      item.Target.Kind,
		ref:       item.Target.Ref,
	}
	if _, exists := s.NotifIndexByID[row.id]; exists {
		return false
	}

	selected := s.SelectedNotif
	idx := sort.Search(len(s.Notifications), func(i int) bool {
		return notifComesBefore(row, s.Notifications[i])
	})
	s.Notifications = append(s.Notifications, notifRow{})
	copy(s.Notifications[idx+1:], s.Notifications[idx:])
	s.Notifications[idx] = row
	s.rebuildNotifIndex()

	if selected == "" && len(s.Notifications) > 0 {
		s.SelectedNotif = s.Notifications[0].id
		s.NotifSelected = 0
		s.setCurrentRefFromSelectedNotification()
		return true
	}
	if selected != "" {
		s.SelectedNotif = selected
		if idx, ok := s.NotifIndexByID[selected]; ok {
			s.NotifSelected = idx
		}
	}
	return false
}

func (s *AppState) setCurrentRefFromSelectedNotification() {
	if s.SelectedNotif == "" {
		s.CurrentRef = ""
		return
	}
	idx, ok := s.NotifIndexByID[s.SelectedNotif]
	if !ok {
		return
	}
	s.CurrentRef = s.Notifications[idx].ref
}

func notifComesBefore(a notifRow, b notifRow) bool {
	if !a.updatedAt.Equal(b.updatedAt) {
		return a.updatedAt.After(b.updatedAt)
	}
	return a.id < b.id
}

func (ts *timelineState) insertTimelineEvent(ev ghpr.TimelineEvent) {
	selected := ts.selectedID

	if isThreadedReviewComment(ev) {
		ts.insertThreadedEvent(ev)
	} else {
		row := timelineRow{
			id:     eventRowID(ev.ID),
			sortAt: ev.OccurredAt,
			event:  &ev,
			pinned: ev.Type == "pr.opened" || ev.Type == "issue.opened",
		}
		if _, exists := ts.rowIndexByID[row.id]; !exists {
			ts.insertBaseRow(row)
		}
	}

	if selected == "" {
		display := ts.displayRows()
		if len(display) > 0 {
			ts.selectedID = display[0].id
			ts.selectedIndex = 0
		}
	} else {
		ts.selectedID = selected
		display := ts.displayRows()
		ts.selectedIndex = indexOfTimelineSelection(display, ts.selectedID)
	}
}

func (ts *timelineState) insertThreadedEvent(ev ghpr.TimelineEvent) {
	threadID := *ev.Comment.ThreadID
	group, ok := ts.threadByID[threadID]
	if !ok {
		group = &threadGroup{id: threadID, path: firstNonEmptyPtr(ev.Comment.Path), firstAt: ev.OccurredAt, lastAt: ev.OccurredAt}
		group.items = append(group.items, ev)
		ts.threadByID[threadID] = group
		ts.expandedThreads[threadID] = true
		ts.insertBaseRow(timelineRow{
			id:     threadHeaderID(threadID),
			sortAt: ev.OccurredAt,
			thread: group,
		})
		return
	}
	if ev.ID != "" {
		for i := range group.items {
			if group.items[i].ID == ev.ID {
				return
			}
		}
	}

	idx := sort.Search(len(group.items), func(i int) bool {
		return timelineEventComesBefore(ev, group.items[i])
	})
	group.items = append(group.items, ghpr.TimelineEvent{})
	copy(group.items[idx+1:], group.items[idx:])
	group.items[idx] = ev
	if ev.OccurredAt.Before(group.firstAt) {
		group.firstAt = ev.OccurredAt
		if ridx, exists := ts.rowIndexByID[threadHeaderID(threadID)]; exists {
			row := ts.rows[ridx]
			ts.rows = append(ts.rows[:ridx], ts.rows[ridx+1:]...)
			row.sortAt = ev.OccurredAt
			ts.rebuildRowIndex()
			ts.insertBaseRow(row)
		}
	}
	if ev.OccurredAt.After(group.lastAt) {
		group.lastAt = ev.OccurredAt
	}
	if group.path == "" {
		group.path = firstNonEmptyPtr(ev.Comment.Path)
	}
}

func (ts *timelineState) insertBaseRow(row timelineRow) {
	idx := sort.Search(len(ts.rows), func(i int) bool {
		return timelineRowComesBefore(row, ts.rows[i])
	})
	ts.rows = append(ts.rows, timelineRow{})
	copy(ts.rows[idx+1:], ts.rows[idx:])
	ts.rows[idx] = row
	ts.rebuildRowIndex()
}

func (ts *timelineState) displayRows() []displayTimelineRow {
	rows := make([]displayTimelineRow, 0, len(ts.rows))
	for _, base := range ts.rows {
		if base.thread != nil {
			head := displayTimelineRow{
				id:             threadHeaderID(base.thread.id),
				threadID:       base.thread.id,
				isThreadHeader: true,
				label:          fmt.Sprintf("%s (%d comments)", firstNonEmpty(base.thread.path, "thread"), len(base.thread.items)),
			}
			rows = append(rows, head)
			if ts.expandedThreads[base.thread.id] {
				for i := range base.thread.items {
					ev := base.thread.items[i]
					lead := "│ "
					prefix := "├─"
					if i == len(base.thread.items)-1 {
						lead = "  "
						prefix = "└─"
					}
					label := lead + prefix + " " + eventSummary(ev)
					rows = append(rows, displayTimelineRow{
						id:       threadChildID(base.thread.id, ev.ID),
						threadID: base.thread.id,
						event:    &ev,
						label:    label,
					})
				}
			}
			continue
		}
		if base.event != nil {
			ev := *base.event
			rows = append(rows, displayTimelineRow{
				id:    eventRowID(ev.ID),
				event: &ev,
				label: eventSummary(ev),
			})
		}
	}
	return rows
}

func indexOfTimelineSelection(rows []displayTimelineRow, selectedID string) int {
	if len(rows) == 0 {
		return -1
	}
	if selectedID == "" {
		return 0
	}
	for i := range rows {
		if rows[i].id == selectedID {
			return i
		}
	}
	return 0
}

func timelineRowComesBefore(a timelineRow, b timelineRow) bool {
	if a.pinned != b.pinned {
		return a.pinned
	}
	if !a.sortAt.Equal(b.sortAt) {
		return a.sortAt.Before(b.sortAt)
	}
	return a.id < b.id
}

func timelineEventComesBefore(a ghpr.TimelineEvent, b ghpr.TimelineEvent) bool {
	if !a.OccurredAt.Equal(b.OccurredAt) {
		return a.OccurredAt.Before(b.OccurredAt)
	}
	return a.ID < b.ID
}

func isThreadedReviewComment(ev ghpr.TimelineEvent) bool {
	return ev.Type == "github.review_comment" && ev.Comment != nil && ev.Comment.ThreadID != nil && *ev.Comment.ThreadID != ""
}

func eventSummary(ev ghpr.TimelineEvent) string {
	base := ev.Type
	if ev.Event != nil && *ev.Event != "" {
		base = *ev.Event
	}
	if ev.Comment != nil && ev.Comment.Body != nil && *ev.Comment.Body != "" {
		return fmt.Sprintf("%s: %s", base, oneLine(*ev.Comment.Body))
	}
	if ev.Pr != nil {
		return fmt.Sprintf("pr.opened: %s", oneLine(ev.Pr.Title))
	}
	if ev.Issue != nil {
		return fmt.Sprintf("issue.opened: %s", oneLine(ev.Issue.Title))
	}
	return base
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

func eventRowID(id string) string              { return "event:" + id }
func threadHeaderID(threadID string) string    { return "thread:" + threadID }
func threadChildID(threadID, id string) string { return "thread:" + threadID + ":" + id }

func firstNonEmptyPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if item != "" {
			return item
		}
	}
	return ""
}

func stringsJoin(items []string, sep string) string {
	out := ""
	for i := range items {
		if i > 0 {
			out += sep
		}
		out += items[i]
	}
	return out
}
