# Error Budget & Resilience Documentation

> Auto-generated from source code analysis. Values extracted from actual implementations.
> Last updated: 2026-05-04

---

## 1. Timeout Configurations

### 1.1 ML Service Client (`go-backend/internal/mlclient/client.go`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `defaultTimeout` | **500ms** | Matches ML service internal timeout. Per-request timeout applied via `context.WithTimeout`. Fast-fail on slow predictions. |
| HTTP Client Timeout | Inherits `defaultTimeout` | Set on `http.Client.Timeout` for connection-level timeouts. |

**Configurable via functional options:**
- `WithTimeout(duration)` — overrides the per-request timeout
- `WithRetryConfig(maxRetries, delay)` — overrides retry behavior

**Test coverage:** `TestPredictTimeout` in `client_test.go` — verifies timeout errors are returned when server exceeds client timeout (50ms client timeout vs 200ms server delay).

### 1.2 ML Client — Legacy (`go-backend/internal/ml/client.go`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `DefaultTimeout` | **10s** | Coarse HTTP client timeout. Configurable via `ML_TIMEOUT` env var (seconds). |
| Env override | `ML_TIMEOUT` | Parsed as integer seconds. Falls back to 10s if unset or invalid. |

**Test coverage:** `TestDefaultConfig_EnvVars` — verifies `ML_TIMEOUT=30` results in 30s timeout.

### 1.3 ML Service Internal (`ml-service/main.py`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `REQUEST_TIMEOUT_SECONDS` | **30s** | Timeout for `asyncio.wait_for()` wrapping prediction computation in a thread pool. Raised as `ModelUnavailableError` on expiry. |

**Test coverage:** `TestTimeoutHandling.test_timeout_handling_simulated_slow_request` — verifies slow requests complete within timeout. `test_error_handling.py` tests 503/timeout responses.

### 1.4 Binance WebSocket Client (`go-backend/internal/binance/client.go`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `DefaultPingTimeout` | **60s** | `SetReadDeadline` for reading messages. If no message in 60s, connection is considered dead. |
| `DefaultWriteTimeout` | **10s** | Write deadline for ping responses and message writes. |
| `HandshakeTimeout` | **10s** | WebSocket dialer handshake timeout. |
| `DefaultConnectionExpiry` | **23h 30m** | Proactive reconnect before Binance's 24h WebSocket limit. |
| Read buffer | 1024 bytes | `gorilla/websocket` default read buffer. |
| Write buffer | 1024 bytes | `gorilla/websocket` default write buffer. |

**Test coverage:** `TestClient_StartStop` — verifies client lifecycle. No dedicated timeout tests for ping/write/expiry.

### 1.5 WebSocket Hub (`go-backend/internal/server/ws.go`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `WriteWait` | **10s** | Time allowed to write a message to a peer before closing. |
| `PongWait` | **60s** | Time allowed to read the next pong from a peer. |
| `PingPeriod` | **54s** | `(PongWait * 9) / 10` — sends pings well before pong deadline. |
| Shutdown timeout | **10s** | `context.WithTimeout` for HTTP server graceful shutdown. |

**Test coverage:** `TestBroadcastToAllClients` — verifies messages reach clients. No dedicated ping/pong timeout tests.

### 1.6 Frontend WebSocket Client (`frontend/js/ws-client.js`)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| `RECONNECT_BASE_DELAY` | **1000ms** | Initial reconnect delay. |
| `MAX_RECONNECT_DELAY` | **30000ms** | Cap on exponential backoff. |
| Backoff formula | `1000 * 2^attempts` | Capped at 30s. Sequence: 1s → 2s → 4s → 8s → 16s → 30s → 30s... |

**Test coverage:** `test_handles_reconnect` in `ws-client.spec.js` — verifies overlay appears on disconnect and reconnect attempts occur.

---

## 2. Retry Policies

### 2.1 ML Service Client — Primary (`go-backend/internal/mlclient/client.go`)

| Policy | Value |
|--------|-------|
| **Max retries** | 2 (initial + 2 = 3 total attempts) |
| **Retry delay** | 100ms between attempts |
| **Retryable status codes** | 503 (Service Unavailable), 504 (Gateway Timeout) |
| **Non-retryable** | All other codes: 422 (validation), 4xx, 5xx except 503/504 |
| **Timeout retry?** | **No** — timeouts are connection errors, retried (continue on `err != nil`), but no separate timeout-specific retry |
| **Invalid JSON retry?** | No — parse errors are returned immediately |

