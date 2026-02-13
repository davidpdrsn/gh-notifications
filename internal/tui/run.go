package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gh-pr/ghpr"
	"gh-pr/internal/readstate"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, token string, stdout io.Writer) error {
	client := ghpr.NewClient(token)
	storePath, err := readStateDBPath()
	if err != nil {
		return err
	}
	store, err := readstate.Open(storePath)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	m := newModel(ctx, client, store)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(stdout))
	_, err = p.Run()
	return err
}

func readStateDBPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(configDir, "gh-pr", "state.db"), nil
}
