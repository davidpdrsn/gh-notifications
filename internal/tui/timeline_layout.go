package tui

type timelineLayoutRow struct {
	index    int
	selected bool
	lines    []string
}

type timelineViewportPlan struct {
	start    int
	selected int
	avail    int
	rows     []timelineLayoutRow
}

func buildTimelineViewportPlan(ts *timelineState, width int, viewport int, hideRead bool, showContent bool) timelineViewportPlan {
	plan := timelineViewportPlan{
		start:    0,
		selected: 0,
		avail:    paneContentWidthWithRelativeNumbers(width, viewport),
		rows:     nil,
	}
	if plan.avail < 1 {
		plan.avail = 1
	}
	rows := ts.rowsReadyForDisplay(ts.displayRows(hideRead))
	if len(rows) == 0 {
		return plan
	}
	if viewport < 1 {
		viewport = 1
	}
	selected := indexOfTimelineSelection(rows, ts.selectedID)
	if selected < 0 {
		selected = 0
	}
	if selected >= len(rows) {
		selected = len(rows) - 1
	}
	start := ts.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(rows) {
		start = len(rows) - 1
	}

	kindWidth := timelineKindColumnWidth(rows)
	timeWidth := timelineTimeColumnWidth(rows)
	actorWidth := timelineActorColumnWidth(rows)
	lineCount := func(i int) int {
		if i < 0 || i >= len(rows) {
			return 1
		}
		return len(wrapTimelineRow(rows[i], ts, plan.avail, timeWidth, kindWidth, actorWidth, showContent))
	}
	start = clampTimelineScrollToVisibleSelected(start, selected, len(rows), viewport, lineCount)
	layoutRows := make([]timelineLayoutRow, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		layoutRows = append(layoutRows, timelineLayoutRow{
			index:    i,
			selected: i == selected,
			lines:    wrapTimelineRow(rows[i], ts, plan.avail, timeWidth, kindWidth, actorWidth, showContent),
		})
	}
	plan.start = start
	plan.selected = selected
	plan.rows = layoutRows
	return plan
}

func clampTimelineScrollToVisibleSelected(scroll, selected, length, viewport int, lineCount func(int) int) int {
	if length <= 0 {
		return 0
	}
	if viewport < 1 {
		viewport = 1
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= length {
		selected = length - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= length {
		scroll = length - 1
	}
	if selected < scroll {
		scroll = selected
	}

	isVisible := func(start int) bool {
		used := 0
		for i := start; i < length; i++ {
			c := lineCount(i)
			if c < 1 {
				c = 1
			}
			if i == selected {
				return used < viewport
			}
			if used+c > viewport {
				return false
			}
			used += c
			if used >= viewport {
				return false
			}
		}
		return false
	}

	for scroll < selected && !isVisible(scroll) {
		scroll++
	}
	if !isVisible(scroll) {
		scroll = selected
	}
	maxScroll := length - 1
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll
}
