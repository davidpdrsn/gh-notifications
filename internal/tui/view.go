package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
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
	status := m.styles.status.Width(m.state.Width).AlignHorizontal(lipgloss.Right).Render(m.bottomStatus())
	base := lipgloss.JoinVertical(lipgloss.Left, row, status)
	if m.state.HelpOpen {
		return overlayModalCentered(base, m.renderHelpModal(), m.state.Width, m.state.Height)
	}
	return base
}

func (m *model) renderHelpModal() string {
	title := m.styles.title.Render("Keyboard shortcuts")
	navigation := m.renderHelpBindingSection("Navigation", []helpBinding{
		{key: "j/k, up/down", desc: "move selection"},
		{key: "h/l, left/right, enter", desc: "drill in / back out"},
		{key: "[ / ]", desc: "jump top / bottom"},
		{key: "tab / shift+tab", desc: "cycle notification org tabs"},
	})
	actions := m.renderHelpBindingSection("Actions", []helpBinding{
		{key: "o", desc: "open selected item in browser"},
		{key: "r", desc: "toggle read state"},
		{key: "space", desc: "mark/unmark row and move down"},
		{key: "alt+space", desc: "mark/unmark row and move up"},
		{key: "shift+a", desc: "mark/unmark all in current list"},
		{key: "a", desc: "archive notification (a then a confirms)"},
		{key: "C", desc: "copy focused column"},
	})
	view := m.renderHelpBindingSection("View & scroll", []helpBinding{
		{key: "H", desc: "hide/show read items"},
		{key: "ctrl+u / ctrl+d", desc: "page up/down"},
		{key: "ctrl+p / ctrl+n", desc: "scroll detail pane"},
		{key: "ctrl+r", desc: "refresh now"},
	})

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		navigation,
		"",
		actions,
		"",
		view,
		"",
		m.styles.muted.Render("Press ? or esc to close."),
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(1, 2).
		MaxWidth(max(44, m.state.Width-4)).
		Render(content)
	return box
}

type helpBinding struct {
	key  string
	desc string
}

func (m *model) renderHelpBindingSection(title string, bindings []helpBinding) string {
	keyWidth := 0
	for _, b := range bindings {
		if w := lipgloss.Width(b.key); w > keyWidth {
			keyWidth = w
		}
	}
	lines := make([]string, 0, len(bindings)+1)
	lines = append(lines, m.styles.secondary.Render(title))
	for _, b := range bindings {
		key := padToDisplayWidth(b.key, keyWidth)
		line := "  " + key + "  " + b.desc
		lines = append(lines, m.styles.text.Render(line))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func overlayModalCentered(base, modal string, width, height int) string {
	if strings.TrimSpace(modal) == "" {
		return base
	}
	baseLines := strings.Split(base, "\n")
	if len(baseLines) < height {
		for len(baseLines) < height {
			baseLines = append(baseLines, "")
		}
	}
	for i := range baseLines {
		baseLines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Left, baseLines[i])
	}
	modalLines := strings.Split(modal, "\n")
	modalWidth := 0
	for _, line := range modalLines {
		if w := xansi.StringWidth(line); w > modalWidth {
			modalWidth = w
		}
	}
	if modalWidth > width {
		modalWidth = width
	}
	x := (width - modalWidth) / 2
	if x < 0 {
		x = 0
	}
	y := (height - len(modalLines)) / 2
	if y < 0 {
		y = 0
	}
	for i, line := range modalLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		left := xansi.Cut(baseLines[row], 0, x)
		overlay := lipgloss.PlaceHorizontal(modalWidth, lipgloss.Left, line)
		right := xansi.Cut(baseLines[row], x+modalWidth, width)
		baseLines[row] = left + overlay + right
	}
	if len(baseLines) > height && height > 0 {
		baseLines = baseLines[:height]
	}
	return strings.Join(baseLines, "\n")
}

func confirmPrimaryOverlayText(kind confirmActionKind, more int) string {
	text := fmt.Sprintf("<< %s >> [a confirm | esc cancel]", confirmActionDisplayName(kind))
	if more > 0 {
		text += fmt.Sprintf(" +%d", more)
	}
	return text
}

