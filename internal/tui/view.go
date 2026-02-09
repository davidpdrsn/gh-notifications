package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

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
	panesInnerHeight := panesOuterHeight
	if panesInnerHeight < 1 {
		panesInnerHeight = 1
	}

	mode := m.state.currentPaneMode()
	leftW, midW, rightW := paneWidths(panesTotalWidth(m.state.Width, m.state.Focus, mode), m.state.Focus, mode)

	panes := make([]string, 0, 2)
	if leftW > 0 {
		panes = append(panes, m.stylePaneActivity(m.renderNotifications(leftW, panesInnerHeight), m.state.Focus == focusNotifications))
	}
	if midW > 0 {
		switch midPaneContent(mode) {
		case paneContentThread:
			panes = append(panes, m.stylePaneActivity(m.renderThread(midW, panesInnerHeight), m.state.Focus == focusThread))
		default:
			panes = append(panes, m.stylePaneActivity(m.renderTimeline(midW, panesInnerHeight), m.state.Focus == focusTimeline))
		}
	}
	if rightW > 0 {
		switch rightPaneContent(mode) {
		case paneContentThread:
			panes = append(panes, m.stylePaneActivity(m.renderThread(rightW, panesInnerHeight), m.state.Focus == focusThread))
		default:
			panes = append(panes, m.stylePaneActivity(m.renderDetail(rightW, panesInnerHeight), m.state.Focus == focusDetail))
		}
	}

	row := ""
	if len(panes) == 0 {
		row = ""
	} else if len(panes) == 1 {
		row = panes[0]
	} else {
		sep := m.verticalSeparator(panesInnerHeight)
		row = lipgloss.JoinHorizontal(lipgloss.Top, panes[0], sep, panes[1])
	}
	status := m.styles.status.Width(m.state.Width).Render(" " + m.debugStatus())
	return lipgloss.JoinVertical(lipgloss.Left, row, status)
}

func (m *model) stylePaneActivity(pane string, active bool) string {
	if active {
		return pane
	}
	return m.styles.inactiveColumn.Render(pane)
}

func (m *model) verticalSeparator(height int) string {
	if height < 1 {
		height = 1
	}
	line := m.styles.separator.Render("│")
	parts := make([]string, height)
	for i := range parts {
		parts[i] = line
	}
	return strings.Join(parts, "\n")
}

func (m *model) renderNotifications(width, height int) string {
	lines := make([]string, 0, max(1, height))
	lines = append(lines, m.renderNotificationTabs())
	if m.state.NotifLoading {
		lines = append(lines, m.styles.muted.Render("loading..."))
	}
	if m.state.NotifErr != "" {
		lines = append(lines, m.styles.error.Render("error: "+m.state.NotifErr))
	}
	visible := m.state.visibleNotifications()
	if len(visible) == 0 && !m.state.NotifLoading && m.state.NotifErr == "" {
		lines = append(lines, m.styles.muted.Render("no notifications"))
	}

	selected := m.state.NotifSelected
	start := m.state.NotifScroll
	end := start + max(1, height-1)
	if end > len(visible) {
		end = len(visible)
	}
	timeColWidth := notificationTimeColumnWidth(visible)
	repoColWidth := notificationRepoColumnWidth(visible)
	for i := start; i < end; i++ {
		n := visible[i]
		prefix := padToDisplayWidth(timeAgo(n.updatedAt), timeColWidth) + " "
		repo := padToDisplayWidth(clampDisplayWidth(oneLine(n.repo), repoColWidth), repoColWidth)
		label := prefix + repo + "  " + oneLine(n.title)
		avail := contentWidth(width)
		if avail < 1 {
			avail = 1
		}
		titleIndent := strings.Repeat(" ", lipgloss.Width(prefix)+repoColWidth+2)
		wrapped := wrapDisplayWidth(label, avail, titleIndent)
		if i == selected {
			for _, seg := range wrapped {
				lines = append(lines, renderSelectedNotificationLine(m.styles.selected, m.styles.selectedMuted, seg, avail, timeColWidth+1))
			}
		} else {
			for _, seg := range wrapped {
				lines = append(lines, renderNotificationTimestamp(seg, timeColWidth+1, m.styles.muted))
			}
		}
	}

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
	return m.styles.text.Render(pane)
}

func (m *model) renderNotificationTabs() string {
	tabs := m.state.notificationTabs()
	active := m.state.activeNotificationTab()
	rendered := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		label := " " + tab + " "
		if tab == active {
			rendered = append(rendered, m.styles.tabActive.Render(label))
		} else {
			rendered = append(rendered, m.styles.tab.Render(label))
		}
	}
	return stringsJoin(rendered, " ")
}

