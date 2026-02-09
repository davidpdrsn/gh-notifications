# Mouse Support - Implementation Plan

## Goals

- Support wheel scrolling in all visible columns.
- Support clicking rows to select.
- Support clicking rows in other columns to switch focus and select.
- Keep existing keyboard behavior unchanged and consistent.

## UX Scope (first iteration)

- Wheel scroll:
  - Notifications column: scroll notification list.
  - Timeline column: scroll timeline list.
  - Detail column: scroll detail view.
- Left click:
  - Notifications row: select notification + focus notifications.
  - Timeline row: select timeline row + focus timeline.
  - Detail pane click: focus detail (no row selection change).
- Tab bar click (new org tabs):
  - Clicking a tab activates that tab.

## Non-goals (first iteration)

- Drag/select text.
- Mouse hover tooltips.
- Double-click special actions.
- Right-click context menus.

## Technical Plan

### 1) Enable mouse in Bubble Tea program

- Update TUI program startup to enable mouse events (if not already):
  - Add `tea.WithMouseCellMotion()` in program options (or appropriate Bubble Tea mouse option supported by current version).

### 2) Reducer event model additions

- Add explicit mouse events so reducer remains source of truth:
  - `MouseClickEvent{X, Y int, Button string}`
  - `MouseWheelEvent{X, Y int, Delta int}` (negative up, positive down)

### 3) Pane hit-testing and layout map

- Add helper that computes exact pane/tab bounds for current render layout:
  - Notifications+Timeline when notifications focused.
  - Timeline+Detail when timeline/detail focused.
- Include:
  - Tab row bounds in notifications pane.
  - Content viewport bounds per pane.

### 4) Row hit-testing

- Notifications:
  - Reuse wrapped row height math to map `y` -> visible row index.
- Timeline:
  - Reuse wrapped row height math + thread rows to map `y` -> visible row.
- Detail:
  - Click only changes focus to detail in v1.

### 5) Mouse behavior wiring

- Wheel handling:
  - In notifications pane: adjust `NotifScroll`.
  - In timeline pane: adjust timeline `scrollOffset`.
  - In detail pane: adjust `DetailScroll`.
  - Clamp using existing normalization helpers.
- Click handling:
  - Notifications row click:
    - set `Focus=focusNotifications`
    - set selected notification and load timeline if ref changes.
  - Timeline row click:
    - set `Focus=focusTimeline`
    - set selected timeline row.
  - Detail pane click:
    - set `Focus=focusDetail`.
  - Notifications tab click:
    - switch tab exactly like keyboard tab-cycling logic.

## State/Rendering Considerations

- Keep all selection updates routed through reducer helpers already used by keyboard path.
- Preserve current behavior of resetting detail scroll on timeline row change.
- Ensure mouse interactions do not spawn extra async waiters (respect TUI async contract).

## Testing Plan

- Reducer unit tests:
  - Wheel in notifications/timeline/detail updates corresponding scroll state.
  - Click on notification row selects and changes focus.
  - Click on timeline row selects and changes focus.
  - Click on detail focuses detail.
  - Click on org tab switches active tab.
- Integration-style TUI loop test:
  - Send mouse messages and verify model state transitions.

## Rollout Steps

1. Enable mouse in Bubble Tea program setup.
2. Add mouse message parsing in `Update` and reducer events.
3. Implement pane + row hit-testing helpers.
4. Wire click and wheel behaviors.
5. Add tests (unit + one loop/integration test).
6. Update status/help line with short mouse hints.
