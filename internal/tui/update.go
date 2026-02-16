package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
)

type notifArrivedMsg struct {
	gen  int
	item ghpr.NotificationEvent
}

type notifDoneMsg struct{ gen int }

type notifErrMsg struct {
	gen int
	err error
}

type timelineArrivedMsg struct {
	gen   int
	ref   string
	event ghpr.TimelineEvent
}

type timelineWarnMsg struct {
	gen int
	ref string
	msg string
}

type timelineDoneMsg struct {
	gen int
	ref string
}

type timelineErrMsg struct {
	gen int
	ref string
	err error
}

type readStateLoadedMsg struct {
	ref      string
	eventIDs []string
	readIDs  []string
}

type readStateLoadErrMsg struct {
	ref      string
	eventIDs []string
	err      error
}

type readStatePersistedMsg struct {
	opID int64
}

type readStatePersistErrMsg struct {
	opID int64
	err  error
}

type parentReadStateLoadedMsg struct {
	refs     []string
	readRefs []string
}

type parentReadStateLoadErrMsg struct {
	refs []string
	err  error
}

type parentReadStatePersistedMsg struct {
	opID int64
}

type parentReadStatePersistErrMsg struct {
	opID int64
	err  error
}

type viewerLoadedMsg struct {
	login string
}

type viewerLoadErrMsg struct {
	err error
}

type reviewReqStateLoadedMsg struct {
	refs        []string
	pendingRefs []string
	mergedRefs  []string
	closedRefs  []string
	draftRefs   []string
	authorByRef map[string]string
}

type reviewReqStateLoadErrMsg struct {
	refs []string
	err  error
}

type ciStateLoadedMsg struct {
	refs        []string
	successRefs []string
	pendingRefs []string
	failedRefs  []string
}

type ciStateLoadErrMsg struct {
	refs []string
	err  error
}

type archiveNotificationSucceededMsg struct {
	opID int64
}

type archiveNotificationErrMsg struct {
	opID int64
	err  error
}

type unsubscribeNotificationSucceededMsg struct {
	opID int64
}

type unsubscribeNotificationErrMsg struct {
	opID int64
	err  error
}

type commitDiffLoadedMsg struct {
	ref     string
	eventID string
	diff    string
}

type commitDiffErrMsg struct {
	ref     string
	eventID string
	err     error
}

type forcePushInterdiffLoadedMsg struct {
	ref        string
	eventID    string
	beforeSHA  string
	afterSHA   string
	compareURL string
	diff       string
}

type forcePushInterdiffErrMsg struct {
	ref     string
	eventID string
	err     error
}

type clipboardCopiedMsg struct{ column string }

type clipboardErrMsg struct {
	column string
	err    error
}

type urlOpenErrMsg struct {
	url string
	err error
}

