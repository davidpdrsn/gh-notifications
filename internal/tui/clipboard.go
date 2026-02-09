package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardCommand struct {
	name string
	args []string
}

func copyToClipboard(text string) error {
	cmds := clipboardCommands()
	if len(cmds) == 0 {
		return errors.New("clipboard is not supported on this platform")
	}

	errs := make([]string, 0, len(cmds))
	for _, c := range cmds {
		cmd := exec.Command(c.name, c.args...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", c.name, err))
		}
	}

	return fmt.Errorf("unable to copy to clipboard (%s)", strings.Join(errs, "; "))
}

func clipboardCommands() []clipboardCommand {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardCommand{{name: "pbcopy"}}
	case "windows":
		return []clipboardCommand{{name: "clip"}}
	default:
		return []clipboardCommand{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
	}
}
