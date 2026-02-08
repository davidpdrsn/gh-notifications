package cli

import (
	"fmt"
	"io"
	"strings"
)

const (
	ExitParse     = 2
	ExitAuth      = 3
	ExitNotFound  = 4
	ExitRateLimit = 5
	ExitTransport = 6
	ExitInternal  = 7
)

type AppError struct {
	Code     string
	Message  string
	ExitCode int
	Details  map[string]any
	Cause    error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Cause)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func writeErrorText(w io.Writer, appErr *AppError) {
	if appErr == nil {
		appErr = &AppError{Code: "internal_error", Message: "internal error", ExitCode: ExitInternal, Details: nil, Cause: nil}
	}

	if appErr.Code != "" {
		_, _ = fmt.Fprintf(w, "%s: %s\n", appErr.Code, appErr.Message)
	} else {
		_, _ = fmt.Fprintln(w, appErr.Message)
	}

	if len(appErr.Details) > 0 {
		for key, value := range appErr.Details {
			_, _ = fmt.Fprintf(w, "  %s: %v\n", key, value)
		}
	}

	if appErr.Cause != nil && !strings.Contains(appErr.Cause.Error(), appErr.Message) {
		_, _ = fmt.Fprintf(w, "cause: %v\n", appErr.Cause)
	}
}
