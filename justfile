default:
	just tui

install-cli:
	mkdir -p "$HOME/.bin"
	go build -o "$HOME/.bin/gh-pr" ./cmd/gh-pr

install:
	nix build .#app
	if [ -e "/Applications/gh-pr.app" ]; then chmod -R u+w "/Applications/gh-pr.app" 2>/dev/null || true; rm -rf "/Applications/gh-pr.app" || sudo rm -rf "/Applications/gh-pr.app"; fi
	cp -R "result/gh-pr.app" "/Applications/gh-pr.app"
	chmod -R u+w "/Applications/gh-pr.app" || sudo chmod -R u+w "/Applications/gh-pr.app"
	python -c "from pathlib import Path; import re; p=Path('/Applications/gh-pr.app/Contents/Resources/ghostty.conf'); s=p.read_text(); s=re.sub(r'command = direct:/nix/store/.*/gh-pr\.app/Contents/MacOS/gh-notifications_core', 'command = direct:/Applications/gh-pr.app/Contents/MacOS/gh-notifications_core', s); p.write_text(s)"

tui:
	go run ./cmd/gh-pr tui