**Test coverage:**
- `TestPredictMLUnavailable` — verifies 3 total attempts (initial + 2 retry) on 503
- `TestPredictNoRetryOn422` — verifies only 1 attempt (no retry) on 422
- `TestPredictRetryOn504` — verifies retry on 504, success on 2nd attempt

### 2.2 ML Client — Legacy (`go-backend/internal/ml/client.go`)

**No built-in retry.** The `Predict()` and `Health()` methods make a single request. Errors are returned immediately.

### 2.3 Binance WebSocket Reconnection (`go-backend/internal/binance/client.go`)

| Policy | Value |
|--------|-------|
| **Strategy** | Exponential backoff |
| **Initial backoff** | 1s |
| **Max backoff** | 30s |
| **Multiplier** | 2x |
| **Sequence** | 1s → 2s → 4s → 8s → 16s → 30s → 30s... |
| **Reset** | Backoff resets to initial on clean disconnect |
| **Clean exit** | `CloseNormalClosure` → no reconnect, return nil |
| **Proactive reconnect** | At 23h 30m connection age (before Binance 24h limit) |

**Test coverage:**
- `TestReconnectBackoff` — verifies exponential backoff sequence up to max
- `TestClient_StartStop` — verifies lifecycle

### 2.4 Frontend WebSocket Reconnection (`frontend/js/ws-client.js`)

| Policy | Value |
|--------|-------|
| **Strategy** | Exponential backoff |
| **Base delay** | 1s |
| **Max delay** | 30s |
| **Multiplier** | 2x |
| **Sequence** | 1s → 2s → 4s → 8s → 16s → 30s → 30s... |
| **Reset** | Attempts reset to 0 on successful `onopen` |

**Test coverage:** `test_handles_reconnect` in `ws-client.spec.js` — verifies reconnect overlay appears after close.

### 2.5 ML Service Retry Handling (`ml-service/main.py`)

**Server-side, not retry.** The service itself does not implement retry logic. It relies on:
- Client-side retries (go-backend `mlclient`)
- Timeout wrappers for prediction execution
- Proper HTTP status codes (503, 422, 500) to signal retryability

---

## 3. Rate Limiting Strategies

### 3.1 WebSocket Client Max Connections (`go-backend/internal/server/ws.go`)

| Parameter | Value |
|-----------|-------|
| `MaxClients` | **10** |
| Behavior on limit | Connection is closed immediately with log message |

**Test coverage:** `TestMaxClients` — verifies at most 10 clients are accepted.

### 3.2 Prediction Serialization (`go-backend/internal/server/ws.go`)

| Mechanism | Implementation |
|-----------|---------------|
| **Stale prediction prevention** | `predictMu sync.Mutex` + `predicting bool` flag |
| Behavior | If a prediction is in-flight, new prediction requests are silently dropped |
| No queuing | Dropped predictions are NOT queued for later execution |

**Test coverage:** `TestPredictionBroadcastOnBufferFull` — verifies exactly 1 predict call when buffer is full. `TestPredictionNotSentWhenBufferNotFull` — verifies 0 predict calls when buffer is partial.

### 3.3 Binance WebSocket Deduplication (`go-backend/internal/binance/client.go`)

| Mechanism | Implementation |
|-----------|---------------|
| **Key** | `close_time` (as Unix milliseconds) |
| **Cleanup** | When `seenCandles` exceeds 100 entries, oldest 50+ are removed |
| **Ring buffer** | Holds exactly 60 candles, oldest evicted on overflow |

**Test coverage:** `TestDeduplicateByCloseTime` — verifies duplicate detection.

### 3.4 Non-blocking Channel Sends (`go-backend/internal/server/ws.go`)

| Mechanism | Behavior |
|-----------|----------|
| Broadcast channel | 256-buffered `chan schemas.Candle` |
| Client send channel | 256-buffered `chan []byte` |
| Write pump drain | If client's send buffer is full, client is unregistered (dropped) |

---

## 4. Error Handling Patterns

### 4.1 Go Backend Error Typing

| Component | Error Type | File |
|-----------|-----------|------|
| ML Client | `MLClientError` (wraps status code, operation, underlying error) | `mlclient/client.go:76-92` |
| WebSocket Hub | `ErrMLServiceUnavailable` (sentinel error) | `server/ws.go:15` |

### 4.2 ML Client Error Classification

| Scenario | Error Message | Retryable? |
|----------|--------------|------------|
| 503 Service Unavailable | `"server returned 503"` | Yes |
| 504 Gateway Timeout | `"server returned 504"` | Yes |
| 422 Unprocessable | `"unexpected status code 422: ..."` | No |
| Connection failure/timeout | `"request failed: ..."` | Yes (continue on err) |
| Invalid JSON response | `"failed to parse response: ..."` | No |
| All retries exhausted | `"all retries exhausted: ..."` | N/A (final) |

