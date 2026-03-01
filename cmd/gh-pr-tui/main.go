package main

import (
	"context"
	"os"

	"gh-pr/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), []string{"tui"}, os.Stdout, os.Stderr))
}
