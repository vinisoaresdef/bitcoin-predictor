## SMA(20) Line Implementation - Learnings

### Patterns Found
- TradingView Lightweight Charts v5 API uses `chart.addSeries(LightweightCharts.LineSeries, {...})`
- `series.update({time, value})` for live updates, `series.setData([...])` for bulk updates
- The project uses IIFE modules with `global.ChartModule` / `global.WSClient` for browser globals

### Successful Approaches
- TDD approach: wrote Playwright test first, then implemented feature
- Used `page.evaluate()` to inject mock candles and read back `ChartModule.smaData` for verification
- Kept SMA calculation and line series management centralized in `chart.js`
- Delegated from `app.js` to `ChartModule.updateCandle()` / `ChartModule.setCandles()` for clean separation

### Gotchas
- The `chart.spec.js` file had a concurrent modification with a WebSocket-based test block using port 8766
- Moving the SMA test to its own `test.describe()` block avoided `beforeAll` port conflicts
- `app.js` already had windowing logic (60-second window) — needed to preserve it while delegating to ChartModule

### Architecture Decision
- `chart.js` owns all chart state: candles array, SMA data, both series
- `app.js` handles WebSocket events and windowing, then delegates to ChartModule methods
- This avoids duplicate state and ensures SMA is always calculated from the canonical candle history
