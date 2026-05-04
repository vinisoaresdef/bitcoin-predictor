# Wave 1 Task 6 Learnings - WebSocket Server

## Implementation Summary

Created `go-backend/internal/server/ws.go` with a WebSocket server using the hub pattern for broadcasting candles to frontend clients.

## Key Design Patterns

### Hub Pattern
- Used the hub pattern for managing multiple WebSocket clients
- Single hub maintains a registry of all connected clients
- Broadcast channel fans out messages to all clients
- Thread-safe with RWMutex for client map access

### Client Management
- Each client gets its own `send` channel (buffered 256)
- Separate goroutines for readPump and writePump
- Graceful disconnect handling via unregister channel
- Maximum 10 concurrent clients enforced

### Buffer Interface
- Defined `BufferSnapshotProvider` interface for loose coupling
- Allows WebSocket server to work with any buffer implementation
- Provides `GetSnapshot() []schemas.Candle` method

## Message Formats

### Status Message
```json
{"type":"status","status":"connected","timestamp":"..."}
```

### Kline Message
```json
{"type":"kline","candle":{"symbol":"BTCUSDT","interval":"1s","open":...,"high":...,"low":...,"close":...,"volume":...,"close_time":"...","timestamp":"..."}}
```

## Constants
- `MaxClients = 10` - Maximum concurrent connections
- `WriteWait = 10s` - Write timeout
- `PongWait = 60s` - Read timeout for pong responses
- `PingPeriod = 54s` - Ping interval (9/10 of PongWait)

## Testing Approach
- TDD with 4 required tests plus 2 additional tests
- All tests pass with `-race` flag
- Tests verify concurrent access patterns

## Dependencies
- `github.com/gorilla/websocket v1.5.3` for WebSocket implementation
