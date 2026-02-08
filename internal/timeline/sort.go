package timeline

import (
	"sort"

	"gh-pr/internal/timelineapi"
)

func sortEvents(events []timelineapi.Event) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].OccurredAt.Before(events[j].OccurredAt)
	})
}