type autoRefreshTickMsg struct{}
type refreshSpinnerTickMsg struct{}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	events := make([]Event, 0, 1)
	asyncMsg := false

	switch t := msg.(type) {
	case tea.WindowSizeMsg:
		events = append(events, WindowSizeEvent{Width: t.Width, Height: t.Height})
	case tea.MouseMsg:
		switch {
		case t.Button == tea.MouseButtonWheelUp:
			events = append(events, MouseWheelEvent{X: t.X, Y: t.Y, Delta: -1})
		case t.Button == tea.MouseButtonWheelDown:
			events = append(events, MouseWheelEvent{X: t.X, Y: t.Y, Delta: 1})
		case t.Action == tea.MouseActionPress && t.Button == tea.MouseButtonLeft:
			events = append(events, MouseClickEvent{X: t.X, Y: t.Y, Button: mouseButtonLeft})
		}
	case tea.KeyMsg:
		events = append(events, KeyEvent{Key: t.String()})
	case notifArrivedMsg:
		asyncMsg = true
		events = append(events, NotificationsArrivedEvent{Generation: t.gen, Item: t.item})
	case notifDoneMsg:
		asyncMsg = true
		events = append(events, NotificationsDoneEvent{Generation: t.gen})
	case notifErrMsg:
		asyncMsg = true
		events = append(events, NotificationsErrEvent{Generation: t.gen, Err: t.err.Error()})
	case timelineArrivedMsg:
		asyncMsg = true
		events = append(events, TimelineArrivedEvent{Generation: t.gen, Ref: t.ref, Event: t.event})
	case timelineWarnMsg:
		asyncMsg = true
		events = append(events, TimelineWarnEvent{Generation: t.gen, Ref: t.ref, Message: t.msg})
	case timelineDoneMsg:
		asyncMsg = true
		events = append(events, TimelineDoneEvent{Generation: t.gen, Ref: t.ref})
	case timelineErrMsg:
		asyncMsg = true
		events = append(events, TimelineErrEvent{Generation: t.gen, Ref: t.ref, Err: t.err.Error()})
	case readStateLoadedMsg:
		asyncMsg = true
		events = append(events, ReadStateLoadedEvent{Ref: t.ref, EventIDs: t.eventIDs, ReadIDs: t.readIDs})
	case readStateLoadErrMsg:
		asyncMsg = true
		events = append(events, ReadStateLoadFailedEvent{Ref: t.ref, EventIDs: t.eventIDs, Err: t.err.Error()})
	case readStatePersistedMsg:
		asyncMsg = true
		events = append(events, ReadStatePersistedEvent{OpID: t.opID})
	case readStatePersistErrMsg:
		asyncMsg = true
		events = append(events, ReadStatePersistFailedEvent{OpID: t.opID, Err: t.err.Error()})
	case parentReadStateLoadedMsg:
		asyncMsg = true
		events = append(events, ParentReadStateLoadedEvent{Refs: t.refs, ReadRefs: t.readRefs})
	case parentReadStateLoadErrMsg:
		asyncMsg = true
		events = append(events, ParentReadStateLoadFailedEvent{Refs: t.refs, Err: t.err.Error()})
	case parentReadStatePersistedMsg:
		asyncMsg = true
		events = append(events, ParentReadStatePersistedEvent{OpID: t.opID})
	case parentReadStatePersistErrMsg:
		asyncMsg = true
		events = append(events, ParentReadStatePersistFailedEvent{OpID: t.opID, Err: t.err.Error()})
	case viewerLoadedMsg:
		asyncMsg = true
		events = append(events, ViewerLoadedEvent{Login: t.login})
	case viewerLoadErrMsg:
		asyncMsg = true
		events = append(events, ViewerLoadFailedEvent{Err: t.err.Error()})
	case reviewReqStateLoadedMsg:
		asyncMsg = true
		events = append(events, ReviewReqStateLoadedEvent{Refs: t.refs, PendingRefs: t.pendingRefs, MergedRefs: t.mergedRefs, ClosedRefs: t.closedRefs, DraftRefs: t.draftRefs, AuthorByRef: t.authorByRef})
	case reviewReqStateLoadErrMsg:
		asyncMsg = true
		events = append(events, ReviewReqStateLoadFailedEvent{Refs: t.refs, Err: t.err.Error()})
	case ciStateLoadedMsg:
		asyncMsg = true
		events = append(events, CIStateLoadedEvent{Refs: t.refs, SuccessRefs: t.successRefs, PendingRefs: t.pendingRefs, FailedRefs: t.failedRefs})
	case ciStateLoadErrMsg:
		asyncMsg = true
		events = append(events, CIStateLoadFailedEvent{Refs: t.refs, Err: t.err.Error()})
	case archiveNotificationSucceededMsg:
		asyncMsg = true
		events = append(events, ArchiveNotificationSucceededEvent{OpID: t.opID})
	case archiveNotificationErrMsg:
		asyncMsg = true
		events = append(events, ArchiveNotificationFailedEvent{OpID: t.opID, Err: t.err.Error()})
	case unsubscribeNotificationSucceededMsg:
		asyncMsg = true
		events = append(events, UnsubscribeNotificationSucceededEvent{OpID: t.opID})
	case unsubscribeNotificationErrMsg:
		asyncMsg = true
		events = append(events, UnsubscribeNotificationFailedEvent{OpID: t.opID, Err: t.err.Error()})
	case commitDiffLoadedMsg:
		asyncMsg = true
		events = append(events, CommitDiffLoadedEvent{Ref: t.ref, EventID: t.eventID, Diff: t.diff})
	case commitDiffErrMsg:
		asyncMsg = true
		events = append(events, CommitDiffErrEvent{Ref: t.ref, EventID: t.eventID, Err: t.err.Error()})
	case forcePushInterdiffLoadedMsg:
		asyncMsg = true
		events = append(events, ForcePushInterdiffLoadedEvent{Ref: t.ref, EventID: t.eventID, BeforeSHA: t.beforeSHA, AfterSHA: t.afterSHA, CompareURL: t.compareURL, Diff: t.diff})
	case forcePushInterdiffErrMsg:
		asyncMsg = true
		events = append(events, ForcePushInterdiffErrEvent{Ref: t.ref, EventID: t.eventID, Err: t.err.Error()})
	case clipboardCopiedMsg:
		asyncMsg = true
		events = append(events, ClipboardCopiedEvent{Column: t.column})
	case clipboardErrMsg:
		asyncMsg = true
		events = append(events, ClipboardCopyFailedEvent{Column: t.column, Err: t.err.Error()})
	case urlOpenErrMsg:
		asyncMsg = true
		events = append(events, URLOpenFailedEvent{URL: t.url, Err: t.err.Error()})
	case autoRefreshTickMsg:
		asyncMsg = true
		events = append(events, AutoRefreshTickEvent{})
	case refreshSpinnerTickMsg:
		asyncMsg = true
		events = append(events, RefreshSpinnerTickEvent{})
	}

	for _, ev := range events {
		next, effects := Reduce(m.state, ev)
		m.state = next
		m.applyEffects(effects)
	}

	if m.state.Quit {
		return m, tea.Quit
	}

	if asyncMsg {
		return m, waitForAsyncMsg(m.msgCh)
	}

	return m, nil
}

