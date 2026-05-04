# WebSocket Client Frontend - Learnings

## Implementation Summary

Created `frontend/js/ws-client.js` - a WebSocket client with auto-reconnect functionality.

## Key Features Implemented

1. **Connection Management**
   - Connects to configurable WebSocket URL (default: `ws://localhost:8080/ws`)
   - Prevents duplicate connection attempts
   - Clean disconnect with proper cleanup

2. **Message Parsing**
   - Parses incoming JSON messages by `type` field
   - Supported message types:
     - `status` → invokes status callback
     - `kline` → invokes candle callback with candle data
     - `prediction` → invokes prediction callback (placeholder for Wave 5)
     - `indicator` → invokes indicator callback (placeholder for Wave 5)

3. **Auto-Reconnect**
   - Exponential backoff: 1s → 2s → 4s → 8s → max 30s
   - Shows "Reconnecting..." overlay during disconnect
   - Emits `ws-reconnected` custom event on successful reconnect
   - Clears reconnect timer on disconnect

4. **API Surface**
   - `connect(url)` - Connect to WebSocket server
   - `disconnect()` - Close connection and stop reconnect attempts
   - `onCandle(callback)` - Register candle data callback
   - `onStatus(callback)` - Register status update callback
   - `onPrediction(callback)` - Register prediction callback
   - `onIndicator(callback)` - Register indicator callback
   - `isConnected` (getter) - Check connection state

## Testing Strategy

- Created Playwright tests in `frontend-tests/ws-client.spec.js`
- Tests use mock WebSocket server for isolation
- Test cases:
  1. `test_parses_kline_message` - Verifies kline message parsing
  2. `test_handles_reconnect` - Verifies reconnect overlay appears on disconnect

## Design Decisions

1. **Module Pattern**: Used IIFE (Immediately Invoked Function Expression) with global export for browser compatibility
2. **Callback Registration**: Multiple callbacks can be registered for each event type
3. **Error Handling**: Callback errors are caught and logged without breaking other callbacks
4. **Overlay Management**: Dynamic DOM element creation for reconnect overlay
5. **Testing Helpers**: Exposed `RECONNECT_BASE_DELAY` and `MAX_RECONNECT_DELAY` for test configuration

## Integration Points

- Task 12 (Chart): `onCandle()` provides data for chart updates
- Task 13 (Status Bar): `onStatus()` provides status updates
- Backend: Connects to Go WebSocket server at `/ws` endpoint
