# Binance WebSocket Client Learnings

## Implementation Details

### 1. Binance WebSocket Format
- URL: `wss://stream.binance.com:9443/ws/btcusdt@kline_1s`
- Message format: JSON with nested kline data
- Key fields:
  - `e`: Event type ("kline")
  - `E`: Event time (milliseconds)
  - `s`: Symbol
  - `k`: Kline object containing OHLCV data
    - `t`: Start time
    - `T`: Close time (used for deduplication)
    - `o`, `h`, `l`, `c`: Open, High, Low, Close (strings)
    - `v`: Volume (string)

### 2. Design Patterns Used
- **Exponential Backoff**: 1s → 2s → 4s → 8s → max 30s
- **Connection Expiry**: Proactive reconnection at 23.5 hours to avoid forced disconnect
- **Ping/Pong**: Automatic response within 60s timeout
- **Deduplication**: Map-based tracking by close_time with automatic cleanup
- **Zero-Volume Handling**: OHLC set to last close price when volume = 0

### 3. Thread Safety
- Used `sync.RWMutex` for safe access to shared state
- Deduplication map protected during read/write operations
- Graceful shutdown with `sync.WaitGroup`

### 4. Testing Approach (TDD)
- Tests written before implementation
- TestParseKlineMessage: Validates JSON parsing to Candle struct
- TestParseKlineMessage_MissingFields: Error handling for incomplete data
- TestDeduplicateByCloseTime: Ensures no duplicate candles emitted
- TestReconnectBackoff: Verifies exponential backoff sequence
- Additional tests for zero-volume handling and client lifecycle

### 5. Configuration
All timeouts and backoff values are configurable via Config struct:
- WSURL: WebSocket endpoint
- InitialBackoff/MaxBackoff: Reconnection timing
- ConnectionExpiry: 23.5 hours for proactive reconnect
- PingTimeout: 60 seconds for ping/pong

## Files Created
- `go-backend/internal/binance/client.go`: Main client implementation
- `go-backend/internal/binance/client_test.go`: TDD test suite

## Dependencies
- `github.com/gorilla/websocket v1.5.3`: WebSocket client library

## Evidence
All tests pass with race detection enabled:
- TestParseKlineMessage ✓
- TestParseKlineMessage_MissingFields ✓
- TestDeduplicateByCloseTime ✓
- TestReconnectBackoff ✓
- TestClient_StartStop ✓
- TestParseKlineMessage_ZeroVolume ✓