func (m *model) applyEffects(effects []Effect) {
	for _, effect := range effects {
		switch e := effect.(type) {
		case StartNotificationsEffect:
			ctx := m.ctx
			var cancel context.CancelFunc
			if m.state.RefreshInFlight && m.state.RefreshStage == "notifications" {
				refreshCtx, c := context.WithTimeout(m.ctx, 45*time.Second)
				ctx = refreshCtx
				cancel = c
			}
			m.startNotificationsLoader(ctx, cancel, e.Generation)
		case CancelTimelineEffect:
			if strings.TrimSpace(e.Ref) == "" {
				for ref, cancel := range m.timelineCancelByRef {
					if cancel != nil {
						cancel()
					}
					delete(m.timelineCancelByRef, ref)
				}
				break
			}
			if cancel, ok := m.timelineCancelByRef[e.Ref]; ok {
				if cancel != nil {
					cancel()
				}
				delete(m.timelineCancelByRef, e.Ref)
			}
		case StartTimelineEffect:
			if cancel, ok := m.timelineCancelByRef[e.Ref]; ok {
				if cancel != nil {
					cancel()
				}
			}
			ctx, cancel := context.WithCancel(m.ctx)
			if m.state.RefreshInFlight && m.state.RefreshStage == "timeline" && m.state.RefreshActiveRef == e.Ref {
				ctx, cancel = context.WithTimeout(m.ctx, 45*time.Second)
			}
			m.timelineCancelByRef[e.Ref] = cancel
			m.startTimelineLoader(ctx, e.Generation, e.Ref)
		case StartCommitDiffEffect:
			m.startCommitDiffLoader(e.Ref, e.EventID, e.DiffURL)
		case StartForcePushInterdiffEffect:
			m.startForcePushInterdiffLoader(e.Ref, e.EventID)
		case CopyColumnEffect:
			m.startClipboardCopy(e.Column, e.Text)
		case OpenURLEffect:
			m.startOpenURL(e.URL)
		case LoadReadStateEffect:
			m.startReadStateLoader(e.Ref, e.EventIDs)
		case PersistReadStateEffect:
			m.startReadStatePersist(e.OpID, e.Ref, e.EventIDs, e.Read)
		case LoadParentReadStateEffect:
			m.startParentReadStateLoader(e.Refs)
		case PersistParentReadStateEffect:
			m.startParentReadStatePersist(e.OpID, e.Ref, e.Read)
		case LoadViewerEffect:
			m.startViewerLoader()
		case LoadReviewReqStateEffect:
			m.startReviewReqStateLoader(e.Refs)
		case LoadCIStateEffect:
			m.startCIStateLoader(e.Refs)
		case ArchiveNotificationEffect:
			m.startArchiveNotification(e.OpID, e.ThreadID, e.UpdatedAt)
		case UnsubscribeNotificationEffect:
			m.startUnsubscribeNotification(e.OpID, e.ThreadID, e.UpdatedAt)
		case ScheduleAutoRefreshTickEffect:
			m.scheduleAutoRefreshTick()
		case ScheduleRefreshSpinnerTickEffect:
			m.scheduleRefreshSpinnerTick()
		}
	}
}

