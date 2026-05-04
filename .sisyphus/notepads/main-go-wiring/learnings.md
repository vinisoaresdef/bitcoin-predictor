# Main.go Wiring Implementation - Learnings

## Architecture Pattern

The application uses a **fan-out pattern** for candle distribution:

1. **Binance Client** → writes to `candleChan`
2. **Buffer Consumer** → reads from `candleChan`, appends to buffer, writes to `broadcastChan`
3. **WebSocket Hub** → reads from `broadcastChan`, broadcasts to all connected clients

This ensures:
- Every candle is stored in the buffer exactly once
- Every candle is broadcast to WebSocket clients exactly once
- No race conditions from multiple readers on the same channel

## Component Initialization Order

1. Create channels (candleChan, broadcastChan)
2. Create ring buffer
3. Create Binance client
4. Create WebSocket hub
5. Create HTTP server
6. Start WebSocket hub (goroutine)
7. Start buffer consumer (goroutine)
8. Start Binance client
9. Start HTTP server (blocks)

## Key Implementation Details

### Channel Buffer Sizes
- `candleChan`: 256 (handles bursts from Binance)
- `broadcastChan`: 256 (handles bursts from buffer consumer)

### Adapter Pattern
Used to adapt internal types to server interface requirements:
- `bufferAdapter` implements `BufferSnapshotProvider` (exposes `GetSnapshot()` wrapping `Snapshot()`)
- `binanceClientAdapter` implements `BinanceClient` interface
- `wsHubAdapter` implements `WSHub` interface

### Graceful Shutdown Sequence
1. Cancel context to signal goroutines
2. Stop Binance client
3. Stop WebSocket hub
4. Close candle channel
5. Close broadcast channel
6. Wait for WaitGroup with timeout

### Configuration
Environment variables with defaults:
- `HTTP_PORT` (default: 8080)
- `FRONTEND_DIR` (default: ./frontend/dist)
- `BINANCE_WS_URL` (default: wss://stream.binance.com:9443/ws/btcusdt@kline_1s)
- `BUFFER_SIZE` (hardcoded: 60)

## Race Condition Prevention

The critical fix was separating the read channels:
- Initially both WebSocketHub and buffer consumer read from `candleChan`
- This caused race where each candle went to only one reader
- Solution: Buffer consumer exclusively reads `candleChan`, then fans out to `broadcastChan` for WebSocket

## Testing Strategy

Integration tests cover:
- Component initialization (TestMainStartupSequence)
- Full pipeline: candle → buffer → broadcast (TestFullPipeline)
- Buffer capacity and eviction (TestBufferCapacityLimit)
- Concurrent processing (TestConcurrentCandleProcessing)
- Graceful shutdown (TestGracefulStop)
- Configuration loading (TestLoadConfigDefaults, TestLoadConfigFromEnv)
