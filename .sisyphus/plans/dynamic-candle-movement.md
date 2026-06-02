# Fix Dynamic Candle Movement (Partial Kline Updates)

## TL;DR

> **Quick Summary**: Remove overly aggressive deduplication that blocks partial kline updates from reaching the frontend. Candles currently appear static because only the FIRST partial update per `close_time` is sent — subsequent updates (with changed OHLCV) are dropped as "duplicates."

> **Deliverables**:
> - Candles update dynamically on chart as partial kline data arrives (before candle closes)
> - Works for all timeframes (1s, 1m, 5m)

> **Estimated Effort**: Quick (3 targeted edits across 2 Go files + rebuild)
> **Parallel Execution**: No — sequential (schema → parser → dedup logic)
> **Critical Path**: Schema change → ParseKlineMessage → isDuplicate → rebuild

---

## Context

### Root Cause Analysis

Binance WebSocket sends kline updates every second for the current (forming) candle. Each update has `k.x = false` (IsFinal) while forming, and `k.x = true` when complete.

The problem is in `go-backend/internal/binance/client.go`:

1. **`isDuplicate` (lines 294-313)**: Only checks `close_time`. Once ANY update for a given `close_time` is seen, ALL subsequent updates (including the final one) are silently dropped.
2. **`ParseKlineMessage` (lines 341-396)**: Does not extract `IsFinal` field even though `BinanceKlineMessage` struct has it at line 103.
3. **`Candle` schema (schemas/schemas.go:5-16)**: No `IsFinal` field exists.

**Impact for each timeframe:**
- **1s candles**: 1 update per candle (close_time changes every second) → works correctly, candles appear dynamic
- **1m candles**: ~60 updates per candle → first update sent, 59 updates DROPPED → candle appears STATIC for 59 seconds
- **5m candles**: ~300 updates per candle → candle appears STATIC for ~299 seconds

**Frontend is already correct**: `app.js` `handleCandle` updates candles with same `close_time` in-place, and `chart.js` `updateCandle` calls `realSeries.update()` which TradingView renders live. The frontend just never receives the partial updates because the backend drops them.

### Scope
- **IN**: schemas/schemas.go (add IsFinal), client.go (extract IsFinal + fix dedup), docker rebuild
- **OUT**: No frontend changes, no WebSocket protocol changes, no ML changes

---

## Work Objectives

### Core Objective
Fix candle deduplication to allow partial kline updates to flow through from Binance → Go backend → WebSocket → frontend, enabling dynamic candle movement on the chart.

### Definition of Done
- [ ] Candles update dynamically on 1m and 5m timeframes (not just 1s)
- [ ] OHLCV changes visible as candle moves before close
- [ ] Docker container rebuilds and deploys successfully
- [ ] No duplicate candles rendered on chart (frontend already handles dedup)

---

## Verification Strategy

> All verification is agent-executed via browser observation and WebSocket inspection.

### Test Decision
- **Go tests**: `go test ./...` (existing tests must pass)
- **Agent-Executed QA**: Observe chart for dynamic candle movement at 1m timeframe

---

## TODOs

### Task 1: Add `IsFinal` field to `Candle` schema + extract it in ParseKlineMessage

**What to do**:

In `go-backend/internal/schemas/schemas.go`:
1. Add `IsFinal bool \`json:"is_final"\`` field to the `Candle` struct

In `go-backend/internal/binance/client.go`, `ParseKlineMessage()`:
2. Extract `IsFinal` from `k.IsFinal` (the `x` field from Binance kline message)
3. Include it in the returned `schemas.Candle{}`

**Must NOT do**:
- Don't change other fields in the Candle struct
- Don't change the JSON marshaling of existing fields

**References**:
- `go-backend/internal/schemas/schemas.go:5-16` — current `Candle` struct
- `go-backend/internal/binance/client.go:103` — `IsFinal` field in `BinanceKlineMessage`
- `go-backend/internal/binance/client.go:385-395` — current `ParseKlineMessage` return

**QA Scenarios**:
```
Scenario: Go tests still pass after schema change
  Tool: Bash
  Steps:
    1. cd go-backend && go test ./...
  Expected Result: All tests pass (PASS for all packages)
  Evidence: Terminal output
```

---

### Task 2: Fix `isDuplicate` to allow partial candle updates through

**What to do**:

In `go-backend/internal/binance/client.go`, `isDuplicate()` function:

