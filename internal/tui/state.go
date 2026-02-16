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
	focusThread
	focusDetail
)

type paneMode int

const (
	paneModeNotificationsTimeline paneMode = iota
	paneModeTimelineDetail
	paneModeTimelineThread
	paneModeThreadDetail
)

type notifRow struct {
	id        string
	updatedAt time.Time
	title     string
	repo      string
	kind      string
	author    string
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
	ref                 string
	rows                []timelineRow
	rowIndexByID        map[string]int
	threadByID          map[string]*threadGroup
	expandedThreads     map[string]bool
	readByEventID       map[string]bool
	readKnownByEventID  map[string]bool
	readLoadInFlight    map[string]bool
	commitDiffByID      map[string]commitDiffState
	forcePushByID       map[string]forcePushDiffState
	selectedID          string
	selectedIndex       int
	scrollOffset        int
	activeThreadID      string
	threadSelectedID    string
	threadSelectedIndex int
	threadScrollOffset  int
	loading             bool
	done                bool
	err                 string
}

type timelineRefreshAnchor struct {
	selectedID          string
	selectedIndex       int
	activeThreadID      string
	threadSelectedID    string
	threadSelectedIndex int
}

type commitDiffState struct {
	loading bool
	err     string
	body    string
}

type forcePushDiffState struct {
	loading    bool
	err        string
	beforeSHA  string
	afterSHA   string
	compareURL string
	body       string
}

type displayTimelineRow struct {
	id             string
	label          string
	threadID       string
	isThreadHeader bool
	isThreadRoot   bool
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
	NotifTab       string

	TimelineGen               int
	CurrentRef                string
	TimelineByRef             map[string]*timelineState
	TimelineLoadQueue         []string
	TimelineLoadGenByRef      map[string]int
	TimelineLoadInFlightByRef map[string]bool
	TimelineLoadPendingByRef  map[string]bool
	TimelineLoadTotal         int
	HideRead                  bool

	Status string
	Quit   bool

	TimelineLoadingRef string

	RefreshInFlight            bool
	RefreshPending             bool
	RefreshStage               string
	RefreshQueue               []string
	RefreshTotalRefs           int
	RefreshActiveRef           string
	RefreshSpinnerIndex        int
	LastRefreshAt              time.Time
	RefreshNotifAnchorID       string
	RefreshNotifAnchorIndex    int
	RefreshNotifBuffer         []notifRow
	RefreshNotifSeen           map[string]bool
	RefreshTimelinePrevByRef   map[string]*timelineState
	RefreshTimelineAnchorByRef map[string]timelineRefreshAnchor

	DetailScroll                int
	NextReadOpID                int64
	PendingRead                 map[int64]pendingReadOp
	PendingParentRead           map[int64]pendingParentReadOp
	NextArchiveOpID             int64
	PendingArchive              map[int64]pendingArchiveOp
	NextUnsubscribeOpID         int64
	PendingUnsubscribe          map[int64]pendingArchiveOp
	ParentReadByRef             map[string]bool
	ParentReadLoadedByRef       map[string]bool
	ParentReadLoadInFlightByRef map[string]bool
	ReadThroughRef              string
	ReadThroughIDs              map[string]bool
	MotionCount                 string
	notifMarkerByRef            map[string]string
	ConfirmIntent               *confirmIntentState
	HelpOpen                    bool
	MarkedNotifications         map[string]bool
	MarkedTimelineByRef         map[string]map[string]bool
	MarkedThreadByRef           map[string]map[string]bool
	ViewerLogin                 string
	ViewerLoaded                bool
	ReviewReqByRef              map[string]bool
	ReviewReqMergedByRef        map[string]bool
	ReviewReqClosedByRef        map[string]bool
	ReviewReqDraftByRef         map[string]bool
	AuthorByRef                 map[string]string
	ReviewReqLoadedByRef        map[string]bool
	ReviewReqLoadInFlightByRef  map[string]bool
}

type pendingReadOp struct {
	ref       string
	eventIDs  []string
	read      bool
	prevRead  map[string]bool
	prevKnown map[string]bool
}

type pendingParentReadOp struct {
	ref        string
	read       bool
	prevLoaded bool
	prevRead   bool
}

type pendingArchiveOp struct {
	notifID  string
	ref      string
	threadID string
	from     focusColumn
}

type confirmActionKind string

