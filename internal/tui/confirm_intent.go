package tui

import "strings"

func confirmActionDisplayName(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "archive"
	default:
		return "action"
	}
}

func confirmActionPrompt(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "Archive selected?"
	default:
		return "Confirm action?"
	}
}

func confirmActionMarkerLabel(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "archive target"
	default:
		return "target"
	}
}

func confirmIntentTargetSet(intent *confirmIntentState) map[string]bool {
	set := make(map[string]bool)
	if intent == nil {
		return set
	}
	for _, id := range intent.TargetNotifIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			set[id] = true
		}
	}
	if strings.TrimSpace(intent.PrimaryNotifID) != "" {
		set[intent.PrimaryNotifID] = true
	}
	return set
}
