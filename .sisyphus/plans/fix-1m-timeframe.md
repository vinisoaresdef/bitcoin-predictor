# Fix 1m Timeframe: Candlestick Gaps & Pattern Predictions

## TL;DR

> **Quick Summary**: Fix two 1m timeframe bugs — candle gaps (missing time labels/proper spacing) and invisible blue/yellow pattern prediction candles (placed too far right on 1m due to 60s offset).

> **Deliverables**:
> - 1m candles render as proper candlesticks with visible time labels (no gaps)
> - Blue/yellow prediction candles appear on 1m timeframe
> - Save ticker state before clearing on timeframe change

> **Estimated Effort**: Quick (3 targeted edits across 2 files)
> **Parallel Execution**: YES — all 3 tasks are independent
> **Critical Path**: N/A (all independent)

---

## Context

### Original Request
User reports: "For 1s timeframe, the blue and yellow candles are working fine. But, for 1m timeframe, I can't see them. Furthermore, the candles on this timeframe do not follow a linear and understandable line. It is like a line chart with gaps on it."

### Root Cause Analysis
1. **Pattern candles invisible on 1m**: `updatePatternPrediction` calculates `avgSeconds ≈ 60` for 1m data, placing predictions at `lastTime + 60` (480px at 8px bar spacing vs 40px right margin)
2. **1m candle gaps**: `timeVisible: true` missing from `createChart()` timeScale; `fitContent()` not called after chart reset
3. **Race condition**: `setupTimeframeSelector` clears `candles = []` without `saveTickerState(activeTicker)` first

### Scope
- **IN**: Fix chart.js timeScale config, fix updatePatternPrediction time offset, fix setupTimeframeSelector state save
- **OUT**: No backend changes, no new features, no ML model changes

---

## Work Objectives

### Core Objective
Fix 1m timeframe rendering so candles display properly (no gaps) and blue/yellow pattern predictions are visible on the chart.

### Definition of Done
- [x] 1m candles render as proper candlesticks with time labels visible on x-axis
- [x] Blue/yellow prediction candles visible at 2-3 bars ahead on 1m timeframe
- [x] Pattern predictions remain working on 1s timeframe
- [x] Ticker state saved before clearing on timeframe change

---

## Verification Strategy

> All verification is agent-executed via browser/curl.

### Test Decision
- **Infrastructure exists**: No frontend test framework
- **Automated tests**: None (manual verification via browser + curl)
- **Agent-Executed QA**: Playwright for browser verification

---

## TODOs

### Task 1: Fix chart.js — add timeVisible + increase rightOffset

**What to do**:
In `frontend/js/chart.js`, `createChart()` config (lines 19-27):

1. Add `timeVisible: true` inside `timeScale: {}`
2. Add `secondsVisible: true` inside `timeScale: {}`
3. Change `rightOffset` from `5` to `8`

**Must NOT do**:
- Don't change `barSpacing` (handled by `resetChart` per-timeframe)
- Don't change any other chart options

**Parallelization**: Can run in parallel with Tasks 2 and 3
**Blocks**: None
**Blocked By**: None

**References**:
- `frontend/js/chart.js:19-27` — current `timeScale` config
- `frontend/js/chart.js:514-546` — `resetChart` for context

**QA Scenarios**:
```
Scenario: 1m candles render with visible time labels and no gaps
  Tool: Playwright
  Preconditions: App running at localhost:8080 with 1m timeframe selected
  Steps:
    1. Navigate to http://localhost:8080
    2. Wait for connection (status bar shows "Connected (1s)")
    3. Select 1m from timeframe dropdown
    4. Wait 2 minutes for 2+ candles to arrive
    5. Check time labels show HH:MM format on x-axis
    6. Check candlesticks render as bars (not dashes) with wick lines
  Expected Result: Time labels visible; candlesticks show body+wick with open/close visible
  Failure Indicators: No time labels; candles appear as flat dashes; gaps between candles
  Evidence: .sisyphus/evidence/task-1-1m-candles.png

Scenario: Pattern predictions visible on 1m after 2+ candles
  Tool: Playwright
  Preconditions: Same as above, 2+ 1m candles accumulated
  Steps:
    1. Wait for 2+ candles to appear on chart
    2. Look for blue or yellow candle(s) 2-3 bars ahead of last real candle
    3. Verify the prediction candles are semi-transparent (rgba colors)
  Expected Result: At least one blue or yellow prediction candle visible ahead
  Failure Indicators: No blue/yellow candles anywhere on chart
  Evidence: .sisyphus/evidence/task-1-pattern-prediction.png
```

**Commit**: YES
- Message: `fix(chart): add timeVisible, increase rightOffset for 1m rendering`
- Files: `frontend/js/chart.js`

---

### Task 2: Fix chart.js — use fixed prediction offset instead of avgSeconds

**What to do**:
In `frontend/js/chart.js`, `updatePatternPrediction()` function (line 569-573):

1. Remove the `avgSeconds` calculation block (lines 569-573: `var avgSeconds = 60; ... if (avgSeconds < 1) avgSeconds = 1;`)
2. Replace with: `var predictionOffset = 3; // 3 time units ahead (consistent visual spacing)`
3. On line 579, change `lastTime + (i + 1) * avgSeconds` to `lastTime + (i + 1) * predictionOffset`