const (
	confirmActionArchive     confirmActionKind = "archive"
	confirmActionUnsubscribe confirmActionKind = "unsubscribe"
)

type confirmIntentState struct {
	Kind           confirmActionKind
	TargetNotifIDs []string
	PrimaryNotifID string
	From           focusColumn
}

func NewState() AppState {
	return AppState{
		Focus:                       focusNotifications,
		NotifGen:                    1,
		NotifIndexByID:              make(map[string]int),
		NotifLoading:                true,
		NotifTab:                    allNotificationsTab,
		TimelineByRef:               make(map[string]*timelineState),
		TimelineLoadGenByRef:        make(map[string]int),
		TimelineLoadInFlightByRef:   make(map[string]bool),
		TimelineLoadPendingByRef:    make(map[string]bool),
		RefreshNotifSeen:            make(map[string]bool),
		RefreshTimelinePrevByRef:    make(map[string]*timelineState),
		RefreshTimelineAnchorByRef:  make(map[string]timelineRefreshAnchor),
		PendingRead:                 make(map[int64]pendingReadOp),
		PendingParentRead:           make(map[int64]pendingParentReadOp),
		PendingArchive:              make(map[int64]pendingArchiveOp),
		PendingUnsubscribe:          make(map[int64]pendingArchiveOp),
		ParentReadByRef:             make(map[string]bool),
		ParentReadLoadedByRef:       make(map[string]bool),
		ParentReadLoadInFlightByRef: make(map[string]bool),
		ReadThroughIDs:              make(map[string]bool),
		notifMarkerByRef:            make(map[string]string),
		MarkedNotifications:         make(map[string]bool),
		MarkedTimelineByRef:         make(map[string]map[string]bool),
		MarkedThreadByRef:           make(map[string]map[string]bool),
		ReviewReqByRef:              make(map[string]bool),
		ReviewReqMergedByRef:        make(map[string]bool),
		ReviewReqClosedByRef:        make(map[string]bool),
		ReviewReqDraftByRef:         make(map[string]bool),
		AuthorByRef:                 make(map[string]string),
		ReviewReqLoadedByRef:        make(map[string]bool),
		ReviewReqLoadInFlightByRef:  make(map[string]bool),
	}
}

func (s AppState) refreshProgress() (left int, total int) {
	total = s.RefreshTotalRefs
	if total < 0 {
		total = 0
	}

	if !s.RefreshInFlight {
		return 0, total
	}

	switch s.RefreshStage {
	case "timeline":
		left = len(s.RefreshQueue)
	case "notifications":
		left = 0
	default:
		left = len(s.RefreshQueue)
	}

	if left < 0 {
		left = 0
	}
	if total < left {
		total = left
	}

	return left, total
}

func (s AppState) timelineLoadProgress() (queued int, inFlight int) {
	queued = len(s.TimelineLoadQueue)
	inFlight = len(s.TimelineLoadInFlightByRef)
	if queued < 0 {
		queued = 0
	}
	if inFlight < 0 {
		inFlight = 0
	}
	return queued, inFlight
}

const allNotificationsTab = "All"