func (m *model) scheduleAutoRefreshTick() {
	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
			m.msgCh <- autoRefreshTickMsg{}
		}
	}()
}

func (m *model) scheduleRefreshSpinnerTick() {
	go func() {
		timer := time.NewTimer(120 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-m.ctx.Done():
			return
		case <-timer.C:
			m.msgCh <- refreshSpinnerTickMsg{}
		}
	}()
}

func (m *model) startReadStateLoader(ref string, eventIDs []string) {
	go func() {
		if m.store == nil {
			m.msgCh <- readStateLoadErrMsg{ref: ref, eventIDs: eventIDs, err: errors.New("read-state store unavailable")}
			return
		}
		read, err := m.store.ListRead(m.ctx, ref, eventIDs)
		if err != nil {
			m.msgCh <- readStateLoadErrMsg{ref: ref, eventIDs: eventIDs, err: err}
			return
		}
		readIDs := make([]string, 0, len(read))
		for id, ok := range read {
			if ok {
				readIDs = append(readIDs, id)
			}
		}
		m.msgCh <- readStateLoadedMsg{ref: ref, eventIDs: eventIDs, readIDs: readIDs}
	}()
}

func (m *model) startReadStatePersist(opID int64, ref string, eventIDs []string, read bool) {
	go func() {
		if m.store == nil {
			m.msgCh <- readStatePersistErrMsg{opID: opID, err: errors.New("read-state store unavailable")}
			return
		}
		var err error
		if read {
			err = m.store.MarkRead(m.ctx, ref, eventIDs)
		} else {
			err = m.store.MarkUnread(m.ctx, ref, eventIDs)
		}
		if err != nil {
			m.msgCh <- readStatePersistErrMsg{opID: opID, err: err}
			return
		}
		m.msgCh <- readStatePersistedMsg{opID: opID}
	}()
}

func (m *model) startParentReadStateLoader(refs []string) {
	go func() {
		if m.store == nil {
			m.msgCh <- parentReadStateLoadErrMsg{refs: refs, err: errors.New("read-state store unavailable")}
			return
		}
		read, err := m.store.ListParentRead(m.ctx, refs)
		if err != nil {
			m.msgCh <- parentReadStateLoadErrMsg{refs: refs, err: err}
			return
		}
		readRefs := make([]string, 0, len(read))
		for ref, ok := range read {
			if ok {
				readRefs = append(readRefs, ref)
			}
		}
		m.msgCh <- parentReadStateLoadedMsg{refs: refs, readRefs: readRefs}
	}()
}

func (m *model) startParentReadStatePersist(opID int64, ref string, read bool) {
	go func() {
		if m.store == nil {
			m.msgCh <- parentReadStatePersistErrMsg{opID: opID, err: errors.New("read-state store unavailable")}
			return
		}
		var err error
		if read {
			err = m.store.MarkParentRead(m.ctx, ref)
		} else {
			err = m.store.MarkParentUnread(m.ctx, ref)
		}
		if err != nil {
			m.msgCh <- parentReadStatePersistErrMsg{opID: opID, err: err}
			return
		}
		m.msgCh <- parentReadStatePersistedMsg{opID: opID}
	}()
}

func (m *model) startViewerLoader() {
	go func() {
		if m.client == nil {
			m.msgCh <- viewerLoadErrMsg{err: errors.New("client unavailable")}
			return
		}
		login, err := m.client.FetchViewerLogin(m.ctx)
		if err != nil {
			m.msgCh <- viewerLoadErrMsg{err: err}
			return
		}
		m.msgCh <- viewerLoadedMsg{login: login}
	}()
}

