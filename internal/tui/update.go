package tui

import (
	"context"
	"errors"
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
			if m.timelineCancel != nil {
				m.timelineCancel()
				m.timelineCancel = nil
			}
		case StartTimelineEffect:
			if m.timelineCancel != nil {
				m.timelineCancel()
			}
			ctx, cancel := context.WithCancel(m.ctx)
			if m.state.RefreshInFlight && m.state.RefreshStage == "timeline" && m.state.RefreshActiveRef == e.Ref {
				ctx, cancel = context.WithTimeout(m.ctx, 45*time.Second)
			}
			m.timelineCancel = cancel
			m.startTimelineLoader(ctx, e.Generation, e.Ref)
		case StartCommitDiffEffect:
			m.startCommitDiffLoader(e.Ref, e.EventID, e.DiffURL)
		case StartForcePushInterdiffEffect:
			m.startForcePushInterdiffLoader(e.Ref, e.EventID)
		case CopyColumnEffect:
			m.startClipboardCopy(e.Column, e.Text)
		case LoadReadStateEffect:
			m.startReadStateLoader(e.Ref, e.EventIDs)
		case PersistReadStateEffect:
			m.startReadStatePersist(e.OpID, e.Ref, e.EventIDs, e.Read)
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

func (m *model) startNotificationsLoader(ctx context.Context, cancel context.CancelFunc, gen int) {
	go func() {
		if cancel != nil {
			defer cancel()
		}
		err := m.client.StreamNotifications(ctx, func(item ghpr.NotificationEvent) error {
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
