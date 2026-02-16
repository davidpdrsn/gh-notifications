package tui

import "strings"

func confirmActionDisplayName(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "archive"
	case confirmActionUnsubscribe:
		return "unsubscribe"
	default:
		return "action"
	}
}

func confirmActionPrompt(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "Archive selected?"
	case confirmActionUnsubscribe:
		return "Unsubscribe selected?"
	default:
		return "Confirm action?"
	}
}

func confirmActionMarkerLabel(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "archive target"
	case confirmActionUnsubscribe:
		return "unsubscribe target"
	default:
		return "target"
	}
}

func confirmActionKey(kind confirmActionKind) string {
	switch kind {
	case confirmActionArchive:
		return "a"
	case confirmActionUnsubscribe:
		return "u"
	default:
		return "enter"
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
