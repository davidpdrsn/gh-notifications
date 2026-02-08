package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m *model) View() string {
	if m.state.Width == 0 || m.state.Height == 0 {
		return "loading..."
	}

	statusHeight := 1
	panesOuterHeight := m.state.Height - statusHeight
	if panesOuterHeight < 1 {
		panesOuterHeight = 1
	}
	panesInnerHeight := panesOuterHeight - 2
	if panesInnerHeight < 1 {
		panesInnerHeight = 1
	}

	leftW, midW, rightW := paneWidths(m.state.Width, m.state.Focus)

	left := m.renderNotifications(leftW, panesInnerHeight)
	mid := m.renderTimeline(midW, panesInnerHeight)
	right := m.renderDetail(rightW, panesInnerHeight)

	row := lipgloss.JoinHorizontal(lipgloss.Top, left, mid, right)
	status := m.styles.status.Width(m.state.Width).Render(" " + m.debugStatus())
	return lipgloss.JoinVertical(lipgloss.Left, row, status)
}

func (m *model) renderNotifications(width, height int) string {
	title := "Notifications"
	if m.state.Focus != focusNotifications {
		title = "· Notifications"
	}
	lines := []string{m.styles.title.Render(title)}
	if m.state.NotifLoading {
		lines = append(lines, m.styles.muted.Render("loading..."))
	}
	if m.state.NotifErr != "" {
		lines = append(lines, "error: "+m.state.NotifErr)
	}

	selected := m.state.NotifSelected
	start := m.state.NotifScroll
	end := start + max(1, height-2)
	if end > len(m.state.Notifications) {
		end = len(m.state.Notifications)
	}
	for i := start; i < end; i++ {
		n := m.state.Notifications[i]
		label := fmt.Sprintf("%s  %s  %s", timeAgo(n.updatedAt), n.repo, oneLine(n.title))
		avail := contentWidth(width) - 2
		if avail < 1 {
			avail = 1
		}
		if i == selected {
			label = m.styles.selected.Render("> " + truncateToWidth(label, avail))
		} else {
			label = "  " + truncateToWidth(label, avail)
		}
		lines = append(lines, label)
	}

	box := m.styles.borderInactive
	if m.state.Focus == focusNotifications {
		box = m.styles.borderActive
	}
	innerW := paneInnerWidth(width)
	return box.Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
}

func (m *model) renderTimeline(width, height int) string {
	title := "Timeline"
	if m.state.Focus != focusTimeline {
		title = "· Timeline"
	}
	lines := []string{m.styles.title.Render(title)}
	ts := m.state.currentTimeline()
	if ts == nil {
		lines = append(lines, m.styles.muted.Render("select a notification"))
	} else {
		if ts.loading {
			lines = append(lines, m.styles.muted.Render("loading..."))
		}
		if ts.err != "" {
			lines = append(lines, "error: "+ts.err)
		}
		rows := ts.displayRows()
		selected := ts.selectedIndex
		start := ts.scrollOffset
		end := start + max(1, height-2)
		if end > len(rows) {
			end = len(rows)
		}
		for i := start; i < end; i++ {
			label := rows[i].label
			if rows[i].isThreadHeader {
				if ts.expandedThreads[rows[i].threadID] {
					label = "▾ " + label
				} else {
					label = "▸ " + label
				}
			} else if strings.HasPrefix(rows[i].id, "thread:") {
				label = "  " + label
			}
			if i == selected {
				avail := contentWidth(width) - 2
				if avail < 1 {
					avail = 1
				}
				label = m.styles.selected.Render("> " + truncateToWidth(label, avail))
			} else {
				avail := contentWidth(width) - 2
				if avail < 1 {
					avail = 1
				}
				label = "  " + truncateToWidth(label, avail)
			}
			lines = append(lines, label)
		}
	}

	box := m.styles.borderInactive
	if m.state.Focus == focusTimeline {
		box = m.styles.borderActive
	}
	innerW := paneInnerWidth(width)
	return box.Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
}