func (m *model) startReviewReqStateLoader(refs []string) {
	go func() {
		if m.client == nil {
			m.msgCh <- reviewReqStateLoadErrMsg{refs: refs, err: errors.New("client unavailable")}
			return
		}
		viewer := strings.TrimSpace(m.state.ViewerLogin)
		if viewer == "" {
			m.msgCh <- reviewReqStateLoadErrMsg{refs: refs, err: errors.New("viewer unavailable")}
			return
		}
		pending := make([]string, 0, len(refs))
		merged := make([]string, 0, len(refs))
		closed := make([]string, 0, len(refs))
		draft := make([]string, 0, len(refs))
		authorByRef := make(map[string]string, len(refs))
		for _, ref := range refs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			status, err := m.client.ReviewRequestStatusForViewer(m.ctx, ref, viewer)
			if err != nil {
				m.msgCh <- reviewReqStateLoadErrMsg{refs: refs, err: err}
				return
			}
			if status.Pending {
				pending = append(pending, ref)
			}
			if status.Merged {
				merged = append(merged, ref)
			}
			if status.Closed {
				closed = append(closed, ref)
			}
			if status.Draft {
				draft = append(draft, ref)
			}
			if author := strings.TrimSpace(status.Author); author != "" {
				authorByRef[ref] = author
			}
		}
		m.msgCh <- reviewReqStateLoadedMsg{refs: refs, pendingRefs: pending, mergedRefs: merged, closedRefs: closed, draftRefs: draft, authorByRef: authorByRef}
	}()
}

func (m *model) startCIStateLoader(refs []string) {
	go func() {
		if m.client == nil {
			m.msgCh <- ciStateLoadErrMsg{refs: refs, err: errors.New("client unavailable")}
			return
		}
		success := make([]string, 0, len(refs))
		pending := make([]string, 0, len(refs))
		failed := make([]string, 0, len(refs))
		for _, ref := range refs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			status := m.client.CIStatusForPR(m.ctx, ref)
			switch status {
			case ghpr.CIStatusSuccess:
				success = append(success, ref)
			case ghpr.CIStatusPending:
				pending = append(pending, ref)
			case ghpr.CIStatusFailed:
				failed = append(failed, ref)
			}
		}
		m.msgCh <- ciStateLoadedMsg{refs: refs, successRefs: success, pendingRefs: pending, failedRefs: failed}
	}()
}

func (m *model) startArchiveNotification(opID int64, threadID string, updatedAt time.Time) {
	go func() {
		if m.client == nil {
			m.msgCh <- archiveNotificationErrMsg{opID: opID, err: errors.New("client unavailable")}
			return
		}
		if err := m.client.ArchiveNotificationThread(m.ctx, threadID); err != nil {
			m.msgCh <- archiveNotificationErrMsg{opID: opID, err: err}
			return
		}
		if m.store != nil && !updatedAt.IsZero() {
			_ = m.store.MarkThreadArchived(m.ctx, threadID, updatedAt)
		}
		m.msgCh <- archiveNotificationSucceededMsg{opID: opID}
	}()
}

func (m *model) startUnsubscribeNotification(opID int64, threadID string, updatedAt time.Time) {
	go func() {
		if m.client == nil {
			m.msgCh <- unsubscribeNotificationErrMsg{opID: opID, err: errors.New("client unavailable")}
			return
		}
		if err := m.client.UnsubscribeNotificationThread(m.ctx, threadID); err != nil {
			m.msgCh <- unsubscribeNotificationErrMsg{opID: opID, err: err}
			return
		}
		if err := m.client.ArchiveNotificationThread(m.ctx, threadID); err != nil {
			m.msgCh <- unsubscribeNotificationErrMsg{opID: opID, err: err}
			return
		}
		if m.store != nil && !updatedAt.IsZero() {
			_ = m.store.MarkThreadArchived(m.ctx, threadID, updatedAt)
		}
		m.msgCh <- unsubscribeNotificationSucceededMsg{opID: opID}
	}()
}

func (m *model) startCommitDiffLoader(ref, eventID, diffURL string) {
	go func() {
		if m.client == nil {
			m.msgCh <- commitDiffErrMsg{ref: ref, eventID: eventID, err: errors.New("client unavailable")}
			return
		}
		diff, err := m.client.FetchCommitDiff(m.ctx, diffURL)
		if err != nil {
			m.msgCh <- commitDiffErrMsg{ref: ref, eventID: eventID, err: err}
			return
		}
		m.msgCh <- commitDiffLoadedMsg{ref: ref, eventID: eventID, diff: diff}
	}()
}

