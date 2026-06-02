# Multi-Ticker Support with Tab UI

## TL;DR

> **Quick Summary**: Add support for multiple tickers (BTCUSDT, ETHUSDT, EURUSDT) streaming simultaneously, with a Chrome-style tab bar to switch between them.
>
> **Deliverables**:
> - Backend: per-ticker buffers and multi-stream Binance connections
> - Frontend: tab bar UI with add/remove/switch
> - WebSocket protocol: ticker subscription messages

## Changes by Component

### 1. Schemas (`go-backend/internal/schemas/schemas.go`)
- Add `TickerSubscribeRequest` — `{type:"ticker_subscribe", ticker, timeframe}`
- Add `TickerUnsubscribeRequest` — `{type:"ticker_unsubscribe", ticker}`
- Add `TickerSubscriptionResponse` — `{type:"ticker_sub", ticker, timeframe, status, message}`
- Add `TickerListMessage` — `{type:"ticker_list", tickers:[...]}`
- Extend `TimeframeChangeRequest` with optional `Ticker` field

### 2. Backend Multi-Ticker Manager (`go-backend/internal/ticker/manager.go`)
- Create `TickerManager` struct with:
  - `map[string]*buffer.RingBuffer` — per-ticker buffers
  - `map[string]*binance.Client` — per-ticker Binance streams
  - `map[string]chan schemas.Candle` — per-ticker candle channels
  - Subscribe/Unsubscribe methods
  - GetSnapshot(ticker) method
  - ListTickers() method

### 3. WebSocket Hub (`go-backend/internal/server/ws.go`)
- Add `tickerManager` field
- Handle `ticker_subscribe` messages in readPump
- Handle `ticker_unsubscribe` messages
- Broadcast candles with ticker info
- Send ticker list on new client connect

### 4. Application (`go-backend/cmd/predictor/main.go`)
- Replace single buffer/binance client with TickerManager
- Start BTCUSDT as default ticker on startup
- Wire TickerManager into WebSocket hub

### 5. Frontend HTML (`frontend/index.html`)
- Add tab bar container above chart
- Default BTCUSDT tab
- "+" button to add new tickers

### 6. Frontend CSS (`frontend/css/style.css`)
- Chrome-style tab bar styling
- Active/inactive tab states
- Close button on tabs
- "+" button styling

### 7. Frontend JS (`frontend/js/app.js`)
- Tab state management (array of open tickers)
- Per-ticker candle arrays
- Tab switching (save/restore chart state)
- Tab add/close logic
- Send ticker_subscribe/ticker_unsubscribe via WebSocket
- Per-ticker timeframe tracking

### 8. Frontend WS Client (`frontend/js/ws-client.js`)
- `sendTickerSubscribe(ticker, timeframe)`
- `sendTickerUnsubscribe(ticker)`
- Handle `ticker_sub` response messages
- Handle `ticker_list` messages

## Implementation Order

**Wave A** (backend — can run in parallel):
- A1: Update schemas
- A2: Create TickerManager
- A3: Update WebSocket hub
- A4: Update Application wiring

**Wave B** (frontend — after Wave A):
- B1: Update HTML with tab bar
- B2: Update CSS for tab styling
- B3: Update ws-client.js
- B4: Update app.js with tab logic

## Default Tickers

Available on startup:
- BTCUSDT (default)
- ETHUSDT
- EURUSDT
- SOLUSDT
- BNBUSDT