func overlayLineLeft(base, overlay string, width int, tailStyle lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	base = lipgloss.PlaceHorizontal(width, lipgloss.Left, base)
	overlayW := xansi.StringWidth(overlay)
	if overlayW <= 0 {
		return base
	}
	if overlayW >= width {
		return xansi.Cut(overlay, 0, width)
	}
	plainBase := stripANSIEscapes(base)
	if xansi.StringWidth(plainBase) < width {
		plainBase = lipgloss.PlaceHorizontal(width, lipgloss.Left, plainBase)
	}
	gap := 1
	if overlayW+gap > width {
		gap = 0
	}
	tailStart := overlayW + gap
	tail := ""
	if tailStart < width {
		tail = tailStyle.Render(xansi.Cut(plainBase, tailStart, width))
	}
	if gap == 0 {
		return overlay + tail
	}
	return overlay + " " + tail
}

func stripANSIEscapes(s string) string {
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansi.ReplaceAllString(s, "")
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
	rowIndexByLine := make([]int, 0, max(1, height))
	selectedRow := -1
	lines = append(lines, m.renderNotificationTabs(contentWidth(width)))
	rowIndexByLine = append(rowIndexByLine, -1)
	if m.state.NotifErr != "" {
		lines = append(lines, m.styles.error.Render("error: "+m.state.NotifErr))
		rowIndexByLine = append(rowIndexByLine, -1)
	}
	visible := m.state.visibleNotifications()
	if len(visible) == 0 && !m.state.NotifLoading && m.state.NotifErr == "" && len(m.state.Notifications) == 0 {
		lines = append(lines, m.styles.muted.Render("no notifications"))
		rowIndexByLine = append(rowIndexByLine, -1)
	}

	selected := m.state.NotifSelected
	start := m.state.NotifScroll
	end := start + max(1, height-1)
	if end > len(visible) {
		end = len(visible)
	}
	timeColWidth := notificationTimeColumnWidth(visible)
	kindColWidth := notificationKindColumnWidthForState(m.state, visible)
	authorColWidth := notificationAuthorColumnWidth(visible)
	repoColWidth := notificationRepoColumnWidth(visible)
	confirmTargets := confirmIntentTargetSet(m.state.ConfirmIntent)
	primaryConfirmRow := -1
	if len(confirmTargets) > 0 {
		preferred := ""
		if m.state.ConfirmIntent != nil {
			preferred = strings.TrimSpace(m.state.ConfirmIntent.PrimaryNotifID)
		}
		for i := start; i < end; i++ {
			if !confirmTargets[visible[i].id] {
				continue
			}
			if primaryConfirmRow < 0 {
				primaryConfirmRow = i
			}
			if preferred != "" && visible[i].id == preferred {
				primaryConfirmRow = i
				break
			}
		}
	}
	moreConfirmTargets := 0
	if primaryConfirmRow >= 0 && len(confirmTargets) > 0 {
		moreConfirmTargets = len(confirmTargets) - 1
		if moreConfirmTargets < 0 {
			moreConfirmTargets = 0
		}
	}
	for i := start; i < end; i++ {
		n := visible[i]
		marker := m.state.notificationUnreadMarker(n)
		waitingOnMe := strings.TrimSpace(n.kind) == "pr" && m.state.ReviewReqByRef[n.ref]
		draftPR := strings.TrimSpace(n.kind) == "pr" && m.state.ReviewReqDraftByRef[n.ref]
		prefix := marker + padToDisplayWidth(timeAgo(n.updatedAt), timeColWidth) + " "
		kind := padToDisplayWidth(notificationKindLabelForNotification(m.state, n), kindColWidth)
		author := padToDisplayWidth(clampDisplayWidth(oneLine(n.author), authorColWidth), authorColWidth)
		repo := padToDisplayWidth(clampDisplayWidth(oneLine(n.repo), repoColWidth), repoColWidth)
		label := prefix + kind + " "
		label += author + " "
		label += repo + "  " + oneLine(n.title)
		avail := paneContentWidthWithRelativeNumbers(width, height)
		if avail < 1 {
			avail = 1
		}
		indentWidth := lipgloss.Width(prefix) + kindColWidth + 1 + repoColWidth + 2
		indentWidth += authorColWidth + 1
		minContinuationWidth := 12
		maxIndent := avail - minContinuationWidth
		if maxIndent < 0 {
			maxIndent = 0
		}
		if indentWidth > maxIndent {
			indentWidth = maxIndent
		}
		titleIndent := strings.Repeat(" ", indentWidth)
		wrapped := wrapDisplayWidth(label, avail, titleIndent)
		marked := m.state.MarkedNotifications[n.id]
		current := i == selected
		if current {
			selectedRow = i
		}
		rendered := make([]string, 0, len(wrapped))
		if current || marked {
			for _, seg := range wrapped {
				rendered = append(rendered, m.renderNotificationStyledLine(seg, avail, timeColWidth+5, kindColWidth, n.kind, waitingOnMe, draftPR, current, marked))
			}
		} else {
			for _, seg := range wrapped {
				rendered = append(rendered, m.renderNotificationStyledLine(seg, avail, timeColWidth+5, kindColWidth, n.kind, waitingOnMe, draftPR, false, false))
			}
		}
		if confirmTargets[n.id] && len(rendered) > 0 {
			if i == primaryConfirmRow {
				overlay := m.styles.confirmOverlay.Render(clampDisplayWidth(confirmPrimaryOverlayText(m.state.ConfirmIntent.Kind, moreConfirmTargets), avail))
				rendered[0] = overlayLineLeft(rendered[0], overlay, avail, m.styles.muted.Strikethrough(true))
			} else {
				overlay := m.styles.muted.Render(clampDisplayWidth("<< "+confirmActionMarkerLabel(m.state.ConfirmIntent.Kind)+" >>", avail))
				rendered[0] = overlayLineLeft(rendered[0], overlay, avail, m.styles.muted.Strikethrough(true))
			}
		}
		for _, line := range rendered {
			lines = append(lines, line)
			rowIndexByLine = append(rowIndexByLine, i)
		}
	}
	lines = m.applyRelativeLineNumbers(lines, rowIndexByLine, selectedRow, height)

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
	return m.styles.text.Render(pane)
}