func (m *model) startForcePushInterdiffLoader(ref, eventID string) {
	go func() {
		if m.client == nil {
			m.msgCh <- forcePushInterdiffErrMsg{ref: ref, eventID: eventID, err: errors.New("client unavailable")}
			return
		}
		interdiff, err := m.client.FetchForcePushInterdiff(m.ctx, ref, eventID)
		if err != nil {
			m.msgCh <- forcePushInterdiffErrMsg{ref: ref, eventID: eventID, err: err}
			return
		}
		m.msgCh <- forcePushInterdiffLoadedMsg{ref: ref, eventID: eventID, beforeSHA: interdiff.BeforeSHA, afterSHA: interdiff.AfterSHA, compareURL: interdiff.CompareURL, diff: interdiff.Diff}
	}()
}

func (m *model) startClipboardCopy(column, text string) {
	go func() {
		if err := copyToClipboard(text); err != nil {
			m.msgCh <- clipboardErrMsg{column: column, err: err}
			return
		}
		m.msgCh <- clipboardCopiedMsg{column: column}
	}()
}

func (m *model) startOpenURL(url string) {
	go func() {
		if err := openURL(url); err != nil {
			m.msgCh <- urlOpenErrMsg{url: url, err: err}
		}
	}()
}

func (m *model) startNotificationsLoader(ctx context.Context, cancel context.CancelFunc, gen int) {
	go func() {
		if cancel != nil {
			defer cancel()
		}
		archived := map[string]time.Time{}
		ignoredByThreadID := map[string]bool{}
		if m.store != nil {
			if loaded, err := m.store.ListArchivedThreads(ctx); err == nil {
				archived = loaded
			}
		}
		err := m.client.StreamNotifications(ctx, func(item ghpr.NotificationEvent) error {
			if shouldSkipArchivedNotification(item, archived) {
				return nil
			}
			if shouldUnarchiveNotification(item, archived) {
				delete(archived, item.ID)
				if m.store != nil {
					_ = m.store.UnmarkThreadArchived(ctx, item.ID)
				}
			}
			skipIgnored, err := shouldSkipIgnoredNotification(ctx, m.client, item.ID, ignoredByThreadID)
			if err == nil && skipIgnored {
				return nil
			}
			m.msgCh <- notifArrivedMsg{gen: gen, item: item}
			return nil
		})
		if err != nil {
			m.msgCh <- notifErrMsg{gen: gen, err: err}
			return
		}
		m.msgCh <- notifDoneMsg{gen: gen}
	}()
}

func shouldSkipIgnoredNotification(ctx context.Context, client *ghpr.Client, threadID string, ignoredByThreadID map[string]bool) (bool, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return false, nil
	}
	if ignored, ok := ignoredByThreadID[threadID]; ok {
		return ignored, nil
	}
	if client == nil {
		return false, errors.New("client unavailable")
	}
	subscription, err := client.FetchNotificationThreadSubscription(ctx, threadID)
	if err != nil {
		return false, err
	}
	ignoredByThreadID[threadID] = subscription.Ignored
	return subscription.Ignored, nil
}

func shouldSkipArchivedNotification(item ghpr.NotificationEvent, archived map[string]time.Time) bool {
	if archived == nil || strings.TrimSpace(item.ID) == "" {
		return false
	}
	archivedUpdatedAt, ok := archived[item.ID]
	if !ok {
		return false
	}
	if item.UpdatedAt.IsZero() {
		return true
	}
	return !item.UpdatedAt.After(archivedUpdatedAt)
}

func shouldUnarchiveNotification(item ghpr.NotificationEvent, archived map[string]time.Time) bool {
	if archived == nil || strings.TrimSpace(item.ID) == "" {
		return false
	}
	archivedUpdatedAt, ok := archived[item.ID]
	if !ok || item.UpdatedAt.IsZero() {
		return false
	}
	return item.UpdatedAt.After(archivedUpdatedAt)
}

func (m *model) startTimelineLoader(ctx context.Context, gen int, ref string) {
	go func() {
		err := m.client.StreamTimeline(ctx, ref, func(event ghpr.TimelineEvent) error {
			m.msgCh <- timelineArrivedMsg{gen: gen, ref: ref, event: event}
			return nil
		}, func(w string) {
			m.msgCh <- timelineWarnMsg{gen: gen, ref: ref, msg: w}
		})
		if err != nil {
			m.msgCh <- timelineErrMsg{gen: gen, ref: ref, err: err}
			return
		}
		m.msgCh <- timelineDoneMsg{gen: gen, ref: ref}
	}()
}
