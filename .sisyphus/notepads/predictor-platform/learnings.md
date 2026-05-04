## Task 10: TradingView Chart Initialization

### Learnings
- TradingView Lightweight Charts v5.2.0 uses `addSeries(LightweightCharts.CandlestickSeries, options)` instead of v4's `addCandlestickSeries()`.
- The standalone build exposes `window.LightweightCharts` globally.
- A single TradingView chart renders with 7 canvas elements (layers for grid, series, price scale, time scale, crosshair, etc.), not 1.
- Playwright tests in this project use the local `@playwright/test` package, not global `playwright`.
- System Chrome (`/usr/bin/google-chrome-stable`) is available and Playwright falls back to it when bundled browsers are missing.

### Decisions
- Exported `chart` and `candlestickSeries` as `window.ChartModule` for browser consumption and `module.exports` for potential Node.js testability.
- Used CDN (`unpkg.com`) for the library in `index.html` rather than the local copy, since Task 9 already set this up. The local copy at `frontend/js/lightweight-charts.js` is available as a fallback.
- Chart resize handler attached to `window.resize` to ensure full viewport coverage.

### Issues
- Pre-existing `ws-client.spec.js` tests fail due to port conflicts and timing issues — unrelated to Task 10.
- `npx playwright install chromium` timed out; tests still pass via system Chrome fallback.

### Evidence
- Screenshot saved to `.sisyphus/evidence/task-10-chart.png`
- Playwright test results: 3/3 passed (`frontend.spec.js` + `chart.spec.js`)
