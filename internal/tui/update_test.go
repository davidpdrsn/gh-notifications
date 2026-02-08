package tui

import (
	"context"
	"testing"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateKeyMsgDoesNotSpawnAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""))

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatalf("expected nil cmd for key message")
	}
}

func TestUpdateAsyncMsgRearmsAsyncWaiter(t *testing.T) {
	m := newModel(context.Background(), ghpr.NewClient(""))

	_, cmd := m.Update(notifDoneMsg{gen: m.state.NotifGen})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd for async message")
	}
}