func (m *model) renderDetail(width, height int) string {
	title := "Detail"
	if m.state.Focus != focusDetail {
		title = "· Detail"
	}
	lines := []string{m.styles.title.Render(title)}
	if m.state.Focus == focusNotifications {
		if n := m.state.selectedNotification(); n != nil {
			lines = append(lines,
				fmt.Sprintf("%s %s", n.kind, n.ref),
				n.repo,
				n.title,
			)
		}
	} else {
		ev := m.state.selectedTimelineEvent()
		if ev != nil {
			lines = append(lines,
				fmt.Sprintf("type: %s", ev.Type),
				fmt.Sprintf("id: %s", ev.ID),
				fmt.Sprintf("at: %s", ev.OccurredAt.Format(time.RFC3339)),
			)
			if ev.Comment != nil && ev.Comment.Body != nil {
				lines = append(lines, "", *ev.Comment.Body)
				if ev.Comment.URL != nil {
					lines = append(lines, "", "link: "+*ev.Comment.URL)
				}
			}
			if ev.Pr != nil {
				lines = append(lines, "", ev.Pr.Title, ev.Pr.Body)
			}
			if ev.Issue != nil {
				lines = append(lines, "", ev.Issue.Title, ev.Issue.Body)
			}
		}
	}

	box := m.styles.borderInactive
	if m.state.Focus == focusDetail {
		box = m.styles.borderActive
	}
	innerW := paneInnerWidth(width)
	return box.Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
}

func paneInnerWidth(outerWidth int) int {
	w := outerWidth - 2
	if w < 1 {
		return 1
	}
	return w
}

func contentWidth(outerWidth int) int {
	w := paneInnerWidth(outerWidth)
	if w < 1 {
		return 1
	}
	return w
}

func paneWidths(totalWidth int, focus focusColumn) (int, int, int) {
	if totalWidth <= 0 {
		return 1, 1, 1
	}
	if totalWidth == 1 {
		return 1, 0, 0
	}
	if totalWidth == 2 {
		return 1, 1, 0
	}
	if totalWidth == 3 {
		return 1, 1, 1
	}

	r1, r2, r3 := focusRatios(focus)
	minPane := 12
	if totalWidth < minPane*3 {
		return proportionalWidths(totalWidth, r1, r2, r3)
	}

	left := minPane
	mid := minPane
	right := minPane
	remaining := totalWidth - (left + mid + right)
	a, b, c := proportionalWidths(remaining, r1, r2, r3)
	left += a
	mid += b
	right += c
	return left, mid, right
}

func focusRatios(focus focusColumn) (int, int, int) {
	switch focus {
	case focusNotifications:
		return 5, 2, 1
	case focusTimeline:
		return 2, 5, 2
	case focusDetail:
		return 1, 2, 5
	default:
		return 2, 5, 2
	}
}

func proportionalWidths(total, r1, r2, r3 int) (int, int, int) {
	if total <= 0 {
		return 0, 0, 0
	}
	sum := r1 + r2 + r3
	if sum <= 0 {
		sum = 1
	}
	a := total * r1 / sum
	b := total * r2 / sum
	c := total - a - b

	if a < 1 {
		a = 1
	}
	if b < 1 {
		b = 1
	}
	if c < 1 {
		c = 1
	}

	for a+b+c > total {
		if c > 1 {
			c--
			continue
		}
		if b > 1 {
			b--
			continue
		}
		if a > 1 {
			a--
			continue
		}
		return a, b, c
	}
	for a+b+c < total {
		switch {
		case r2 >= r1 && r2 >= r3:
			b++
		case r1 >= r3:
			a++
		default:
			c++
		}
	}

	return a, b, c
}

func fitPaneLines(lines []string, maxLines int, maxWidth int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	out := make([]string, 0, min(maxLines, len(lines)))
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		for _, part := range parts {
			if len(out) >= maxLines {
				return strings.Join(out, "\n")
			}
			if strings.Contains(part, "\x1b[") {
				out = append(out, part)
				continue
			}
			out = append(out, truncateToWidth(part, maxWidth))
		}
	}
	return strings.Join(out, "\n")
}

func truncateToWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return truncateDisplayWidth(s, width)
	}
	return truncateDisplayWidth(s, width-3) + "..."
}

func truncateDisplayWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw <= 0 {
			rw = 0
		}
		if used+rw > maxWidth {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func (m *model) String() string {
	return m.View()
}

var _ tea.Model = &model{}
