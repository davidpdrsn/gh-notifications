package github

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type TokenResolver struct {
	LookupEnv func(string) (string, bool)
	ExecToken func(context.Context) (string, error)
}

func NewTokenResolver() TokenResolver {
	return TokenResolver{
		LookupEnv: os.LookupEnv,
		ExecToken: execGhAuthToken,
	}
}

func (r TokenResolver) Resolve(ctx context.Context) (string, error) {
	if r.LookupEnv == nil {
		r.LookupEnv = os.LookupEnv
	}
	if r.ExecToken == nil {
		r.ExecToken = execGhAuthToken
	}

	if token, ok := r.LookupEnv("GITHUB_TOKEN"); ok && strings.TrimSpace(token) != "" {
		return strings.TrimSpace(token), nil
	}

	if token, ok := r.LookupEnv("GH_TOKEN"); ok && strings.TrimSpace(token) != "" {
		return strings.TrimSpace(token), nil
	}

	token, err := r.ExecToken(ctx)
	if err != nil {
		return "", err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("gh auth token returned empty token")
	}

	return token, nil
}

func execGhAuthToken(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("failed to resolve token from gh auth token: %s", msg)
	}
	return string(out), nil
}