func notificationTimeColumnWidth(rows []notifRow) int {
	width := 4
	for i := range rows {
		w := lipgloss.Width(timeAgo(rows[i].updatedAt))
		if w > width {
			width = w
		}
	}
	return width
}

func notificationRepoColumnWidth(rows []notifRow) int {
	width := 0
	for i := range rows {
		w := lipgloss.Width(oneLine(rows[i].repo))
		if w > width {
			width = w
		}
	}
	if width > 36 {
		return 36
	}
	return width
}

func renderNotificationTimestamp(line string, width int, style lipgloss.Style) string {
	if width <= 0 || line == "" {
		return line
	}
	prefix, rest := splitAtDisplayWidth(line, width)
	if prefix == "" {
		return line
	}
	return style.Render(prefix) + rest
}

func renderSelectedNotificationLine(selected lipgloss.Style, selectedMuted lipgloss.Style, line string, width int, timeWidth int) string {
	if width < 1 {
		width = 1
	}
	padded := padToDisplayWidth(line, width)
	if timeWidth <= 0 {
		return selected.Render(padded)
	}
	prefix, rest := splitAtDisplayWidth(padded, timeWidth)
	if prefix == "" {
		return selected.Render(padded)
	}
	return selectedMuted.Render(prefix) + selected.Render(rest)
}

func (m *model) renderTimeline(width, height int) string {
	lines := make([]string, 0, max(1, height))
	highlightSelection := m.state.Focus == focusTimeline
	ts := m.state.currentTimeline()
	if ts == nil {
		lines = append(lines, m.styles.muted.Render("select a notification"))
	} else {
		if ts.loading {
			lines = append(lines, m.styles.muted.Render("loading..."))
		}
		if ts.err != "" {
			lines = append(lines, m.styles.error.Render("error: "+ts.err))
		}
		rows := ts.displayRows()
		plan := buildTimelineViewportPlan(ts, width, max(1, height-2))
		for _, row := range plan.rows {
			if highlightSelection && row.selected {
				wrapped := row.lines
				style := m.styleForTimelineRow(rows[row.index])
				for _, seg := range wrapped {
					lines = append(lines, renderSelectedLineWithBase(m.styles.selected, style, seg, plan.avail))
				}
			} else {
				style := m.styleForTimelineRow(rows[row.index])
				for _, seg := range row.lines {
					lines = append(lines, style.Render(seg))
				}
			}
		}
	}

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
	return m.styles.text.Render(pane)
}

func (m *model) renderThread(width, height int) string {
	lines := make([]string, 0, max(1, height))
	highlightSelection := m.state.Focus == focusThread
	ts := m.state.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		lines = append(lines, m.styles.muted.Render("no thread selected"))
	} else {
		rows := ts.threadRows(ts.activeThreadID)
		if len(rows) == 0 {
			lines = append(lines, m.styles.muted.Render("no replies"))
		} else {
			avail := contentWidth(width)
			if avail < 1 {
				avail = 1
			}
			actorWidth := timelineActorColumnWidth(rows)
			start := ts.threadScrollOffset
			if start < 0 {
				start = 0
			}
			if start >= len(rows) {
				start = len(rows) - 1
			}
			for i := start; i < len(rows); i++ {
				wrapped := wrapThreadRow(rows[i], ts, avail, actorWidth)
				if highlightSelection && i == ts.threadSelectedIndex {
					style := m.styleForTimelineRow(rows[i])
					for _, seg := range wrapped {
						lines = append(lines, renderSelectedLineWithBase(m.styles.selected, style, seg, avail))
					}
				} else {
					style := m.styleForTimelineRow(rows[i])
					for _, seg := range wrapped {
						lines = append(lines, style.Render(seg))
					}
				}
			}
		}
	}

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
	return m.styles.text.Render(pane)
}

func (m *model) renderDetail(width, height int) string {
	lines := make([]string, 0, max(1, height))
	avail := contentWidth(width)
	if avail < 1 {
		avail = 1
	}
	highlightDiff := shouldHighlightDetailDiff(m.state)
	highlightMentions := shouldHighlightDetailMentions(m.state)
	for _, line := range detailLines(m.state) {
		safe := sanitizeForRender(line)
		for _, part := range strings.Split(safe, "\n") {
			kind := ""
			if highlightDiff {
				kind = diffLineKind(part)
			}
			for _, seg := range wrapDisplayWidth(part, avail, "") {
				if highlightMentions {
					seg = highlightDetailMentions(seg, m.styles.eventWarning)
				}
				lines = append(lines, m.renderDiffDetailLineWithKind(seg, kind))
			}
		}
	}

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLinesFromOffset(lines, m.state.DetailScroll, height))
	return m.styles.text.Render(pane)
}

