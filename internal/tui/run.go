package tui

import (
	"context"
	"io"

	"gh-pr/ghpr"

	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, token string, stdout io.Writer) error {
	client := ghpr.NewClient(token)
	m := newModel(ctx, client)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(stdout))
	_, err := p.Run()
	return err
}