func (m *model) renderNotificationTabs(width int) string {
	if width < 1 {
		width = 1
	}
	tabs := m.state.notificationTabs()
	active := m.state.activeNotificationTab()

	type tabLabel struct {
		name  string
		label string
		width int
	}

	labels := make([]tabLabel, 0, len(tabs))
	for _, tab := range tabs {
		name := tab
		maxNameWidth := width - 2
		if maxNameWidth < 1 {
			maxNameWidth = 1
		}
		name = clampDisplayWidth(name, maxNameWidth)
		label := " " + name + " "
		labels = append(labels, tabLabel{name: tab, label: label, width: lipgloss.Width(label)})
	}

	activeIdx := 0
	for i := range labels {
		if labels[i].name == active {
			activeIdx = i
			break
		}
	}

	start := activeIdx
	end := activeIdx + 1
	used := labels[activeIdx].width
	for {
		added := false
		if start > 0 {
			w := labels[start-1].width + 1
			if used+w <= width {
				start--
				used += w
				added = true
			}
		}
		if end < len(labels) {
			w := labels[end].width + 1
			if used+w <= width {
				end++
				used += w
				added = true
			}
		}
		if !added {
			break
		}
	}

	rendered := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		if labels[i].name == active {
			rendered = append(rendered, m.styles.tabActive.Render(labels[i].label))
		} else {
			rendered = append(rendered, m.styles.tab.Render(labels[i].label))
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

func notificationAuthorColumnWidth(rows []notifRow) int {
	width := 12
	for i := range rows {
		w := lipgloss.Width(oneLine(rows[i].author))
		if w > width {
			width = w
		}
	}
	if width > 20 {
		return 20
	}
	return width
}

func notificationKindLabel(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "pull_request", "pull-request", "pr":
		return "pr"
	case "issue", "is":
		return "is"
	default:
		return "??"
	}
}

func notificationKindLabelForNotification(state AppState, n notifRow) string {
	if strings.TrimSpace(strings.ToLower(n.kind)) == "pr" && state.ReviewReqDraftByRef[n.ref] {
		return "draft"
	}
	return notificationKindLabel(n.kind)
}

func notificationKindColumnWidth(rows []notifRow) int {
	width := 2
	for i := range rows {
		w := lipgloss.Width(notificationKindLabel(rows[i].kind))
		if w > width {
			width = w
		}
	}
	if width < 2 {
		width = 2
	}
	if width > 2 {
		return width
	}
	return width
}

func notificationKindColumnWidthForState(state AppState, rows []notifRow) int {
	width := 2
	for i := range rows {
		w := lipgloss.Width(notificationKindLabelForNotification(state, rows[i]))
		if w > width {
			width = w
		}
	}
	if width < 2 {
		width = 2
	}
	if width > 8 {
		return 8
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

func (m *model) renderNotificationStyledLine(line string, width int, timeWidth int, kindWidth int, kind string, waitingOnMe bool, draftPR bool, current bool, marked bool) string {
	if width < 1 {
		width = 1
	}
	padded := padToDisplayWidth(line, width)
	prefix, rest := splitAtDisplayWidth(padded, timeWidth)
	renderRest := func(content string) string {
		if kindWidth <= 0 {
			if current {
				return m.styles.current.Render(content)
			}
			if marked {
				return m.styles.selected.Render(content)
			}
			return content
		}
		kindCol, tail := splitAtExactDisplayWidth(content, kindWidth)
		sep, body := splitAtExactDisplayWidth(tail, 1)
		kindStyle := m.kindStyle(kindCol, waitingOnMe, draftPR, current, marked)
		kindRendered := kindStyle.Render(kindCol)
		if current {
			return kindRendered + m.styles.current.Render(sep+body)
		}
		if marked {
			return kindRendered + m.styles.selected.Render(sep+body)
		}
		return kindRendered + sep + body
	}

	if current {
		if marker, remainder, ok := splitUnreadMarkerPrefix(prefix); ok {
			return m.notificationMarkerStyle(marker, current, marked).Render(marker) +
				m.styles.currentMuted.Render(remainder) +
				renderRest(rest)
		}
		return m.styles.currentMuted.Render(prefix) + renderRest(rest)
	}
	if marked {
		if marker, remainder, ok := splitUnreadMarkerPrefix(prefix); ok {
			return m.notificationMarkerStyle(marker, current, marked).Render(marker) +
				m.styles.selectedMuted.Render(remainder) +
				renderRest(rest)
		}
		return m.styles.selectedMuted.Render(prefix) + renderRest(rest)
	}

	if marker, remainder, ok := splitUnreadMarkerPrefix(prefix); ok {
		return m.notificationMarkerStyle(marker, current, marked).Render(marker) + m.styles.muted.Render(remainder) + renderRest(rest)
	}
	return m.styles.muted.Render(prefix) + renderRest(rest)
}

func (m *model) notificationMarkerStyle(marker string, current bool, marked bool) lipgloss.Style {
	if strings.Contains(marker, "+") {
		if marked {
			return m.styles.selectedMuted
		}
		if current {
			return m.styles.currentMuted
		}
		return m.styles.muted
	}
	if marked {
		return m.styles.unreadSelected
	}
	if current {
		return m.styles.unreadCurrent
	}
	return m.styles.unreadMarker
}

func (m *model) kindStyle(kind string, waitingOnMe bool, draftPR bool, current bool, marked bool) lipgloss.Style {
	normalized := strings.TrimSpace(strings.ToLower(kind))
	if waitingOnMe {
		if current {
			return m.styles.kindPRWaitCur
		}
		if marked {
			return m.styles.kindPRWaitSel
		}
		return m.styles.kindPRWaiting
	}
	if draftPR {
		if current {
			return m.styles.kindPRDraftCur
		}
		if marked {
			return m.styles.kindPRDraftSel
		}
		return m.styles.kindPRDraft
	}
	if current {
		switch normalized {
		case "pr":
			return m.styles.kindPRCurrent
		case "is":
			return m.styles.kindISCurrent
		default:
			return m.styles.kindUnkCurrent
		}
	}
	if marked {
		switch normalized {
		case "pr":
			return m.styles.kindPRSelected
		case "is":
			return m.styles.kindISSelected
		default:
			return m.styles.kindUnkSelected
		}
	}
	switch normalized {
	case "pr":
		return m.styles.kindPR
	case "is":
		return m.styles.kindIS
	default:
		return m.styles.kindUnknown
	}
}

func (m *model) renderTimeline(width, height int) string {
	lines := make([]string, 0, max(1, height))
	rowIndexByLine := make([]int, 0, max(1, height))
	selectedRow := -1
	highlightSelection := m.state.Focus == focusTimeline
	ts := m.state.currentTimeline()
	if ts == nil {
		lines = append(lines, m.styles.muted.Render("select a notification"))
		rowIndexByLine = append(rowIndexByLine, -1)
	} else {
		if ts.err != "" {
			lines = append(lines, m.styles.error.Render("error: "+ts.err))
			rowIndexByLine = append(rowIndexByLine, -1)
		}
		allRows := ts.displayRows(m.state.HideRead)
		rows := ts.rowsReadyForDisplay(allRows)
		pendingReadState := ts.hasPendingReadState(allRows)
		if len(rows) == 0 && !ts.loading && ts.err == "" {
			if pendingReadState {
				lines = append(lines, m.styles.muted.Render("loading read state..."))
				rowIndexByLine = append(rowIndexByLine, -1)
			} else if m.state.HideRead {
				lines = append(lines, m.styles.muted.Render("all events are read"))
				rowIndexByLine = append(rowIndexByLine, -1)
			} else {
				lines = append(lines, m.styles.muted.Render("no timeline events"))
				rowIndexByLine = append(rowIndexByLine, -1)
			}
		}
		showContent := m.state.Focus == focusTimeline
		plan := buildTimelineViewportPlan(ts, width, max(1, height-2), m.state.HideRead, showContent)
		selectedRow = plan.selected
		timeWidth := timelineTimeColumnWidth(rows)
		kindWidth := timelineKindColumnWidth(rows)
		leadWidth := timeWidth + 2 + kindWidth
		for _, row := range plan.rows {
			isRead := ts.rowRead(rows[row.index])
			marked := m.state.MarkedTimelineByRef[m.state.CurrentRef][rows[row.index].id]
			current := highlightSelection && row.selected
			if current || marked {
				wrapped := row.lines
				style := m.styleForTimelineRow(ts, rows[row.index])
				for _, seg := range wrapped {
					lines = append(lines, m.renderTimelineStyledLine(style, seg, plan.avail, current, marked, leadWidth, timeWidth, isRead))
					rowIndexByLine = append(rowIndexByLine, row.index)
				}
			} else {
				style := m.styleForTimelineRow(ts, rows[row.index])
				for _, seg := range row.lines {
					lines = append(lines, m.renderTimelineStyledLine(style, seg, plan.avail, false, false, leadWidth, timeWidth, isRead))
					rowIndexByLine = append(rowIndexByLine, row.index)
				}
			}
		}
	}
	lines = m.applyRelativeLineNumbers(lines, rowIndexByLine, selectedRow, height)

	innerW := paneInnerWidth(width)
	pane := lipgloss.NewStyle().Width(innerW).Height(height).Render(fitPaneLines(lines, height, contentWidth(width)))
	return m.styles.text.Render(pane)
}

func (m *model) renderThread(width, height int) string {
	lines := make([]string, 0, max(1, height))
	rowIndexByLine := make([]int, 0, max(1, height))
	selectedRow := -1
	highlightSelection := m.state.Focus == focusThread
	ts := m.state.currentTimeline()
	if ts == nil || ts.activeThreadID == "" {
		lines = append(lines, m.styles.muted.Render("no thread selected"))
		rowIndexByLine = append(rowIndexByLine, -1)
	} else {
		allRows := ts.threadRows(ts.activeThreadID, m.state.HideRead)
		rows := ts.rowsReadyForDisplay(allRows)
		pendingReadState := ts.hasPendingReadState(allRows)
		if len(rows) == 0 {
			if pendingReadState {
				lines = append(lines, m.styles.muted.Render("loading read state..."))
				rowIndexByLine = append(rowIndexByLine, -1)
			} else if m.state.HideRead {
				lines = append(lines, m.styles.muted.Render("all thread events are read"))
				rowIndexByLine = append(rowIndexByLine, -1)
			} else {
				lines = append(lines, m.styles.muted.Render("no replies"))
				rowIndexByLine = append(rowIndexByLine, -1)
			}
		} else {
			avail := paneContentWidthWithRelativeNumbers(width, height)
			if avail < 1 {
				avail = 1
			}
			timeWidth := timelineTimeColumnWidth(rows)
			actorWidth := timelineActorColumnWidth(rows)
			start := ts.threadScrollOffset
			if start < 0 {
				start = 0
			}
			if start >= len(rows) {
				start = len(rows) - 1
			}
			for i := start; i < len(rows); i++ {
				wrapped := wrapThreadRow(rows[i], ts, avail, timeWidth, actorWidth)
				isRead := ts.rowRead(rows[i])
				if i == ts.threadSelectedIndex {
					selectedRow = i
				}
				marked := m.state.MarkedThreadByRef[m.state.CurrentRef][rows[i].id]
				current := highlightSelection && i == ts.threadSelectedIndex
				if current || marked {
					style := m.styleForTimelineRow(ts, rows[i])
					for _, seg := range wrapped {
						lines = append(lines, m.renderTimelineStyledLine(style, seg, avail, current, marked, timeWidth, timeWidth, isRead))
						rowIndexByLine = append(rowIndexByLine, i)
					}
				} else {
					style := m.styleForTimelineRow(ts, rows[i])
					for _, seg := range wrapped {
						lines = append(lines, m.renderTimelineStyledLine(style, seg, avail, false, false, timeWidth, timeWidth, isRead))
						rowIndexByLine = append(rowIndexByLine, i)
					}
				}
			}
		}
	}
	lines = m.applyRelativeLineNumbers(lines, rowIndexByLine, selectedRow, height)

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

func (m *model) styleForTimelineRow(ts *timelineState, row displayTimelineRow) lipgloss.Style {
	if ts != nil && ts.rowRead(row) {
		return m.styles.muted
	}
	if row.event == nil {
		return m.styles.secondary
	}
	kind := eventKindLabel(*row.event)
	switch kind {
	case "opened", "merged", "closed":
		return m.styles.eventSuccess
	case "review_requested", "review_request_removed", "reviewed":
		return m.styles.eventWarning
	case "committed":
		return m.styles.eventInfo
	case "force_pushed":
		return m.styles.eventDanger
	default:
		return m.styles.text
	}
}

func (m *model) renderTimelineStyledLine(base lipgloss.Style, line string, width int, current bool, marked bool, kindWidth int, timeWidth int, read bool) string {
	if width < 1 {
		width = 1
	}
	padded := padToDisplayWidth(line, width)
	renderTimestamp := func(s string) string {
		if current {
			return m.styles.currentMuted.Render(s)
		}
		if marked {
			return m.styles.selectedMuted.Render(s)
		}
		return m.styles.muted.Render(s)
	}
	renderRest := func(rest string) string {
		if kindWidth > 0 {
			kindCol, tail := splitAtExactDisplayWidth(rest, kindWidth)
			if timeWidth > 0 {
				tsPart, restLead := splitAtExactDisplayWidth(kindCol, timeWidth)
				tsSep, restLeadTail := splitAtExactDisplayWidth(restLead, 2)
				body := restLeadTail + tail
				if current {
					if read {
						return renderTimestamp(tsPart) + m.styles.currentMuted.Render(tsSep) + m.styles.currentMuted.Render(body)
					}
					return renderTimestamp(tsPart) + m.styles.currentMuted.Render(tsSep) + m.styles.current.Render(base.Render(body))
				}
				if marked {
					if read {
						return renderTimestamp(tsPart) + m.styles.selectedMuted.Render(tsSep) + m.styles.selectedMuted.Render(body)
					}
					return renderTimestamp(tsPart) + m.styles.selectedMuted.Render(tsSep) + m.styles.selected.Render(base.Render(body))
				}
				if read {
					return renderTimestamp(tsPart) + m.styles.muted.Render(tsSep) + m.styles.muted.Render(body)
				}
				return renderTimestamp(tsPart) + m.styles.muted.Render(tsSep) + base.Render(body)
			}
			if current {
				if read {
					return m.styles.current.Render(base.Render(kindCol)) + m.styles.currentMuted.Render(tail)
				}
				return m.styles.current.Render(base.Render(kindCol)) + m.styles.current.Render(tail)
			}
			if marked {
				if read {
					return m.styles.selected.Render(base.Render(kindCol)) + m.styles.selectedMuted.Render(tail)
				}
				return m.styles.selected.Render(base.Render(kindCol)) + m.styles.selected.Render(tail)
			}
			if read {
				return base.Render(kindCol) + m.styles.muted.Render(tail)
			}
			return base.Render(kindCol) + tail
		}

		if current {
			if read {
				return m.styles.currentMuted.Render(rest)
			}
			return m.styles.current.Render(base.Render(rest))
		}
		if marked {
			if read {
				return m.styles.selectedMuted.Render(rest)
			}
			return m.styles.selected.Render(base.Render(rest))
		}
		if read {
			return m.styles.muted.Render(rest)
		}
		return base.Render(rest)
	}

	if marker, rest, ok := splitUnreadMarkerPrefix(padded); ok {
		if current {
			return m.styles.unreadCurrent.Render(marker) + renderRest(rest)
		}
		if marked {
			return m.styles.unreadSelected.Render(marker) + renderRest(rest)
		}
		return m.styles.unreadMarker.Render(marker) + renderRest(rest)
	}

	return renderRest(padded)
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

func relativeLineNumberGutterWidth(maxLines int) int {
	if maxLines < 1 {
		maxLines = 1
	}
	digits := len(fmt.Sprintf("%d", maxLines-1))
	if digits < 1 {
		digits = 1
	}
	return digits + 1
}

func paneContentWidthWithRelativeNumbers(outerWidth int, maxLines int) int {
	w := contentWidth(outerWidth) - relativeLineNumberGutterWidth(maxLines)
	if w < 1 {
		return 1
	}
	return w
}

func (m *model) applyRelativeLineNumbers(lines []string, rowIndexByLine []int, selectedRow int, maxLines int) []string {
	if len(lines) == 0 {
		return lines
	}
	if len(rowIndexByLine) != len(lines) {
		return lines
	}
	if selectedRow < 0 {
		for _, row := range rowIndexByLine {
			if row >= 0 {
				selectedRow = row
				break
			}
		}
	}
	if selectedRow < 0 {
		return lines
	}
	gutterWidth := relativeLineNumberGutterWidth(maxLines)
	digits := gutterWidth - 1
	out := make([]string, 0, len(lines))
	prevRow := -2
	for i, line := range lines {
		row := rowIndexByLine[i]
		if row < 0 {
			out = append(out, line)
			prevRow = -2
			continue
		}
		if row == prevRow {
			blank := strings.Repeat(" ", digits) + " "
			out = append(out, m.styles.lineNumber.Render(blank)+line)
			continue
		}
		n := row - selectedRow
		if n < 0 {
			n = -n
		}
		label := padToDisplayWidth(fmt.Sprintf("%d", n), digits) + " "
		if row == selectedRow {
			out = append(out, m.styles.lineNumberZero.Render(label)+line)
			prevRow = row
			continue
		}
		out = append(out, m.styles.lineNumber.Render(label)+line)
		prevRow = row
	}
	return out
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

func splitAtExactDisplayWidth(s string, maxWidth int) (string, string) {
	if maxWidth <= 0 || s == "" {
		return "", s
	}
	used := 0
	lastByte := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		nextByte := i + len(string(r))
		if used+rw > maxWidth {
			if lastByte == 0 {
				_, size := runeAtStart(s)
				return s[:size], s[size:]
			}
			return s[:lastByte], s[lastByte:]
		}
		used += rw
		lastByte = nextByte
	}
	return s, ""
}

func splitUnreadMarkerPrefix(s string) (string, string, bool) {
	for _, marker := range []string{" !  ", " +  ", " ●  ", " ◐  ", "! ", "+ ", "● ", "◐ "} {
		if strings.HasPrefix(s, marker) {
			return marker, strings.TrimPrefix(s, marker), true
		}
	}
	return "", s, false
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
	return width
}

func timelineTimeColumnWidth(rows []displayTimelineRow) int {
	width := 3
	for _, row := range rows {
		if row.event == nil {
			continue
		}
		w := lipgloss.Width(timeAgo(row.event.OccurredAt))
		if w > width {
			width = w
		}
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

func wrapTimelineRow(row displayTimelineRow, ts *timelineState, maxWidth int, timeWidth int, kindWidth int, actorWidth int, showContent bool) []string {
	prefix, content, messageOffset := timelineRowPrefixAndContent(row, ts, timeWidth, kindWidth, actorWidth, showContent)
	if !showContent {
		if maxWidth < 1 {
			maxWidth = 1
		}
		return []string{clampDisplayWidth(prefix+content, maxWidth)}
	}
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

func timelineRowPrefixAndContent(row displayTimelineRow, ts *timelineState, timeWidth int, kindWidth int, actorWidth int, showContent bool) (string, string, int) {
	marker := " ●  "
	if ts != nil {
		marker = ts.rowUnreadMarker(row)
	}
	if !showContent {
		when := "?"
		kind := "event"
		actor := ""
		if row.event != nil {
			when = timeAgo(row.event.OccurredAt)
			kind = eventKindLabel(*row.event)
			actor = eventActorLabel(*row.event)
		} else if row.isThreadHeader {
			kind = "thread"
		}
		timeCol := padToDisplayWidth(when, max(1, timeWidth))
		kindCol := padToDisplayWidth(kind, max(1, kindWidth))
		parts := []string{timeCol, kindCol}
		if actorWidth > 0 && actor != "" {
			parts = append(parts, padToDisplayWidth(clampDisplayWidth(actor, actorWidth), actorWidth))
		}
		content := stringsJoin(parts, "  ")
		return marker, content, 0
	}
	if row.event != nil {
		when := timeAgo(row.event.OccurredAt)
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
		content, messageOffset := formatTimelineColumns(timeWidth, kindWidth, actorWidth, when, kind, actor, message)
		return marker, content, messageOffset
	}

	label := row.label
	if row.isThreadHeader {
		return marker, label, 0
	}
	return marker, label, 0
}

func formatTimelineColumns(timeWidth int, kindWidth int, actorWidth int, when, kind, actor, message string) (string, int) {
	if timeWidth < 1 {
		timeWidth = 1
	}
	timeCol := padToDisplayWidth(when, timeWidth)
	if kindWidth < 1 {
		kindWidth = 1
	}
	kindCol := padToDisplayWidth(kind, kindWidth)
	if actorWidth < 0 {
		actorWidth = 0
	}

	if actorWidth > 0 {
		actorCol := padToDisplayWidth(clampDisplayWidth(actor, actorWidth), actorWidth)
		if message == "" {
			return timeCol + "  " + kindCol + "  " + actorCol, timeWidth + 2 + kindWidth + 2 + actorWidth + 2
		}
		return timeCol + "  " + kindCol + "  " + actorCol + "  " + message, timeWidth + 2 + kindWidth + 2 + actorWidth + 2
	}

	if message == "" {
		return timeCol + "  " + kindCol, timeWidth + 2 + kindWidth + 2
	}
	return timeCol + "  " + kindCol + "  " + message, timeWidth + 2 + kindWidth + 2
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

func wrapThreadRow(row displayTimelineRow, ts *timelineState, maxWidth int, timeWidth int, actorWidth int) []string {
	when := "?"
	actor := ""
	message := ""
	if row.event != nil {
		when = timeAgo(row.event.OccurredAt)
		actor = eventActorLabel(*row.event)
		message = truncatePreview(eventPreviewText(*row.event), 96)
	}
	timeCol := padToDisplayWidth(when, max(1, timeWidth))
	content, messageOffset := formatThreadChildColumns(actorWidth, actor, message)
	lead := timeCol + "  "
	prefix := " ●  "
	if ts != nil {
		prefix = ts.rowUnreadMarker(row)
	}
	indent := strings.Repeat(" ", lipgloss.Width(lead))
	if messageOffset > 0 {
		indent += strings.Repeat(" ", messageOffset)
	}
	return wrapDisplayWidth(prefix+lead+content, maxWidth, indent)
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
	days := int(d.Hours() / 24)
	if days >= 365 {
		years := days / 365
		remainingDays := days % 365
		return fmt.Sprintf("%dy %dd", years, remainingDays)
	}
	return fmt.Sprintf("%dd", days)
}

func (m *model) String() string {
	return m.View()
}

var _ tea.Model = &model{}
