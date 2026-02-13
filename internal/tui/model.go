package tui

import (
	"context"

	"gh-pr/ghpr"
	"gh-pr/internal/readstate"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	ctx    context.Context
	client *ghpr.Client
	store  *readstate.Store

	state AppState

	msgCh chan tea.Msg

	timelineCancel context.CancelFunc

	styles styles
}

type styles struct {
	title          lipgloss.Style
	text           lipgloss.Style
	secondary      lipgloss.Style
	selected       lipgloss.Style
	selectedMuted  lipgloss.Style
	muted          lipgloss.Style
	unreadMarker   lipgloss.Style
	unreadSelected lipgloss.Style
	error          lipgloss.Style
	status         lipgloss.Style
	tab            lipgloss.Style
	tabActive      lipgloss.Style
	separator      lipgloss.Style
	inactiveColumn lipgloss.Style
	eventInfo      lipgloss.Style
	eventSuccess   lipgloss.Style
	eventWarning   lipgloss.Style
	eventDanger    lipgloss.Style
	diffHeader     lipgloss.Style
	diffHunk       lipgloss.Style
	diffAdd        lipgloss.Style
	diffDel        lipgloss.Style
	lineNumber     lipgloss.Style
	lineNumberZero lipgloss.Style
}

func newModel(ctx context.Context, client *ghpr.Client, store *readstate.Store) *model {
	t := catppuccinMocha

	return &model{
		ctx:    ctx,
		client: client,
		store:  store,
		state:  NewState(),
		msgCh:  make(chan tea.Msg, 512),
		styles: styles{
			title:     lipgloss.NewStyle().Bold(true).Foreground(t.title),
			text:      lipgloss.NewStyle().Foreground(t.textPrimary),
			secondary: lipgloss.NewStyle().Foreground(t.textSecondary),
			selected:  lipgloss.NewStyle().Background(t.selectedBg),
			selectedMuted: lipgloss.NewStyle().
				Background(t.selectedBg).
				Foreground(t.textMuted),
			muted:          lipgloss.NewStyle().Foreground(t.textMuted),
			unreadMarker:   lipgloss.NewStyle().Foreground(t.warning),
			unreadSelected: lipgloss.NewStyle().Foreground(t.warning).Background(t.selectedBg),
			error:          lipgloss.NewStyle().Foreground(t.danger),
			status: lipgloss.NewStyle().
				Foreground(t.statusFg).
				Background(t.statusBg),
			tab:            lipgloss.NewStyle().Foreground(t.textSecondary),
			tabActive:      lipgloss.NewStyle().Foreground(t.focus).Background(t.surface).Bold(true),
			separator:      lipgloss.NewStyle().Foreground(t.separator),
			inactiveColumn: lipgloss.NewStyle().Foreground(t.textMuted),
			eventInfo:      lipgloss.NewStyle().Foreground(t.info),
			eventSuccess:   lipgloss.NewStyle().Foreground(t.success),
			eventWarning:   lipgloss.NewStyle().Foreground(t.warning),
			eventDanger:    lipgloss.NewStyle().Foreground(t.danger),
			diffHeader:     lipgloss.NewStyle().Foreground(t.info),
			diffHunk:       lipgloss.NewStyle().Foreground(t.warning),
			diffAdd:        lipgloss.NewStyle().Foreground(t.success),
			diffDel:        lipgloss.NewStyle().Foreground(t.danger),
			lineNumber: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8A8FA8")).
				Background(lipgloss.Color("#2B2F45")),
			lineNumberZero: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C7CCE5")).
				Background(lipgloss.Color("#3A3F5A")),
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
	parts := []string{"q", "tab", "h/l", "j/k", "r", "H", "^p/^n", "^u/^d", "C"}
	return stringsJoin(parts, "   ")
}
