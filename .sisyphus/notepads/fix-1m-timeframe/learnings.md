# Learnings from fix-1m-timeframe session

## Root Causes Found

1. **Pattern candles invisible on 1m**: `avgSeconds` was calculated from actual candle spacing (~60s for 1m), placing predictions at `lastTime + 60`, which is 480px away at 8px bar spacing. Only 40px right margin (rightOffset:5 x barSpacing:8) meant predictions were off-screen.

2. **1m candle gaps**: `timeVisible: true` was missing from timeScale config. Without it, TradingView Lightweight Charts does not render proper time labels on the x-axis, making candles appear as dashes/gaps.

3. **State loss on TF change**: `saveTickerState(activeTicker)` was never called before `candles = []` in `setupTimeframeSelector`. When switching tabs during a timeframe change, the old ticker's candles were lost.

## Fixes Applied

### chart.js (Task 1)
- `timeScale`: Added `timeVisible: true`, `secondsVisible: true`
- `timeScale`: Changed `rightOffset` from 5 to 8

### chart.js (Task 2)
- `updatePatternPrediction`: Replaced `avgSeconds` calculation block with fixed `predictionOffset = 3`
- This places predictions 3 time-units ahead on ALL timeframes (12px on 1s, 24px on 1m)

### app.js (Task 3)
- `setupTimeframeSelector`: Added `saveTickerState(activeTicker)` before `candles = []`

## Verification
- Syntax: `node -c` passes for both chart.js and app.js
