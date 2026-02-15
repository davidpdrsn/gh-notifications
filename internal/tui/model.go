package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	timelineCancelByRef map[string]context.CancelFunc

	styles styles
}

type styles struct {
	title           lipgloss.Style
	text            lipgloss.Style
	secondary       lipgloss.Style
	selected        lipgloss.Style
	selectedMuted   lipgloss.Style
	current         lipgloss.Style
	currentMuted    lipgloss.Style
	muted           lipgloss.Style
	unreadMarker    lipgloss.Style
	unreadSelected  lipgloss.Style
	unreadCurrent   lipgloss.Style
	kindPR          lipgloss.Style
	kindIS          lipgloss.Style
	kindUnknown     lipgloss.Style
	kindPRDraft     lipgloss.Style
	kindPRWaiting   lipgloss.Style
	kindPRSelected  lipgloss.Style
	kindISSelected  lipgloss.Style
	kindUnkSelected lipgloss.Style
	kindPRDraftSel  lipgloss.Style
	kindPRWaitSel   lipgloss.Style
	kindPRCurrent   lipgloss.Style
	kindISCurrent   lipgloss.Style
	kindUnkCurrent  lipgloss.Style
	kindPRDraftCur  lipgloss.Style
	kindPRWaitCur   lipgloss.Style
	error           lipgloss.Style
	status          lipgloss.Style
	tab             lipgloss.Style
	tabActive       lipgloss.Style
	separator       lipgloss.Style
	inactiveColumn  lipgloss.Style
	eventInfo       lipgloss.Style
	eventSuccess    lipgloss.Style
	eventWarning    lipgloss.Style
	eventDanger     lipgloss.Style
	diffHeader      lipgloss.Style
	diffHunk        lipgloss.Style
	diffAdd         lipgloss.Style
	diffDel         lipgloss.Style
	lineNumber      lipgloss.Style
	lineNumberZero  lipgloss.Style
}

func newModel(ctx context.Context, client *ghpr.Client, store *readstate.Store) *model {
	t := catppuccinMocha
	markedBg := lipgloss.Color("#323548")

	return &model{
		ctx:                 ctx,
		client:              client,
		store:               store,
		state:               NewState(),
		msgCh:               make(chan tea.Msg, 512),
		timelineCancelByRef: make(map[string]context.CancelFunc),
		styles: styles{
			title:     lipgloss.NewStyle().Bold(true).Foreground(t.title),
			text:      lipgloss.NewStyle().Foreground(t.textPrimary),
			secondary: lipgloss.NewStyle().Foreground(t.textSecondary),
			selected:  lipgloss.NewStyle().Background(markedBg),
			selectedMuted: lipgloss.NewStyle().
				Background(markedBg).
				Foreground(t.textMuted),
			current: lipgloss.NewStyle().
				Background(lipgloss.Color("#3A3C4F")),
			currentMuted: lipgloss.NewStyle().
				Background(lipgloss.Color("#3A3C4F")).
				Foreground(t.textMuted),
			muted:          lipgloss.NewStyle().Foreground(t.textMuted),
			unreadMarker:   lipgloss.NewStyle().Foreground(t.warning),
			unreadSelected: lipgloss.NewStyle().Foreground(t.warning).Background(markedBg),
			unreadCurrent:  lipgloss.NewStyle().Foreground(t.warning).Background(lipgloss.Color("#3A3C4F")),
			kindPR:         lipgloss.NewStyle().Foreground(t.info),
			kindIS:         lipgloss.NewStyle().Foreground(t.success),
			kindUnknown:    lipgloss.NewStyle().Foreground(t.textMuted),
			kindPRDraft:    lipgloss.NewStyle().Foreground(t.textMuted),
			kindPRWaiting:  lipgloss.NewStyle().Foreground(t.info),
			kindPRSelected: lipgloss.NewStyle().Foreground(t.info).Background(markedBg),
			kindISSelected: lipgloss.NewStyle().Foreground(t.success).Background(markedBg),
			kindUnkSelected: lipgloss.NewStyle().
				Foreground(t.textMuted).
				Background(markedBg),
			kindPRDraftSel: lipgloss.NewStyle().Foreground(t.textMuted).Background(markedBg),
			kindPRWaitSel: lipgloss.NewStyle().
				Foreground(t.info).
				Background(markedBg),
			kindPRCurrent:  lipgloss.NewStyle().Foreground(t.info).Background(lipgloss.Color("#3A3C4F")),
			kindISCurrent:  lipgloss.NewStyle().Foreground(t.success).Background(lipgloss.Color("#3A3C4F")),
			kindUnkCurrent: lipgloss.NewStyle().Foreground(t.textMuted).Background(lipgloss.Color("#3A3C4F")),
			kindPRDraftCur: lipgloss.NewStyle().Foreground(t.textMuted).Background(lipgloss.Color("#3A3C4F")),
			kindPRWaitCur:  lipgloss.NewStyle().Foreground(t.info).Background(lipgloss.Color("#3A3C4F")),
			error:          lipgloss.NewStyle().Foreground(t.danger),
			status:         lipgloss.NewStyle().Foreground(t.textMuted),
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

func (m *model) bottomStatus() string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(m.state.Status) != "" {
		parts = append(parts, m.state.Status)
	}
	if m.state.RefreshInFlight {
		spinnerFrames := []string{"-", "\\", "|", "/"}
		frame := spinnerFrames[m.state.RefreshSpinnerIndex%len(spinnerFrames)]
		left, total := m.state.refreshProgress()
		parts = append(parts, fmt.Sprintf("refresh %s %d/%d", frame, left, total))
	} else {
		queued, inFlight := m.state.timelineLoadProgress()
		left := queued + inFlight
		if left > 0 {
			total := m.state.TimelineLoadTotal
			if total < left {
				total = left
			}
			parts = append(parts, fmt.Sprintf("loading %d/%d", left, total))
		}
	}
	if !m.state.LastRefreshAt.IsZero() {
		parts = append(parts, "refreshed "+relativeRefreshAge(m.state.LastRefreshAt))
	} else {
		parts = append(parts, "refreshed never")
	}
	right := strings.Join(parts, " | ")
	if right == "" {
		return ""
	}
	rightW := lipgloss.Width(right)
	avail := m.state.Width - 1
	if avail < 1 {
		avail = 1
	}
	if rightW >= avail {
		return clampDisplayWidth(right, avail)
	}
	return right
}

func relativeRefreshAge(ts time.Time) string {
	d := time.Since(ts)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d / time.Minute)
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d / time.Hour)
		if hours == 1 {
			return "1 hr ago"
		}
		return fmt.Sprintf("%d hr ago", hours)
	}
	days := int(d / (24 * time.Hour))
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