Replace current logic with:
```
func (c *Client) isDuplicate(candle schemas.Candle) bool {
    c.mu.Lock()
    defer c.mu.Unlock()

    closeTimeMs := candle.CloseTime.UnixMilli()
    last, exists := c.seenCandles[closeTimeMs]

    if !exists {
        // First time seeing this close_time
        c.seenCandles[closeTimeMs] = candle
        return false
    }

    // If we've already seen the FINAL candle, skip duplicates
    if last.IsFinal {
        return true
    }

    // Update the stored candle (partial updates flow through)
    c.seenCandles[closeTimeMs] = candle
    return false
}
```

Change `seenCandles` type from `map[int64]struct{}` to `map[int64]schemas.Candle`.

**Why**: 
- Tracks whether the FINAL candle has been seen for each close_time
- PARTIAL updates (IsFinal=false) always flow through, allowing dynamic candle movement
- FINAL update (IsFinal=true) marks the close_time as complete
- After final is seen, subsequent duplicates are blocked

**Must NOT do**:
- Don't break the cleanup logic (`cleanupOldEntries`)
- Don't change the processing flow in `connectAndProcess`

**References**:
- `go-backend/internal/binance/client.go:70` — `seenCandles` field declaration
- `go-backend/internal/binance/client.go:294-313` — current `isDuplicate` function
- `go-backend/internal/binance/client.go:316-339` — `cleanupOldEntries` (needs update for new type)
- `go-backend/internal/binance/client.go:131` — `ChangeStreamURL` resets `seenCandles` (needs update)

**QA Scenarios**:
```
Scenario: Build succeeds after changes
  Tool: Bash
  Steps:
    1. cd go-backend && go build ./...
  Expected Result: Build succeeds with no errors
  Evidence: Terminal output

Scenario: Race tests pass
  Tool: Bash
  Steps:
    1. cd go-backend && go test -race ./...
  Expected Result: All tests pass with race detection
  Evidence: Terminal output
```

---

### Task 3: Rebuild Go backend Docker container

**What to do**:
1. Navigate to project root
2. Run `docker compose build go-backend`
3. Run `docker compose up -d go-backend`
4. Verify container is healthy: `docker compose ps`

**QA Scenarios**:
```
Scenario: Container healthy after rebuild
  Tool: Bash
  Steps:
    1. docker compose ps go-backend
  Expected Result: Status shows "healthy" or "Up"
  Evidence: Terminal output

Scenario: WebSocket connection works
  Tool: curl
  Steps:
    1. curl -s http://localhost:8080/health
  Expected Result: {"status": "ok"}
  Evidence: Terminal output

Scenario: 1m candles move dynamically on chart
  Tool: Playwright
  Preconditions: App running, backend rebuilt, frontend refreshed
  Steps:
    1. Navigate to http://localhost:3000
    2. Switch to 1m timeframe
    3. Wait for a candle to appear
    4. Observe the current (rightmost) candle — it should change OHLCV over 60s
    5. Take screenshot after 5s and after 30s to compare
  Expected Result: Candle wick/body changes position over time (dynamic movement)
  Evidence: .sisyphus/evidence/task-3-dynamic-1m-before.png + after.png
```

---

## Commit Strategy

All changes in one commit (interdependent):
- `fix(backend): enable dynamic candle movement by forwarding partial kline updates`
- Files: `go-backend/internal/schemas/schemas.go`, `go-backend/internal/binance/client.go`
- Pre-commit: `cd go-backend && go test -race ./... && go build ./...`

---

## Success Criteria

### Verification Commands
```bash
# Go tests
cd go-backend && go test -race ./... && echo "TESTS: OK"

# Build
cd go-backend && go build ./... && echo "BUILD: OK"

# Container
docker compose build go-backend && docker compose up -d go-backend
```

### Final Checklist
- [ ] `IsFinal` field in `Candle` struct
- [ ] `ParseKlineMessage` extracts `IsFinal`
- [ ] `isDuplicate` allows partial updates through, blocks after final
- [ ] `seenCandles` type updated to `map[int64]schemas.Candle`
- [ ] `cleanupOldEntries` updated for new map type
- [ ] `ChangeStreamURL` reset updated for new map type
- [ ] All Go tests pass with race detection
- [ ] Docker container rebuilds and deploys successfully
- [ ] 1m candles show dynamic movement on chart
