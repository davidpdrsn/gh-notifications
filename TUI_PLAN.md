# TUI Implementation Plan (No Implementation Yet)

## Goal

Build a Yazi-style terminal UI with drilldown navigation and non-blocking async loading:

- Left column: notifications (PRs/issues only), sorted newest-first.
- Middle column: timeline for selected notification, sorted oldest-first.
- Right column: details for selected timeline row.
- Drill in and out without losing context/selection.

## Hard Requirements

1. UI thread never blocks on network or sorting work.
2. Data is streamed and inserted into sorted positions incrementally.
3. No "load all then sort" behavior.
4. Selection must remain stable while new items are inserted.
5. PR threaded comment events are visually grouped as threads.

## Command Surface

Add a new command:

- `gh-pr tui`

This command launches the interactive TUI and uses the existing library client (`ghpr.Client`) for streaming.

## Dependencies

Use:

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles/viewport`

No network calls from Bubble Tea `Update`; only from goroutines started by Tea commands.

## Architecture

### State model

Top-level model contains:

- Active focus column (`left|middle|right`)
- Notifications pane state
- Timeline pane state
- Detail pane state
- Loader generation counters for stale-message rejection
- Optional status/error line

Pane state includes:

- `rows []Row`
- `selectedID string`
- `selectedIndex int`
- `scrollOffset int`
- `indexByID map[string]int`
- `loading bool`
- `done bool`
- `err string`

### Row types

Notifications row:

- `id`
- `updatedAt`
- `title`
- `repo`
- `kind` (`pr|issue`)
- `ref` (`owner/repo#number`)

Timeline row union:

- `event-row` for normal events
- `thread-row` for grouped PR review comments

Thread row stores:

- `threadID`
- `children []TimelineRow` (comment events)
- `expanded bool`
- aggregate display metadata (path/count/firstAt/latestAt)

## Async Data Flow

### Notifications loader

1. Start goroutine with generation id `notifGen`.
2. Stream notifications via `ghpr.Client.StreamNotifications`.
3. Emit Tea messages: `notifArrived(gen, item)`, `notifDone(gen)`, `notifErr(gen, err)`.
4. `Update` ignores messages where `gen != current notifGen`.

### Timeline loader

1. Triggered when notification selection changes or user drills into timeline.
2. Cancel previous timeline context.
3. Start goroutine with generation id `timelineGen`.
4. Stream timeline via `ghpr.Client.StreamTimeline`.
5. Emit Tea messages: `timelineArrived(gen, event)`, `timelineWarn(gen, text)`, `timelineDone(gen)`, `timelineErr(gen, err)`.
6. `Update` ignores stale generation messages.
7. `Update` accepts timeline messages only when both generation and `ref` match the current selection.

### Async command scheduling

1. Keep exactly one in-flight `waitForAsyncMsg` Tea command.
2. Re-arm `waitForAsyncMsg` only after handling an async loader message.
3. Do not spawn `waitForAsyncMsg` for key or window-size input events.

## Incremental Sorting Strategy

### Notifications (newest-first)

Sort key:

- primary: `updatedAt` descending
- tie-break: `id` ascending

On insert:

1. Binary search insertion point.
2. Insert into slice (shift tail).
3. Rebuild/patch `indexByID`.
4. Re-anchor selection by `selectedID`.

### Timeline (oldest-first)

Sort key:

- synthetic opened event pinned first
- then `occurredAt` ascending
- tie-break: `id` ascending

On insert:

1. If threaded PR comment, route into thread group logic.
2. Else binary-insert normal event row.
3. Re-anchor selection by stable row id.

## Thread Grouping for PR Events

Threading key:

- `event.comment.thread_id` for `github.review_comment`

Data structures:

- `threadByID map[string]*ThreadGroup`
- thread group represented by a `thread-row` in middle pane

Insert behavior:

1. If group does not exist, create thread row and binary-insert it in timeline rows.
2. Insert new comment into group children by `occurredAt` ascending.
3. Update group metadata (count/path/latestAt).
4. Preserve current selection id/index.

Visual representation:

- Collapsed header: `file/path  (N comments)`
- Expanded children rendered with tree guides:
  - `├─`
  - `└─`
  - vertical continuation `│`

## Selection Stability Rules

Selection is id-based, never index-only.

After any insert/rebuild:

1. If `selectedID` still exists, set `selectedIndex = indexByID[selectedID]`.
2. If missing, clamp to nearest valid row and update `selectedID`.
3. Keep `scrollOffset` unless selected row moves outside viewport.

Back navigation restores prior pane state (`selectedID`, `selectedIndex`, `scrollOffset`).

## Keybindings

- `j` / `Down`: move down
- `k` / `Up`: move up
- `l` / `Enter`: drill in / expand thread
- `h` / `Backspace`: go back / collapse thread
- `q`: quit

## File Plan

- `internal/tui/model.go` (state structs + constructors)
- `internal/tui/update.go` (message handling, loader orchestration)
- `internal/tui/view.go` (column rendering and styles)
- `internal/tui/keys.go` (key map)
- `internal/tui/insert.go` (binary insert + re-anchor helpers)
- `internal/tui/threads.go` (thread grouping and expansion logic)
- `internal/cli/timeline.go` (register `tui` subcommand)

## Rollout Sequence

1. Add command scaffold and empty model/view.
2. Add async notifications stream and incremental sorted insertion.
3. Add async timeline stream and incremental sorted insertion.
4. Add thread grouping for PR review comments.
5. Add detail pane and drill/back navigation stack.
6. Add polish: status line, loading states, error display.
7. Validate selection stability under live inserts.

## Progress

- [x] Added `gh-pr tui` command scaffold.
- [x] Added Bubble Tea/Lip Gloss dependencies.
- [x] Implemented async notifications loader (never blocks UI update loop).
- [x] Implemented incremental sorted notifications insertion (newest-first) without buffering all data.
- [x] Implemented async timeline loader with cancellation + generation IDs to ignore stale messages.
- [x] Implemented incremental sorted timeline insertion (oldest-first) without buffering all data.
- [x] Added PR threaded grouping using `comment.thread_id` with visual grouping and expand/collapse.
- [x] Preserved selection by stable IDs while inserts stream in.
- [x] Implemented Yazi-style drill/back navigation (`h/j/k/l`, `enter`, `backspace`, `q`).
- [x] Added detail pane showing selected notification/event content.
- [x] Added notifications `updated_at` to schema + mapping for proper UI sorting.
- [x] Refactored TUI around a single app state + pure reducer + effect runner.
- [x] Added reducer-focused state transition tests for ordering, stale generations, and thread grouping.
- [x] Verified `go test ./...` and `golangci-lint run` pass.
- [x] Hardened timeline stale-message rejection to require both generation and current ref match.
- [x] Prevented async waiter fan-out by re-arming async wait only for async messages.
- [x] Added tests for cached-ref generation invalidation, non-current timeline message rejection, and update-loop waiter scheduling.

## Validation Checklist

- Rapidly changing notification selection does not show stale timeline data.
- Notification list remains newest-first while streaming.
- Timeline remains oldest-first while streaming.
- Threaded comments group correctly by `thread_id`.
- Expanding/collapsing threads does not lose selection.
- Switching to a cached timeline invalidates old in-flight generation messages from prior refs.
- UI remains responsive during active network streams.
- `go test ./...` and `golangci-lint run` pass after implementation.
