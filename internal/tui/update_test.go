package tui

import (
	"context"
	"testing"
	"time"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateKeyMsgDoesNotSpawnAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""), nil)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for key message")
	}
}

func TestUpdateAsyncMsgRearmsAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""), nil)

	_, cmd := m.Update(notifDoneMsg{gen: m.state.NotifGen})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for async message")
	}
}

func TestShouldSkipArchivedNotification(t *testing.T) {
	archivedAt := time.Now().UTC().Add(-time.Minute)
	archived := map[string]time.Time{"42": archivedAt}

	if !shouldSkipArchivedNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt}, archived) {
		t.Fatalf("expected archived notification with same updated_at to be skipped")
	}
	if shouldSkipArchivedNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt.Add(time.Second)}, archived) {
		t.Fatalf("expected newer notification not to be skipped")
	}
}

func TestShouldUnarchiveNotification(t *testing.T) {
	archivedAt := time.Now().UTC().Add(-time.Minute)
	archived := map[string]time.Time{"42": archivedAt}

	if shouldUnarchiveNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt}, archived) {
		t.Fatalf("expected same timestamp not to unarchive")
	}
	if !shouldUnarchiveNotification(ghpr.NotificationEvent{ID: "42", UpdatedAt: archivedAt.Add(time.Second)}, archived) {
		t.Fatalf("expected newer timestamp to unarchive")
	}
}