func (m *model) styleForTimelineRow(row displayTimelineRow) lipgloss.Style {
	if row.event == nil {
		return m.styles.secondary
	}
	kind := eventKindLabel(*row.event)
	switch kind {
	case "opened", "merged", "closed":
		return m.styles.eventSuccess
	case "requested", "unrequested", "reviewed":
		return m.styles.eventWarning
	case "committed":
		return m.styles.eventInfo
	case "force_push":
		return m.styles.eventDanger
	default:
		return m.styles.text
	}
}

func shouldHighlightDetailDiff(state AppState) bool {
	ev := state.selectedDetailEvent()
	if ev == nil {
		return false
	}
	if selectedDetailIsThreadRoot(state) && ev.Comment != nil && ev.Comment.DiffHunk != nil && strings.TrimSpace(*ev.Comment.DiffHunk) != "" {
		return true
	}
	return ev.Type == "github.timeline.committed" || ev.Type == "github.timeline.head_ref_force_pushed"
}

var mentionPattern = regexp.MustCompile(`@[A-Za-z0-9][A-Za-z0-9-]*`)

func shouldHighlightDetailMentions(state AppState) bool {
	ev := state.selectedDetailEvent()
	if ev == nil {
		return false
	}
	name := timelineEventName(*ev)
	return name == "commented" || name == "line-commented" || ev.Type == "github.review_comment"
}

func highlightDetailMentions(line string, style lipgloss.Style) string {
	return mentionPattern.ReplaceAllStringFunc(line, func(match string) string {
		rendered := style.Render(match)
		if rendered == match {
			return "[" + match + "]"
		}
		return rendered
	})
}

func fitPaneLinesFromOffset(lines []string, offset int, maxLines int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) == 0 {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		offset = len(lines) - 1
	}
	end := offset + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[offset:end], "\n")
}

func (m *model) renderDiffDetailLineWithKind(line string, kind string) string {
	switch kind {
	case "header":
		return m.styles.diffHeader.Render(line)
	case "hunk":
		return m.styles.diffHunk.Render(line)
	case "add":
		return m.styles.diffAdd.Render(line)
	case "del":
		return m.styles.diffDel.Render(line)
	default:
		return line
	}
}

func diffLineKind(line string) string {
	if strings.HasPrefix(line, "diff --") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
		return "header"
	}
	if strings.HasPrefix(line, "@@") {
		return "hunk"
	}
	if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
		return "add"
	}
	if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
		return "del"
	}
	return ""
}

func paneInnerWidth(outerWidth int) int {
	w := outerWidth
	if w < 1 {
		return 1
	}
	return w
}

func panesTotalWidth(totalWidth int, focus focusColumn, mode paneMode) int {
	separators := 1
	leftW, midW, rightW := paneWidths(totalWidth, focus, mode)
	visible := 0
	if leftW > 0 {
		visible++
	}
	if midW > 0 {
		visible++
	}
	if rightW > 0 {
		visible++
	}
	if visible <= 1 {
		separators = 0
	}
	w := totalWidth - separators
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

func paneWidths(totalWidth int, focus focusColumn, mode paneMode) (int, int, int) {
	if totalWidth <= 0 {
		return 1, 1, 1
	}
	if totalWidth == 1 {
		switch focus {
		case focusNotifications:
			return 1, 0, 0
		default:
			return 0, 1, 0
		}
	}

	var first, second int
	switch focus {
	case focusNotifications:
		first, second = twoPaneWidths(totalWidth, 5, 3)
		return first, second, 0
	case focusTimeline:
		first, second = twoPaneWidths(totalWidth, 5, 2)
		return 0, first, second
	case focusThread:
		first, second = twoPaneWidths(totalWidth, 5, 2)
		return 0, first, second
	case focusDetail:
		if mode == paneModeTimelineDetail {
			first, second = twoPaneWidths(totalWidth, 2, 5)
		} else {
			first, second = twoPaneWidths(totalWidth, 3, 5)
		}
		return 0, first, second
	default:
		first, second = twoPaneWidths(totalWidth, 5, 2)
		return 0, first, second
	}
}

func twoPaneWidths(total, r1, r2 int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	sum := r1 + r2
	if sum <= 0 {
		sum = 1
	}
	a := total * r1 / sum
	b := total - a

	if a < 1 {
		a = 1
	}
	if b < 1 {
		b = 1
	}

	for a+b > total {
		if b > 1 {
			b--
			continue
		}
		if a > 1 {
			a--
			continue
		}
		return a, b
	}
	for a+b < total {
		switch {
		case r2 >= r1:
			b++
		default:
			a++
		}
	}

	return a, b
}

func fitPaneLines(lines []string, maxLines int, maxWidth int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	out := make([]string, 0, min(maxLines, len(lines)))
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		for _, part := range parts {
			wrapped := wrapDisplayWidth(part, maxWidth, "")
			if strings.Contains(part, "\x1b[") {
				wrapped = []string{part}
			}
			for _, seg := range wrapped {
				if len(out) >= maxLines {
					return strings.Join(out, "\n")
				}
				out = append(out, seg)
			}
		}
	}
	return strings.Join(out, "\n")
}