### 4.3 ML Service Error Response Matrix (`ml-service/main.py`)

| Error Class | HTTP Status | Client Action |
|-------------|------------|---------------|
| `ValidationError` (wrong candle count) | 422 | No retry (validation) |
| `ValidationError` (missing fields) | 422 | No retry (validation) |
| `ValidationError` (non-positive values) | 422 | No retry (validation) |
| `ValidationError` (non-chronological) | 422 | No retry (validation) |
| `ModelUnavailableError` (no model) | 503 | Retry (transient) |
| `ModelUnavailableError` (timeout) | 503 | Retry (transient) |
| `ModelUnavailableError` (inference fail) | 503 | Retry (transient) |
| `HTTPException` (bad request) | 422 | No retry |
| Unhandled `Exception` | 500 | Internal — not retried (mlclient only retries 503/504) |
| Malformed JSON | 422 | No retry |
| Wrong types | 422 | No retry |

**Test coverage:**
- `TestErrorHandling` class (14 tests) — covers all validation scenarios
- `TestMalformedRequest` class (5 tests) — covers malformed inputs
- `TestTimeoutHandling` class (2 tests) — covers slow request and error message safety
- `TestErrorLogging` class (2 tests) — covers logging without leak

### 4.4 WebSocket Error Messages Broadcast to Frontend

| Status String | Trigger | Frontend Action |
|--------------|---------|-----------------|
| `"connected"` | Client joins hub | Status bar green, restore predictions if recovering |
| `"disconnected"` | WS close handler fires | Status bar red |
| `"error"` | WS error handler fires | Status bar red |
| `"reconnecting"` | Auto-reconnect in progress | Reconnect overlay shown |
| `"ml_error"` | `triggerPrediction` fails | Status bar red, clear predictions from chart |
| `"ML unavailable"` / `"ml_unavailable"` | ML service health check fails | Status bar orange, clear predictions, hide predicted series |

### 4.5 Frontend Error Handling (`frontend/js/app.js`)

| Scenario | Behavior |
|----------|----------|
| `mlAvailable = false` | `handlePrediction()` returns early — no prediction rendering |
| ML becomes available again | `restorePredictionsToChart()` re-shows predicted SMA series |
| Invalid candle data | `console.warn()`, returns early (no crash) |
| Invalid prediction data | `console.warn()`, returns early (no crash) |
| Missing `ChartModule` | `console.error()`, init aborts |
| Missing `WSClient` | `console.error()`, init aborts |
| Callback errors | `try/catch` in `emit()` — one bad callback doesn't break others |

### 4.6 Frontend Predicted Series Cleanup (`frontend/js/app.js:111-127`)

When ML becomes unavailable, `clearPredictionsFromChart()` clears:
1. `predictedSmaSeries.setData([])` — clears SMA line data
2. `predictedSmaData.length = 0` — resets SMA data array
3. `predictedSmaSeries.applyOptions({ visible: false })` — hides SMA line
4. `predictedSeries.setData([])` — clears predicted candles
5. `predictedCandles.length = 0` — resets predicted candle array

**Test coverage:** `ml-degradation.spec.js` — 7 tests covering ml_unavailable, ml_error, predicted series clearing, recovery, real candle continuity, and error suppression.

---

## 5. Circuit Breaker / Degradation Behaviors

### 5.1 ML Prediction Gating (`go-backend/internal/server/ws.go:159-184`)

```
triggerPrediction():
  1. Lock predictMu
  2. If predicting == true: unlock and return (DROP)
  3. Set predicting = true, unlock
  4. Spawn goroutine:
     a. Call mlClient.Predict(candles)
     b. On error: BroadcastStatus("ml_error") → frontend shows "Prediction error"
     c. On success: BroadcastPrediction(result)
     d. defer: predictMu.Lock, predicting = false, unlock
```

This is a **circuit breaker** at the prediction level — one in-flight prediction blocks all subsequent triggers. Each new candle re-evaluates: if the previous prediction goroutine is still running, the new trigger is silently dropped.

### 5.2 ML Unavailable Degradation (Frontend)

| State | Visual | Chart Behavior |
|-------|--------|----------------|
| ML available, prediction received | Normal | Predicted SMA visible, predicted candles visible |
| ML unavailable (`ml_unavailable` / `ML unavailable`) | Orange status bar "Prediction unavailable" | Predicted SMA hidden, predicted candles cleared |
| ML error (`ml_error` / `ML error`) | Red status bar "Prediction error" | Predicted SMA hidden, predicted candles cleared |
| Recovery (non-ML status received) | Green/yellow per status | Predicted SMA restored to visible |

