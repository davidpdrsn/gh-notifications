package tui

import (
	"context"

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

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	events := make([]Event, 0, 1)
	asyncMsg := false

	switch t := msg.(type) {
	case tea.WindowSizeMsg:
		events = append(events, WindowSizeEvent{Width: t.Width, Height: t.Height})
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
			m.startNotificationsLoader(e.Generation)
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
			m.timelineCancel = cancel
			m.startTimelineLoader(ctx, e.Generation, e.Ref)
		}
	}
}

func (m *model) startNotificationsLoader(gen int) {
	go func() {
		err := m.client.StreamNotifications(m.ctx, func(item ghpr.NotificationEvent) error {
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
