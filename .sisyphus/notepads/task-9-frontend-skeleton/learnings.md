# Task 9: HTML Page Skeleton + Dark Theme CSS + Status Overlay

## Learnings

- Used flexbox column layout with `#chart-container { flex: 1; min-height: 75vh; }` to ensure chart occupies most of viewport
- Status bar uses `position: fixed; bottom: 0;` with CSS class-based state transitions (status-connecting, status-connected, etc.)
- Loader overlay uses `position: fixed;` with semi-transparent `rgba(26, 26, 46, 0.92)` background
- Spinner implemented with CSS `@keyframes spin` and border-top-color animation
- Status dot uses `@keyframes pulse` for live/reconnecting states
- Playwright configured to use system Chrome at `/usr/bin/google-chrome` to avoid downloading Chromium

## Decisions

- Dark theme palette: background #1a1a2e, containers #16213e, text #e0e0e0, grid borders #2a2a4e
- Status colors: connected=green (#4caf50), connecting/collecting/reconnecting=yellow (#d4a017), disconnected/error=red (#ef5350)
- Added TradingView Lightweight Charts CDN script in HTML (unpkg.com) so Task 10 has it ready
- Kept existing HTML structure from prior work but enhanced with loader overlay and CDN

## Evidence

- Playwright tests: 2/2 passed
  - test_page_loads: chart container visible, height > 400px, dark background rgb(26, 26, 46), no JS errors
  - test_status_bar_visible: status bar at bottom with "Connecting" text and status-connecting class
- Screenshot: .sisyphus/evidence/task-9-screenshot.png