### 5.3 Connection Failure — Frontend Overlay (`frontend/js/ws-client.js:21-62`)

When WebSocket disconnects:
1. Full-screen dark overlay with "Reconnecting..." message
2. Auto-reconnect with exponential backoff
3. Overlay removed on successful reconnect
4. `ws-reconnected` custom event dispatched for downstream recovery

### 5.4 Connection Failure — Server Side (`go-backend/internal/server/ws.go`)

When a client's send buffer fills up (256 messages):
- Client is unregistered from the hub
- Channel is closed
- Connection is closed
- No queuing or backpressure — client is simply dropped

### 5.5 Zero-Volume Candle Handling (`go-backend/internal/binance/client.go:252-259`)

When a candle has `Volume == 0`:
- All OHLC values are set to `lastClose` (previous candle's close)
- Volume remains 0
- Candle is NOT dropped — it's kept in the ring buffer

### 5.6 Deduplication (`go-backend/internal/binance/client.go:273-291`)

By `close_time` (Unix milliseconds):
- If seen before → skip (continue reading next message)
- If new → store, send to channel
- Memory management: cleanup when >100 entries, keep most recent 50

---

## 6. Failure Scenario Matrix

| Scenario | Source | Behavior | User-Visible |
|----------|--------|----------|--------------|
| ML service timeout (500ms) | mlclient → ML service | Retry 2x, then return error | `ml_error` status → red bar, predictions cleared |
| ML service 503 (no model) | mlclient → ML service | Retry 2x, then return `MLClientError` | `ml_error` status → red bar, predictions cleared |
| ML service 503 via health check | mlclient health → ML service | Returns `model_loaded = false` | Depends on caller |
| ML service 504 (gateway) | mlclient → ML service | Retry 2x, then return error | `ml_error` status → red bar |
| ML service 422 (validation) | mlclient → ML service | No retry, immediate error | `ml_error` status → red bar |
| ML service crash/container down | mlclient → ML service | Connection error on each attempt, retry 2x, fail | `ml_error` status → red bar |
| Binance WS disconnect | Binance client | Exponential backoff reconnect (1s→30s) | Reconnect overlay, "Reconnecting..." |
| Binance WS 24h expiry | Binance client | Proactive reconnect at 23h30m | Brief reconnect, candles resume |
| Frontend WS disconnect | ws-client.js | Exponential backoff reconnect (1s→30s) | Dark overlay "Reconnecting..." |
| Server WebSocket max clients (11th) | WebSocket hub | Connection rejected, closed immediately | Cannot connect (silent, retry later) |
| Client send buffer full | writePump | Client unregistered and disconnected | Drop then reconnect |
| Two simultaneous predictions | triggerPrediction | Second is silently dropped (predicting=true) | No visual change |
| ML recovers after outage | Frontend handleStatus | `restorePredictionsToChart()` called | Predicted SMA reappears |
| Invalid candle from Binance | ParseKlineMessage | Error logged, message skipped | No chart update for that tick |
| Zero-volume candle from Binance | Binance client | Sets OHLC to lastClose, volume=0 | Flattened candle rendered |
| Shutdown signal (SIGINT/SIGTERM) | HTTP server | Graceful: close Binance, WS hub, then HTTP (10s) | Clean disconnect |
| Prediction timeout inside ML service (30s) | ML service | `asyncio.TimeoutError` → 503 | mlclient retry 2x, then `ml_error` |
| Internal server error in ML service | ML service | 500 response logged internally, 503 to client | `ml_error` (not retried — 500 ≠ 503/504) |

---

## 7. Test Coverage Summary

| Configuration | Test File | Test Function | What It Covers |
|--------------|-----------|---------------|----------------|
| ML timeout (500ms) | `mlclient/client_test.go` | `TestPredictTimeout` | Client times out when server is slow |
| ML retry on 503 | `mlclient/client_test.go` | `TestPredictMLUnavailable` | 3 attempts on 503 |
| ML no-retry on 422 | `mlclient/client_test.go` | `TestPredictNoRetryOn422` | 1 attempt on 422 |
| ML retry on 504 | `mlclient/client_test.go` | `TestPredictRetryOn504` | Retry succeeds on 2nd attempt |
| ML invalid JSON | `mlclient/client_test.go` | `TestPredictInvalidJSON` | Parse error returned |
| ML default config | `mlclient/client_test.go` | `TestNewClientDefaultConfig` | 500ms, 2 retries, 100ms delay |
| ML health check | `mlclient/client_test.go` | `TestHealthCheckModelLoaded` / `TestHealthCheckModelNotLoaded` | Both paths |
| Legacy ML timeout | `ml/client_test.go` | `TestMLClient_Predict_ConnectionFailed` | Connection failure detected |
| Legacy ML env config | `ml/client_test.go` | `TestDefaultConfig_EnvVars` | `ML_TIMEOUT` env override |
| Binance backoff | `binance/client_test.go` | `TestReconnectBackoff` | Exponential backoff sequence |
| Binance dedup | `binance/client_test.go` | `TestDeduplicateByCloseTime` | Duplicate detection |
| Ring buffer | `buffer/buffer_test.go` | 7 tests | Capacity, eviction, snapshot copy, concurrency |
| WS prediction trigger | `server/ws_test.go` | `TestPredictionBroadcastOnBufferFull` | 1 prediction call |
| WS no-trigger partial | `server/ws_test.go` | `TestPredictionNotSentWhenBufferNotFull` | 0 prediction calls |
| WS ML error broadcast | `server/ws_test.go` | `TestMLErrorBroadcastsStatus` | `ml_error` status sent |
| WS max clients | `server/ws_test.go` | `TestMaxClients` | At most 10 clients |
| HTTP graceful shutdown | `server/http_test.go` | `TestGracefulShutdown` / `TestSignalHandling` | CloseBinance, CloseWSHub called |
| Frontend reconnect | `ws-client.spec.js` | `test_handles_reconnect` | Overlay appears on disconnect |
| ML unavailable status | `ml-degradation.spec.js` | 7 tests | Status bar, predicted clear, recovery, candle continuity, error suppression, ml_error, predicted candles |
| ML error handling | `test_error_handling.py` | 14 tests | All validation and error response cases |
| ML timeout handling | `test_error_handling.py` | 2 tests | Slow request safety, error message isolation |

### Coverage Gaps (Documented, Not Tested)

| Configuration | Gap |
|--------------|-----|
| Binance ping timeout (60s) | No dedicated test for ping timeout triggering reconnect |
| Binance write timeout (10s) | No dedicated test for write timeout |
| Binance connection expiry (23h30m) | No test for expiry-triggered reconnect |
| WebSocket Hub ping/pong cycle (54s/60s) | No dedicated ping/pong timeout test |
| WebSocket send buffer overflow | No test for 256-message buffer overflow triggering unregister |
| Multiple rapid predictions with mutex | No test explicitly verifying second prediction is dropped while first in-flight (single-prediction test only verifies call count of 1) |
| Exponential backoff reset on clean disconnect | No test for backoff reset behavior |
| Frontend exponential backoff exact values | `test_handles_reconnect` only checks overlay, not backoff sequence |

---

## 8. Summary of Resilience Tiers

| Tier | Component | Behavior |
|------|-----------|----------|
| **L1: Fast Fail** | ML client timeout | 500ms → retry 2x → error |
| **L2: Retry & Backoff** | ML client (503/504), Binance WS, Frontend WS | Up to 3 total attempts / exponential to 30s max |
| **L3: Circuit Breaker** | Prediction serialization (mutex) | One in-flight, drops new requests |
| **L4: Degradation** | Frontend ML unavailable | Orange indicator, predictions hidden, candles continue |
| **L5: Self-Healing** | ML recovery → predictions restored | Frontend auto-restores predicted SMA on any non-ML status |
| **L6: Graceful Stop** | HTTP shutdown, Binance/proactive expiry | 10s drain, close all connections |

---

## Appendix: Configuration Quick Reference

```bash
# ML Client (go-backend/internal/mlclient)
timeout = 500ms (per request)
retries = 2 (max, on 503/504)
retry_delay = 100ms

# ML Client Legacy (go-backend/internal/ml)
ML_TIMEOUT = 10s (env: seconds)
ML_SERVICE_URL = http://ml-service:8000

# Binance WS
initial_backoff = 1s
max_backoff = 30s
backoff_multiplier = 2
connection_expiry = 23h30m
ping_timeout = 60s
write_timeout = 10s
handshake_timeout = 10s

# WebSocket Hub
max_clients = 10
write_wait = 10s
pong_wait = 60s
ping_period = 54s
send_buffer = 256 messages
broadcast_buffer = 256 candles

# HTTP Server
shutdown_timeout = 10s

# ML Service (Python)
REQUEST_TIMEOUT = 30s
model_path = models/model.joblib

# Frontend WS
reconnect_base = 1s
reconnect_max = 30s
backoff = base * 2^attempts
```
