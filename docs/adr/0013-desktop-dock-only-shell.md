# ADR 0013 — Dock-only Desktop shell

Status: accepted 2026-10-31.

## Context

The Shell had a 34 px status bar on top (clock, brand, bell, Control Center) and a floating dock at the bottom; an
app in focus mode lost ~140 px of height and 24 px of width, and "maximize" still kept both bars. On a 1366×768
laptop an app got 1342×628 px. The Home screen also listed apps already pinned in the dock.

## Decision

**One bar: the dock.** The status bar is removed. The dock holds, in order: Home · pinned and running apps · Search,
Running apps · tray (bell with unread count, connection/Control Center, clock). Vertical docks stack the same items.

**Dock preferences** (`preferences.dock`, Desktop preferences API, versioned as before):
- `position`: `bottom` (default) | `left` | `right`. Below 700 px wide the dock is always at the bottom.
- `size`: `small` (40 px icons) | `medium` (52, default) | `large` (64).
- Changed in Settings → Desktop → Appearance → *Dock*, or by right-clicking empty dock space.
- Pin/unpin: right-click (or the context-menu key / Shift+F10) on a dock or Home icon; Spotlight keeps its pin button.
  Pinned apps can be reordered by dragging them in the dock. Running apps that are not pinned show after the pinned ones.
- The Home screen lists only apps that are **not** pinned (no duplicates); every app stays reachable from Search.

**Work area.** Windows (focus and window modes) and Home use the screen minus the dock on its edge (dock thickness +
8 px gap), instead of fixed top/bottom bars. Focus mode fills the work area with an 8 px margin.

**Maximize** (both modes; double-click title bar in window mode): the app takes the whole viewport, no margin or
radius, and the dock slides away. What stays is a small pill on the dock's edge with the bell and unread count (opens
notifications) and a handle that shows the dock. Pointing at the dock edge (6 px hot zone) also shows it; it hides again
when the pointer leaves. Restore, minimize, Home or opening another app brings the normal layout back.

**Notifications** open as a **side panel** (macOS Notification Center / Windows 11 style) along the right edge of the
work area, full height, not the whole app: latest notifications, mark read, and *Open Notification Center* for the
full app. Reason: checking a notification must not replace the app the user is working in; the full app stays one
click away. Control Center opens as a small panel next to the tray.

## Consequences

- An app in focus mode on 1366×768 gets ~1350×670 px (dock medium); maximized it gets 1366×768.
- Older saved preferences without `dock` read as bottom/medium.
- Not done: auto-hide dock in normal mode, multiple dock rows, per-app dock badges other than notifications.