func (s *AppState) currentTimeline() *timelineState {
	if s.CurrentRef == "" {
		return nil
	}
	ts, ok := s.TimelineByRef[s.CurrentRef]
	if !ok {
		ts = &timelineState{
			ref:                s.CurrentRef,
			rowIndexByID:       make(map[string]int),
			threadByID:         make(map[string]*threadGroup),
			expandedThreads:    make(map[string]bool),
			readByEventID:      make(map[string]bool),
			readKnownByEventID: make(map[string]bool),
			readLoadInFlight:   make(map[string]bool),
			commitDiffByID:     make(map[string]commitDiffState),
			forcePushByID:      make(map[string]forcePushDiffState),
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

func (s *AppState) currentPaneMode() paneMode {
	if s.Focus == focusNotifications {
		return paneModeNotificationsTimeline
	}
	if s == nil || s.CurrentRef == "" {
		return paneModeTimelineDetail
	}
	ts := s.TimelineByRef[s.CurrentRef]
	if ts != nil && ts.activeThreadID != "" {
		if s.Focus == focusTimeline {
			return paneModeTimelineThread
		}
		return paneModeThreadDetail
	}
	return paneModeTimelineDetail
}

func (s *AppState) notificationTabs() []string {
	tabs := []string{allNotificationsTab}
	seen := map[string]bool{allNotificationsTab: true}
	for _, n := range s.Notifications {
		org := notificationOrgFromRepo(n.repo)
		if org == "" || seen[org] {
			continue
		}
		tabs = append(tabs, org)
		seen[org] = true
	}
	return tabs
}

func (s *AppState) activeNotificationTab() string {
	tab := strings.TrimSpace(s.NotifTab)
	if tab == "" {
		return allNotificationsTab
	}
	for _, t := range s.notificationTabs() {
		if t == tab {
			return t
		}
	}
	return allNotificationsTab
}

func (s *AppState) visibleNotifications() []notifRow {
	tab := s.activeNotificationTab()
	rows := make([]notifRow, 0, len(s.Notifications))
	for _, n := range s.Notifications {
		if !s.notificationRowReady(n) {
			continue
		}
		if tab != allNotificationsTab && notificationOrgFromRepo(n.repo) != tab {
			continue
		}
		if s.HideRead {
			known, read := s.notificationReadState(n)
			if known && read {
				continue
			}
		}
		rows = append(rows, n)
	}
	return rows
}

func (s *AppState) notificationRowReady(n notifRow) bool {
	if strings.TrimSpace(strings.ToLower(n.kind)) != "pr" {
		return true
	}
	return s.ReviewReqLoadedByRef[strings.TrimSpace(n.ref)]
}

func indexOfNotificationByID(rows []notifRow, id string) int {
	if id == "" {
		return -1
	}
	for i := range rows {
		if rows[i].id == id {
			return i
		}
	}
	return -1
}

func (s *AppState) selectedTimelineEvent() *ghpr.TimelineEvent {
	ts := s.currentTimeline()
	if ts == nil {
		return nil
	}
	rows := ts.displayRows(s.HideRead)
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
	return nil
}

func (s *AppState) selectedThreadEvent() *ghpr.TimelineEvent {
	ts := s.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		return nil
	}
	rows := ts.threadRows(ts.activeThreadID, s.HideRead)
	if len(rows) == 0 {
		return nil
	}
	idx := indexOfThreadSelection(rows, ts.threadSelectedID)
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	return rows[idx].event
}

func (s *AppState) selectedDetailEvent() *ghpr.TimelineEvent {
	ts := s.currentTimeline()
	if ts == nil {
		return nil
	}
	if ts.activeThreadID != "" && (s.Focus == focusThread || s.Focus == focusDetail) {
		if ev := s.selectedThreadEvent(); ev != nil {
			return ev
		}
	}
	return s.selectedTimelineEvent()
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
		author:    strings.TrimSpace(s.AuthorByRef[item.Target.Ref]),
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
		display := ts.displayRows(false)
		if len(display) > 0 {
			ts.selectedID = display[0].id
			ts.selectedIndex = 0
		}
	} else {
		ts.selectedID = selected
		display := ts.displayRows(false)
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

func (ts *timelineState) displayRows(hideRead bool) []displayTimelineRow {
	rows := make([]displayTimelineRow, 0, len(ts.rows))
	for _, base := range ts.rows {
		if base.thread != nil {
			root := base.thread.rootEvent()
			if root == nil {
				continue
			}
			head := displayTimelineRow{
				id:             threadHeaderID(base.thread.id),
				threadID:       base.thread.id,
				isThreadHeader: true,
				event:          root,
				label:          compactEventSummary(*root),
			}
			if hideRead && ts.rowRead(head) {
				continue
			}
			rows = append(rows, head)
			continue
		}
		if base.event != nil {
			ev := *base.event
			row := displayTimelineRow{
				id:    eventRowID(ev.ID),
				event: &ev,
				label: compactEventSummary(ev),
			}
			if hideRead && ts.rowRead(row) {
				continue
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func (ts *timelineState) threadRows(threadID string, hideRead bool) []displayTimelineRow {
	group := ts.threadByID[threadID]
	if group == nil || len(group.items) == 0 {
		return nil
	}
	rows := make([]displayTimelineRow, 0, len(group.items))
	for i := 0; i < len(group.items); i++ {
		ev := group.items[i]
		row := displayTimelineRow{
			id:           threadChildID(threadID, ev.ID),
			threadID:     threadID,
			isThreadRoot: i == 0,
			event:        &ev,
			label:        compactThreadChildSummary(ev),
		}
		if hideRead && ts.rowRead(row) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func (ts *timelineState) rowLeafEventIDs(row displayTimelineRow) []string {
	if row.isThreadHeader {
		group := ts.threadByID[row.threadID]
		if group == nil {
			return nil
		}
		ids := make([]string, 0, len(group.items))
		for _, ev := range group.items {
			if ev.ID == "" {
				continue
			}
			ids = append(ids, ev.ID)
		}
		return ids
	}
	if row.event == nil || row.event.ID == "" {
		return nil
	}
	return []string{row.event.ID}
}

func (ts *timelineState) rowRead(row displayTimelineRow) bool {
	ids := ts.rowLeafEventIDs(row)
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if !ts.readByEventID[id] {
			return false
		}
	}
	return true
}

func (ts *timelineState) rowReadKnown(row displayTimelineRow) bool {
	ids := ts.rowLeafEventIDs(row)
	if len(ids) == 0 {
		return true
	}
	for _, id := range ids {
		if ts.readKnownByEventID[id] {
			continue
		}
		if ts.readLoadInFlight[id] {
			return false
		}
	}
	return true
}

func (ts *timelineState) rowsReadyForDisplay(rows []displayTimelineRow) []displayTimelineRow {
	filtered := make([]displayTimelineRow, 0, len(rows))
	for _, row := range rows {
		if !ts.rowReadKnown(row) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func (ts *timelineState) hasPendingReadState(rows []displayTimelineRow) bool {
	for _, row := range rows {
		if !ts.rowReadKnown(row) {
			return true
		}
	}
	return false
}

func (ts *timelineState) rowUnreadMarker(row displayTimelineRow) string {
	ids := ts.rowLeafEventIDs(row)
	if len(ids) == 0 {
		return "    "
	}

	unread := 0
	for _, id := range ids {
		if !ts.readByEventID[id] {
			unread++
		}
	}
	if unread == 0 {
		return "    "
	}
	if unread == len(ids) {
		return " ●  "
	}
	return " ◐  "
}

func (ts *timelineState) allEventIDs() []string {
	ids := make([]string, 0, len(ts.rows))
	for _, base := range ts.rows {
		if base.event != nil && base.event.ID != "" {
			ids = append(ids, base.event.ID)
			continue
		}
		if base.thread == nil {
			continue
		}
		for _, ev := range base.thread.items {
			if ev.ID == "" {
				continue
			}
			ids = append(ids, ev.ID)
		}
	}
	return ids
}

func (s *AppState) notificationReadState(n notifRow) (known bool, read bool) {
	ts := s.TimelineByRef[n.ref]
	if ts != nil {
		ids := ts.allEventIDs()
		if len(ids) > 0 {
			for _, id := range ids {
				if id == "" {
					continue
				}
				if !ts.readKnownByEventID[id] {
					return false, false
				}
				if !ts.readByEventID[id] {
					return true, false
				}
			}
			return true, true
		}
	}

	if s.ParentReadByRef[n.ref] {
		return true, true
	}
	if s.ParentReadLoadedByRef[n.ref] {
		return true, false
	}
	if ts == nil {
		return false, false
	}
	return ts.done, ts.done
}

func (s *AppState) notificationUnreadMarker(n notifRow) string {
	ts := s.TimelineByRef[n.ref]
	if s.notifMarkerByRef == nil {
		s.notifMarkerByRef = make(map[string]string)
	}
	if strings.TrimSpace(n.kind) == "pr" {
		if s.ReviewReqByRef[n.ref] {
			s.notifMarkerByRef[n.ref] = " !  "
			return " !  "
		}
		if s.ReviewReqMergedByRef[n.ref] {
			s.notifMarkerByRef[n.ref] = " +  "
			return " +  "
		}
		if s.ReviewReqClosedByRef[n.ref] {
			s.notifMarkerByRef[n.ref] = " +  "
			return " +  "
		}
	}
	known, read := s.notificationReadState(n)
	if known {
		if read {
			s.notifMarkerByRef[n.ref] = "    "
			return "    "
		}
		if ts == nil {
			s.notifMarkerByRef[n.ref] = " ●  "
			return " ●  "
		}
	}
	if ts == nil {
		if cached, ok := s.notifMarkerByRef[n.ref]; ok {
			return cached
		}
		return " ●  "
	}
	if !known {
		if cached, ok := s.notifMarkerByRef[n.ref]; ok {
			return cached
		}
		return " ●  "
	}
	if read {
		s.notifMarkerByRef[n.ref] = "    "
		return "    "
	}
	ids := ts.allEventIDs()
	if len(ids) == 0 {
		if !read {
			s.notifMarkerByRef[n.ref] = " ●  "
			return " ●  "
		}
		s.notifMarkerByRef[n.ref] = "    "
		return "    "
	}

	unread := 0
	for _, id := range ids {
		if !ts.readByEventID[id] {
			unread++
		}
	}
	if unread == 0 {
		s.notifMarkerByRef[n.ref] = "    "
		return "    "
	}
	if unread == len(ids) {
		s.notifMarkerByRef[n.ref] = " ●  "
		return " ●  "
	}
	s.notifMarkerByRef[n.ref] = " ◐  "
	return " ◐  "
}

func compactThreadChildSummary(ev ghpr.TimelineEvent) string {
	actor := eventActorLabel(ev)
	message := truncatePreview(eventPreviewText(ev), 96)

	parts := make([]string, 0, 2)
	if actor != "" {
		parts = append(parts, actor)
	}
	if message != "" {
		parts = append(parts, message)
	}
	return stringsJoin(parts, "  ")
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

func indexOfThreadSelection(rows []displayTimelineRow, selectedID string) int {
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
	return -1
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

func compactEventSummary(ev ghpr.TimelineEvent) string {
	kind := eventKindLabel(ev)
	actor := eventActorLabel(ev)
	message := truncatePreview(eventPreviewText(ev), 96)

	parts := []string{kind}
	if actor != "" {
		parts = append(parts, actor)
	}
	if message != "" {
		parts = append(parts, message)
	}
	return stringsJoin(parts, "  ")
}

func eventKindLabel(ev ghpr.TimelineEvent) string {
	if ev.Event != nil {
		if e := oneLine(*ev.Event); e != "" {
			if e == "head_ref_force_pushed" {
				return "force_pushed"
			}
			return e
		}
	}
	if idx := strings.LastIndex(ev.Type, "."); idx >= 0 && idx < len(ev.Type)-1 {
		label := ev.Type[idx+1:]
		if label == "head_ref_force_pushed" {
			return "force_pushed"
		}
		return label
	}
	return ev.Type
}

func eventActorLabel(ev ghpr.TimelineEvent) string {
	if ev.Actor == nil {
		return ""
	}
	return oneLine(ev.Actor.Login)
}

func eventPreviewText(ev ghpr.TimelineEvent) string {
	if ev.Comment != nil && ev.Comment.Body != nil {
		if body := oneLine(*ev.Comment.Body); body != "" {
			return body
		}
	}
	if ev.Pr != nil {
		if title := oneLine(ev.Pr.Title); title != "" {
			return title
		}
	}
	if ev.Issue != nil {
		if title := oneLine(ev.Issue.Title); title != "" {
			return title
		}
	}
	if ev.Commit != nil && ev.Commit.SHA != nil {
		if sha := oneLine(*ev.Commit.SHA); sha != "" {
			if len(sha) > 12 {
				sha = sha[:12]
			}
			return sha
		}
	}
	return ""
}

func truncatePreview(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	s = oneLine(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "."
	}
	return strings.TrimSpace(s[:maxLen-3]) + "..."
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func compactThreadPath(path string) string {
	path = oneLine(path)
	if path == "" {
		return path
	}

	parts := strings.Split(path, "/")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}

	if len(filtered) < 4 {
		return stringsJoin(filtered, "/")
	}

	return filtered[0] + "/../" + filtered[len(filtered)-2] + "/" + filtered[len(filtered)-1]
}

func eventRowID(id string) string              { return "event:" + id }
func threadHeaderID(threadID string) string    { return "thread:" + threadID }
func threadChildID(threadID, id string) string { return "thread:" + threadID + ":" + id }

func (g *threadGroup) rootEvent() *ghpr.TimelineEvent {
	if g == nil || len(g.items) == 0 {
		return nil
	}
	ev := g.items[0]
	return &ev
}

func firstNonEmptyPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func notificationOrgFromRepo(repo string) string {
	repo = oneLine(repo)
	if repo == "" {
		return ""
	}
	idx := strings.Index(repo, "/")
	if idx <= 0 {
		return repo
	}
	return repo[:idx]
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
