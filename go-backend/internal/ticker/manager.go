package ticker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"predictor/internal/binance"
	"predictor/internal/buffer"
	"predictor/internal/schemas"
)

type TickerManager struct {
	mu             sync.RWMutex
	tickers        map[string]*tickerState
	bufferCapacity int
	restBaseURL    string
	outputChan     chan schemas.Candle
	ctx            context.Context
	cancel         context.CancelFunc
}

type tickerState struct {
	buffer    *buffer.RingBuffer
	client    *binance.Client
	timeframe string
	symbol    string
}

func NewManager(bufferCapacity int, restBaseURL string) *TickerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TickerManager{
		tickers:        make(map[string]*tickerState),
		bufferCapacity: bufferCapacity,
		restBaseURL:    restBaseURL,
		outputChan:     make(chan schemas.Candle, 1024),
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (tm *TickerManager) Subscribe(symbol, timeframe string) error {
	symbolUpper := strings.ToUpper(symbol)

	tm.mu.Lock()
	if _, exists := tm.tickers[symbolUpper]; exists {
		tm.mu.Unlock()
		return fmt.Errorf("ticker %s already subscribed", symbolUpper)
	}

	candleChan := make(chan schemas.Candle, 256)
	buf := buffer.New(tm.bufferCapacity)

	streamURL := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@kline_%s",
		strings.ToLower(symbol), timeframe)

	binanceConfig := binance.DefaultConfig()
	binanceConfig.WSURL = streamURL

	client := binance.NewClient(binanceConfig, candleChan)

	state := &tickerState{
		buffer:    buf,
		client:    client,
		timeframe: timeframe,
		symbol:    symbolUpper,
	}

	tm.tickers[symbolUpper] = state
	tm.mu.Unlock()

	// Fetch historical data before starting live stream
	tm.loadHistory(symbolUpper, timeframe, buf)

	if err := client.Start(tm.ctx); err != nil {
		tm.mu.Lock()
		delete(tm.tickers, symbolUpper)
		tm.mu.Unlock()
		return fmt.Errorf("failed to start client for %s: %w", symbolUpper, err)
	}

	go tm.consumeCandles(state)
	return nil
}

func (tm *TickerManager) Unsubscribe(symbol string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	symbolUpper := strings.ToUpper(symbol)
	state, exists := tm.tickers[symbolUpper]
	if !exists {
		return fmt.Errorf("ticker %s not found", symbolUpper)
	}

	state.client.Stop()
	delete(tm.tickers, symbolUpper)
	return nil
}

func (tm *TickerManager) ChangeTimeframe(symbol, timeframe string) error {
	symbolUpper := strings.ToUpper(symbol)

	tm.mu.Lock()
	state, exists := tm.tickers[symbolUpper]
	if !exists {
		tm.mu.Unlock()
		return fmt.Errorf("ticker %s not found", symbolUpper)
	}

	streamURL := fmt.Sprintf("wss://stream.binance.com:9443/ws/%s@kline_%s",
		strings.ToLower(symbol), timeframe)

	if err := state.client.ChangeStreamURL(streamURL); err != nil {
		tm.mu.Unlock()
		return err
	}
	state.timeframe = timeframe
	state.buffer.Clear()
	tm.mu.Unlock()

	// Fetch historical data for the new timeframe
	tm.loadHistory(symbolUpper, timeframe, state.buffer)
	return nil
}

// loadHistory fetches historical klines from Binance REST API and fills the buffer.
func (tm *TickerManager) loadHistory(symbol, interval string, buf *buffer.RingBuffer) {
	if tm.restBaseURL == "" {
		return
	}

	limit := binance.HistoryCandleCount(interval)
	log.Printf("Fetching %d historical candles for %s %s...", limit, symbol, interval)

	candles, err := binance.FetchKlines(tm.restBaseURL, symbol, interval, limit)
	if err != nil {
		log.Printf("Failed to fetch historical klines for %s %s: %v", symbol, interval, err)
		return
	}

	for _, c := range candles {
		buf.Append(c)
	}

	log.Printf("Loaded %d historical candles for %s %s", len(candles), symbol, interval)
}

func (tm *TickerManager) consumeCandles(state *tickerState) {
	for {
		select {
		case candle, ok := <-state.client.OutputChan():
			if !ok {
				return
			}
			state.buffer.Append(candle)
			select {
			case tm.outputChan <- candle:
			case <-tm.ctx.Done():
				return
			}
		case <-tm.ctx.Done():
			return
		}
	}
}

func (tm *TickerManager) GetBuffer(symbol string) (*buffer.RingBuffer, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	state, exists := tm.tickers[strings.ToUpper(symbol)]
	if !exists {
		return nil, false
	}
	return state.buffer, true
}

func (tm *TickerManager) GetSnapshot(symbol string) []schemas.Candle {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	state, exists := tm.tickers[strings.ToUpper(symbol)]
	if !exists {
		return nil
	}
	return state.buffer.Snapshot()
}

func (tm *TickerManager) GetDefaultSnapshot() []schemas.Candle {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	if state, ok := tm.tickers["BTCUSDT"]; ok {
		return state.buffer.Snapshot()
	}
	for _, state := range tm.tickers {
		return state.buffer.Snapshot()
	}
	return []schemas.Candle{}
}

func (tm *TickerManager) ListTickers() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	tickers := make([]string, 0, len(tm.tickers))
	for symbol := range tm.tickers {
		tickers = append(tickers, symbol)
	}
	return tickers
}

func (tm *TickerManager) HasTicker(symbol string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, exists := tm.tickers[strings.ToUpper(symbol)]
	return exists
}

func (tm *TickerManager) GetTimeframe(symbol string) (string, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	state, exists := tm.tickers[strings.ToUpper(symbol)]
	if !exists {
		return "", false
	}
	return state.timeframe, true
}

func (tm *TickerManager) StopAll() {
	tm.cancel()
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for _, state := range tm.tickers {
		state.client.Stop()
	}
	tm.tickers = make(map[string]*tickerState)
}

func (tm *TickerManager) OutputChan() <-chan schemas.Candle {
	return tm.outputChan
}
