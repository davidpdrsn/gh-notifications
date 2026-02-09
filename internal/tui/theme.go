package tui

import "github.com/charmbracelet/lipgloss"

type theme struct {
	title         lipgloss.Color
	textPrimary   lipgloss.Color
	textSecondary lipgloss.Color
	textMuted     lipgloss.Color
	selectedFg    lipgloss.Color
	selectedBg    lipgloss.Color
	statusFg      lipgloss.Color
	statusBg      lipgloss.Color
	separator     lipgloss.Color
	surface       lipgloss.Color
	info          lipgloss.Color
	success       lipgloss.Color
	warning       lipgloss.Color
	danger        lipgloss.Color
	focus         lipgloss.Color
}

var catppuccinMocha = theme{
	title:         lipgloss.Color("#B4BEFE"),
	textPrimary:   lipgloss.Color("#CDD6F4"),
	textSecondary: lipgloss.Color("#BAC2DE"),
	textMuted:     lipgloss.Color("#7F849C"),
	selectedFg:    lipgloss.Color("#CDD6F4"),
	selectedBg:    lipgloss.Color("#45475A"),
	statusFg:      lipgloss.Color("#94E2D5"),
	statusBg:      lipgloss.Color("#181825"),
	separator:     lipgloss.Color("#6C7086"),
	surface:       lipgloss.Color("#313244"),
	info:          lipgloss.Color("#89B4FA"),
	success:       lipgloss.Color("#A6E3A1"),
	warning:       lipgloss.Color("#F9E2AF"),
	danger:        lipgloss.Color("#F38BA8"),
	focus:         lipgloss.Color("#CBA6F7"),
}
