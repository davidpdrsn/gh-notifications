# Mark Events as Read - Implementation Plan

## Goals

- Let users mark timeline events as read/unread.
- Show read state clearly in TUI (timeline + detail context).
- Support hiding read events.
- Persist state locally in SQLite.

## UX Proposal

- Keybinds:
  - `r`: toggle read/unread for selected timeline event. Go to next thing after toggling.
  - `H`: toggle hide-read mode.
- Indicators:
  - Unread event rows: leading marker using `●`.
  - Read event rows: muted style.
  - Detail pane line: `status: read|unread`.
- Threads:
  - Thread header read state is derived from children (read iff all visible children are read).

## SQLite Storage

- Store in local app state DB (e.g. user data dir `state.db`).
- Table:

```sql
CREATE TABLE IF NOT EXISTS event_read_state (
  event_id TEXT PRIMARY KEY,
  read_at  TEXT NOT NULL, -- RFC3339 UTC
  source   TEXT NOT NULL  -- "manual" for now
);

CREATE INDEX IF NOT EXISTS idx_event_read_state_read_at
  ON event_read_state(read_at);
```

- Keying: use existing stable event IDs from timeline mapper.

## Architecture Changes

### 1) Storage package

- Add `internal/readstate` (or similar):
  - `Open(path)`
  - `MarkRead(eventID)`
  - `MarkUnread(eventID)`
  - `ListRead(eventIDs []string) -> map[string]bool`

### 2) TUI state

- Extend `timelineState` with:
  - `readByEventID map[string]bool`
- Add app-level toggle:
  - `HideRead bool`

### 3) Reducer/events/effects

- Events:
  - `ToggleEventReadEvent{Ref, EventID}`
  - `MarkEventUnreadEvent{Ref, EventID}`
  - `MarkVisibleReadEvent{Ref}`
  - `ReadStateLoadedEvent{Ref, ReadIDs []string}`
  - `ToggleHideReadEvent{}`
- Effects:
  - Async DB writes for mark read/unread.
  - Async DB read for freshly loaded timeline IDs.
- Keep reducer pure; all SQLite IO in effect runners.

### 4) Rendering

- Timeline rows:
  - unread marker/style on unread rows.
  - muted style for read rows.
- Hide-read filter:
  - filter rows in display path.
  - maintain valid selection and scroll after filtering.

### 5) Detail + copy

- Include `status: read|unread` in details.
- Keep copied text plain (no ANSI).

## Data Loading Strategy

- On timeline events arrival:
  - batch lookup read state for new IDs.
- On manual read/unread action:
  - optimistic UI update immediately.
  - persist asynchronously; on failure show status and rollback.

## Edge Cases

- Missing event ID: skip persistence for that row.
- Hide-read with all events read: show empty-state message.
- Thread headers with mixed read child states.
- Selection currently on a row that becomes hidden.

## Testing Plan

- Storage unit tests:
  - migration/table creation
  - mark read/unread
  - persistence across reopen
- Reducer tests:
  - toggling read/unread updates state
  - hide-read filters rows and normalizes selection
  - mark visible read works for wrapped/thread rows
- View tests:
  - unread marker/rendering
  - hide-read empty state
- Integration-style TUI loop test:
  - key -> effect -> async msg -> state/render update

## Rollout Steps

1. Add SQLite read-state storage package + tests.
2. Wire reducer events/effects for read/unread.
3. Add row indicators in timeline rendering.
4. Add hide-read filtering + normalization.
5. Add keybind hints to status/help text.
6. Add integration test coverage.