func wrapDisplayWidth(s string, maxWidth int, continuationIndent string) []string {
	if maxWidth <= 0 {
		return []string{""}
	}
	if s == "" {
		return []string{""}
	}
	indent := continuationIndent
	indentWidth := lipgloss.Width(indent)
	if indentWidth >= maxWidth {
		indent = ""
		indentWidth = 0
	}
	continuationWidth := maxWidth - indentWidth
	if continuationWidth < 1 {
		continuationWidth = 1
	}

	var lines []string
	remaining := s
	first := true
	for strings.TrimSpace(remaining) != "" {
		lineWidth := maxWidth
		prefix := ""
		if !first {
			lineWidth = continuationWidth
			prefix = indent
		}

		chunk, rest := splitAtDisplayWidth(remaining, lineWidth)
		chunk = strings.TrimRightFunc(chunk, unicode.IsSpace)
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)

		lines = append(lines, chunk)
		if prefix != "" {
			lines[len(lines)-1] = prefix + lines[len(lines)-1]
		}

		remaining = rest
		first = false
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func splitAtDisplayWidth(s string, maxWidth int) (string, string) {
	if maxWidth <= 0 || s == "" {
		return "", s
	}
	used := 0
	lastByte := 0
	lastBreak := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		nextByte := i + len(string(r))
		if used+rw > maxWidth {
			if lastBreak > 0 {
				return s[:lastBreak], s[lastBreak:]
			}
			if lastByte == 0 {
				_, size := runeAtStart(s)
				return s[:size], s[size:]
			}
			return s[:lastByte], s[lastByte:]
		}
		used += rw
		lastByte = nextByte
		if isLogicalBreakRune(r) {
			lastBreak = nextByte
		}
	}
	return s, ""
}

func timelineKindColumnWidth(rows []displayTimelineRow) int {
	width := 9
	for _, row := range rows {
		if row.event == nil {
			continue
		}
		w := lipgloss.Width(eventKindLabel(*row.event))
		if w > width {
			width = w
		}
	}
	if width > 12 {
		return 12
	}
	return width
}

func timelineActorColumnWidth(rows []displayTimelineRow) int {
	width := 0
	for _, row := range rows {
		if row.event == nil {
			continue
		}
		actor := eventActorLabel(*row.event)
		w := lipgloss.Width(actor)
		if w > width {
			width = w
		}
	}
	if width > 16 {
		return 16
	}
	return width
}

func wrapTimelineRow(row displayTimelineRow, ts *timelineState, maxWidth int, kindWidth int, actorWidth int) []string {
	prefix, content, messageOffset := timelineRowPrefixAndContent(row, ts, kindWidth, actorWidth)
	indent := ""
	if messageOffset > 0 {
		indent = timelineContinuationIndent(row, prefix, messageOffset)
	} else if prefix != "" {
		indent = strings.Repeat(" ", lipgloss.Width(prefix))
	}
	return wrapDisplayWidth(prefix+content, maxWidth, indent)
}

func timelineContinuationIndent(row displayTimelineRow, prefix string, messageOffset int) string {
	if messageOffset < 0 {
		messageOffset = 0
	}
	if row.threadID == "" || row.isThreadHeader {
		return strings.Repeat(" ", lipgloss.Width(prefix)+messageOffset)
	}
	if strings.HasPrefix(prefix, "  ├─ ") {
		return "  │  " + strings.Repeat(" ", messageOffset)
	}
	return strings.Repeat(" ", lipgloss.Width(prefix)+messageOffset)
}

