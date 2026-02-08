package tui

import (
	"context"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	ctx    context.Context
	client *ghpr.Client

	state AppState

	msgCh chan tea.Msg

	timelineCancel context.CancelFunc

	styles styles
}

type styles struct {
	borderActive   lipgloss.Style
	borderInactive lipgloss.Style
	title          lipgloss.Style
	selected       lipgloss.Style
	muted          lipgloss.Style
	status         lipgloss.Style
}

func newModel(ctx context.Context, client *ghpr.Client) *model {
	return &model{
		ctx:    ctx,
		client: client,
		state:  NewState(),
		msgCh:  make(chan tea.Msg, 512),
		styles: styles{
			borderActive: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("39")),
			borderInactive: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")),
			title:    lipgloss.NewStyle().Bold(true),
			selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")),
			muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
			status: lipgloss.NewStyle().
				Foreground(lipgloss.Color("36")),
		},
	}
}

func (m *model) Init() tea.Cmd {
	next, effects := Reduce(m.state, InitEvent{})
	m.state = next
	m.applyEffects(effects)
	return waitForAsyncMsg(m.msgCh)
}

func waitForAsyncMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m *model) debugStatus() string {
	parts := []string{"q quit", "h/l back/drill", "j/k move"}
	return stringsJoin(parts, "   ")
}