**Why**: `avgSeconds` is ~60 on 1m, placing predictions 480px away (invisible). A fixed 3-unit offset places them 24px ahead on 1m (visible) and 12px ahead on 1s (visible). This is a "visual prediction" — the exact timestamp doesn't matter, only the relative visual position.

**Must NOT do**:
- Don't change the candlestick rendering logic (body/colors remain same)
- Don't change `setData` or `applyOptions` calls
- Don't add new calculations

**Parallelization**: Can run in parallel with Tasks 1 and 3
**Blocks**: None
**Blocked By**: None

**References**:
- `frontend/js/chart.js:552-613` — full `updatePatternPrediction` function
- `frontend/js/chart.js:569-573` — lines to replace (avgSeconds block)
- `frontend/js/chart.js:579` — line to change (futureTime calc)

**QA Scenarios**:
```
Scenario: Predictions visible on 1s (regression check)
  Tool: Playwright
  Preconditions: App running at localhost:8080, 1s timeframe
  Steps:
    1. Refresh page, wait for 5+ 1s candles
    2. Look for blue/yellow prediction candle(s) just ahead of last real candle
    3. Verify predictions are 2-3 bars ahead (not glued to last candle, not far away)
  Expected Result: Predictions visible, positioned 2-3 bar-widths ahead
  Evidence: .sisyphus/evidence/task-2-1s-prediction.png

Scenario: Predictions visible on 1m after fix
  Tool: Playwright
  Preconditions: 1m timeframe, 3+ candles accumulated
  Steps:
    1. Select 1m from timeframe dropdown
    2. Wait 3 minutes for 3+ candles
    3. Look for blue/yellow prediction candles 2-3 bars ahead
  Expected Result: At least one prediction candle visible just ahead of last real candle
  Evidence: .sisyphus/evidence/task-2-1m-prediction.png
```

**Commit**: YES
- Message: `fix(chart): use fixed prediction offset for consistent visual spacing across timeframes`
- Files: `frontend/js/chart.js`

---

### Task 3: Fix app.js — save ticker state before clearing on timeframe change

**What to do**:
In `frontend/js/app.js`, `setupTimeframeSelector()` (line 179):

1. Before `candles = []`, add: `saveTickerState(activeTicker);`

**Why**: When changing timeframe, the current candles need to be saved to `tickerData[activeTicker].candles` before the array is cleared. Without this, switching tabs during a timeframe change causes data loss because `restoreTickerState` finds an empty array.

**Must NOT do**:
- Don't change any other logic in `setupTimeframeSelector`
- Don't add calls to `restoreTickerState` (that's handled by tab switching)

**Parallelization**: Can run in parallel with Tasks 1 and 2
**Blocks**: None
**Blocked By**: None

**References**:
- `frontend/js/app.js:141-144` — `saveTickerState` function (ref to copy)
- `frontend/js/app.js:168-185` — `setupTimeframeSelector` (insertion point at line 179)
- `frontend/js/app.js:146-161` — `restoreTickerState` (shows why saving matters)

**QA Scenarios**:
```
Scenario: Tab switch during timeframe change preserves data
  Tool: Playwright
  Preconditions: App running, BTCUSDT with active 1s candles
  Steps:
    1. Wait for 10+ candles on BTCUSDT (1s)
    2. Change timeframe to 1m
    3. Immediately add ETHUSDT tab (click "+", enter "ETHUSDT")
    4. Switch back to BTCUSDT tab
    5. Verify previous 1s candles are restored (chart shows historical data)
  Expected Result: BTCUSDT shows previously accumulated candles (not empty)
  Evidence: .sisyphus/evidence/task-3-state-preserve.png
```

**Commit**: YES
- Message: `fix(app): save ticker state before clearing on timeframe change`
- Files: `frontend/js/app.js`

---

## Commit Strategy

All 3 tasks committed separately:
- **1**: `fix(chart): add timeVisible, increase rightOffset for 1m rendering` — `frontend/js/chart.js`
- **2**: `fix(chart): use fixed prediction offset for consistent visual spacing across timeframes` — `frontend/js/chart.js`
- **3**: `fix(app): save ticker state before clearing on timeframe change` — `frontend/js/app.js`

---

## Success Criteria

### Verification Commands
```bash
# Syntax check all modified files
node -c frontend/js/chart.js && echo "chart.js: OK"
node -c frontend/js/app.js && echo "app.js: OK"
```

### Final Checklist
- [x] `timeVisible: true` and `secondsVisible: true` present in timeScale config
- [x] `rightOffset: 8` (was 5)
- [x] `updatePatternPrediction` uses fixed `predictionOffset = 3` instead of `avgSeconds`
- [x] `setupTimeframeSelector` calls `saveTickerState(activeTicker)` before `candles = []`
- [x] 1m candles render as candlesticks (not dashes/gaps)
- [x] Blue/yellow predictions visible on 1m
- [x] 1s predictions still visible (no regression)
- [x] Tab switch during timeframe change preserves state
