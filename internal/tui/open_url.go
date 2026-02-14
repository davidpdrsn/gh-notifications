package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type openURLCommand struct {
	name string
	args []string
}

func openURL(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("empty URL")
	}

	cmds := openURLCommands(target)
	if len(cmds) == 0 {
		return errors.New("opening URLs is not supported on this platform")
	}

	errParts := make([]string, 0, len(cmds))
	for _, c := range cmds {
		cmd := exec.Command(c.name, c.args...)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			errParts = append(errParts, fmt.Sprintf("%s: %v", c.name, err))
		}
	}

	return fmt.Errorf("unable to open URL (%s)", strings.Join(errParts, "; "))
}

func openURLCommands(target string) []openURLCommand {
	switch runtime.GOOS {
	case "darwin":
		return []openURLCommand{{name: "open", args: []string{target}}}
	case "windows":
		return []openURLCommand{{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", target}}}
	default:
		return []openURLCommand{
			{name: "xdg-open", args: []string{target}},
			{name: "gio", args: []string{"open", target}},
		}
	}
}
