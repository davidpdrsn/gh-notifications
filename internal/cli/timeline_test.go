package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunTimelineWithoutArgsShowsHelpTextOnStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"timeline"}, &stdout, &stderr)
	if exitCode != ExitParse {
		t.Fatalf("expected exit code %d, got %d", ExitParse, exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	errText := stderr.String()
	if !strings.Contains(errText, "missing issue or pull request reference") {
		t.Fatalf("expected missing argument message, got %q", errText)
	}
	if !strings.Contains(errText, "gh-pr timeline <owner>/<repo>#<number>") {
		t.Fatalf("expected usage help in stderr, got %q", errText)
	}
}

func TestRunNotificationsWithArgsShowsHelpTextOnStderr(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"notifications", "extra"}, &stdout, &stderr)
	if exitCode != ExitParse {
		t.Fatalf("expected exit code %d, got %d", ExitParse, exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	errText := stderr.String()
	if !strings.Contains(errText, "notifications does not accept positional arguments") {
		t.Fatalf("expected positional argument error, got %q", errText)
	}
	if !strings.Contains(errText, "gh-pr notifications") {
		t.Fatalf("expected usage help in stderr, got %q", errText)
	}
}