func timelineRowPrefixAndContent(row displayTimelineRow, ts *timelineState, kindWidth int, actorWidth int) (string, string, int) {
	if row.event != nil {
		kind := eventKindLabel(*row.event)
		actor := eventActorLabel(*row.event)
		message := truncatePreview(eventPreviewText(*row.event), 96)
		if row.isThreadHeader {
			group := ts.threadByID[row.threadID]
			path := "thread"
			if group != nil {
				path = compactThreadPath(firstNonEmpty(group.path, "thread"))
			}
			if path != "" {
				message = fmt.Sprintf("%s  %s", path, message)
			}
		}
		content, messageOffset := formatTimelineColumns(kindWidth, actorWidth, kind, actor, message)
		return "", content, messageOffset
	}

	label := row.label
	if row.isThreadHeader {
		return "", label, 0
	}
	return "", label, 0
}

func formatTimelineColumns(kindWidth int, actorWidth int, kind, actor, message string) (string, int) {
	if kindWidth < 1 {
		kindWidth = 1
	}
	kindCol := padToDisplayWidth(clampDisplayWidth(kind, kindWidth), kindWidth)
	if actorWidth < 0 {
		actorWidth = 0
	}

	if actorWidth > 0 {
		actorCol := padToDisplayWidth(clampDisplayWidth(actor, actorWidth), actorWidth)
		if message == "" {
			return kindCol + "  " + actorCol, kindWidth + 2 + actorWidth + 2
		}
		return kindCol + "  " + actorCol + "  " + message, kindWidth + 2 + actorWidth + 2
	}

	if message == "" {
		return kindCol, kindWidth + 2
	}
	return kindCol + "  " + message, kindWidth + 2
}

func formatThreadChildColumns(actorWidth int, actor, message string) (string, int) {
	if actorWidth < 0 {
		actorWidth = 0
	}

	if actorWidth > 0 {
		actorCol := padToDisplayWidth(clampDisplayWidth(actor, actorWidth), actorWidth)
		if message == "" {
			return actorCol, actorWidth + 2
		}
		return actorCol + "  " + message, actorWidth + 2
	}

	if actor == "" {
		return message, 0
	}
	if message == "" {
		return actor, lipgloss.Width(actor) + 2
	}
	return actor + "  " + message, lipgloss.Width(actor) + 2
}

func wrapThreadRow(row displayTimelineRow, ts *timelineState, maxWidth int, actorWidth int) []string {
	actor := ""
	message := ""
	if row.event != nil {
		actor = eventActorLabel(*row.event)
		message = truncatePreview(eventPreviewText(*row.event), 96)
	}
	content, messageOffset := formatThreadChildColumns(actorWidth, actor, message)
	prefix := ""
	indent := ""
	if messageOffset > 0 {
		indent += strings.Repeat(" ", messageOffset)
	}
	return wrapDisplayWidth(prefix+content, maxWidth, indent)
}

type paneContent int

const (
	paneContentTimeline paneContent = iota
	paneContentThread
	paneContentDetail
)

func midPaneContent(mode paneMode) paneContent {
	switch mode {
	case paneModeThreadDetail:
		return paneContentThread
	default:
		return paneContentTimeline
	}
}

func rightPaneContent(mode paneMode) paneContent {
	switch mode {
	case paneModeTimelineThread:
		return paneContentThread
	default:
		return paneContentDetail
	}
}

func isLogicalBreakRune(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '/', '\\', '-', '_', ',', '.', ':', ';', ')', ']', '}', '>', '|':
		return true
	default:
		return false
	}
}

func runeAtStart(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return 0, 0
}

func sanitizeForRender(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func renderSelectedLine(style lipgloss.Style, s string, width int) string {
	if width < 1 {
		width = 1
	}
	return style.Width(width).Render(padToDisplayWidth(s, width))
}

func renderSelectedLineWithBase(selected lipgloss.Style, base lipgloss.Style, s string, width int) string {
	if width < 1 {
		width = 1
	}
	padded := padToDisplayWidth(s, width)
	return selected.Width(width).Render(base.Render(padded))
}

func padToDisplayWidth(s string, width int) string {
	current := lipgloss.Width(s)
	if current >= width {
		return s
	}
	return s + strings.Repeat(" ", width-current)
}

func clampDisplayWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	cut, _ := splitAtDisplayWidth(s, width-3)
	return strings.TrimRightFunc(cut, unicode.IsSpace) + "..."
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
