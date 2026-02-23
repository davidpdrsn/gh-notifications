default:
	just tui

install:
	mkdir -p "$HOME/.bin"
	go build -o "$HOME/.bin/gh-pr" ./cmd/gh-pr

tui:
	go run ./cmd/gh-pr tui
