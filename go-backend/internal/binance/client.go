package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"predictor/internal/schemas"
)

const (
	// DefaultBinanceWSURL is the default Binance WebSocket stream URL
	DefaultBinanceWSURL = "wss://stream.binance.com:9443/ws/btcusdt@kline_1s"

	// DefaultInitialBackoff is the initial reconnection backoff duration
	DefaultInitialBackoff = 1 * time.Second

	// DefaultMaxBackoff is the maximum reconnection backoff duration
	DefaultMaxBackoff = 30 * time.Second

	// DefaultBackoffMultiplier is the multiplier for exponential backoff
	DefaultBackoffMultiplier = 2

	// DefaultConnectionExpiry is the maximum connection duration before proactive reconnection
	DefaultConnectionExpiry = 23*time.Hour + 30*time.Minute

	// DefaultPingTimeout is the timeout for responding to ping messages
	DefaultPingTimeout = 60 * time.Second

	// DefaultWriteTimeout is the timeout for write operations
	DefaultWriteTimeout = 10 * time.Second
)

// Config holds configuration options for the WebSocket client
type Config struct {
	WSURL             string
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	ConnectionExpiry  time.Duration
	PingTimeout       time.Duration
	WriteTimeout      time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		WSURL:             DefaultBinanceWSURL,
		InitialBackoff:    DefaultInitialBackoff,
		MaxBackoff:        DefaultMaxBackoff,
		BackoffMultiplier: DefaultBackoffMultiplier,
		ConnectionExpiry:  DefaultConnectionExpiry,
		PingTimeout:       DefaultPingTimeout,
		WriteTimeout:      DefaultWriteTimeout,
	}
}

type Client struct {
	config       Config
	conn         *websocket.Conn
	candleChan   chan schemas.Candle
	stopChan     chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	seenCandles  map[int64]schemas.Candle
	lastClose    float64
	isRunning    bool
}

// NewClient creates a new Binance WebSocket client
func NewClient(config Config, candleChan chan schemas.Candle) *Client {
	return &Client{
		config:      config,
		candleChan:  candleChan,
		stopChan:    make(chan struct{}),
		seenCandles: make(map[int64]schemas.Candle),
	}
}

// BinanceKlineMessage represents the raw WebSocket message from Binance
type BinanceKlineMessage struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime         int64  `json:"t"`
		CloseTime         int64  `json:"T"`
		Symbol            string `json:"s"`
		Interval          string `json:"i"`
		FirstTradeID      int64  `json:"f"`
		LastTradeID       int64  `json:"L"`
		Open              string `json:"o"`
		High              string `json:"h"`
		Low               string `json:"l"`
		Close             string `json:"c"`
		Volume            string `json:"v"`
		NumTrades         int    `json:"n"`
		IsFinal           bool   `json:"x"`
		QuoteVolume       string `json:"q"`
		ActiveBuyVolume   string `json:"V"`
		ActiveBuyQuoteVol string `json:"Q"`
		Ignore            string `json:"B"`
	} `json:"k"`
}

// Start begins the WebSocket connection and starts processing messages
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("client is already running")
	}
	c.isRunning = true
	c.mu.Unlock()

	c.wg.Add(1)
	go c.run(ctx)

	return nil
}

func (c *Client) ChangeStreamURL(url string) error {
	c.mu.Lock()
	oldURL := c.config.WSURL
	c.config.WSURL = url
	c.seenCandles = make(map[int64]schemas.Candle)
	c.mu.Unlock()

	if oldURL != url && c.conn != nil {
		c.conn.Close()
	}

	return nil
}

func (c *Client) GetStreamURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.WSURL
}

func (c *Client) Stop() error {
	c.mu.Lock()
	if !c.isRunning {
		c.mu.Unlock()
		return nil
	}
	c.isRunning = false
	c.mu.Unlock()

	close(c.stopChan)
	c.wg.Wait()

	if c.conn != nil {
		c.conn.Close()
	}

	return nil
}

func (c *Client) OutputChan() <-chan schemas.Candle {
	return c.candleChan
}

// run is the main connection loop with reconnection logic
func (c *Client) run(ctx context.Context) {
	defer c.wg.Done()

	backoff := c.config.InitialBackoff

	for {
		select {
		case <-c.stopChan:
			return
		case <-ctx.Done():
			return
		default:
		}

		err := c.connectAndProcess(ctx)
		if err != nil {
			// Connection error, apply backoff
			select {
			case <-c.stopChan:
				return
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				// Exponential backoff
				backoff = time.Duration(float64(backoff) * c.config.BackoffMultiplier)
				if backoff > c.config.MaxBackoff {
					backoff = c.config.MaxBackoff
				}
			}
		} else {
			// Clean disconnect, reset backoff
			backoff = c.config.InitialBackoff
		}
	}
}

// connectAndProcess establishes a connection and processes messages
func (c *Client) connectAndProcess(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}

	conn, resp, err := dialer.DialContext(ctx, c.config.WSURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// Set up ping/pong handlers
	conn.SetPingHandler(func(data string) error {
		conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
		return conn.WriteMessage(websocket.PongMessage, []byte(data))
	})

	// Set up connection expiry timer
	connectionExpiry := time.AfterFunc(c.config.ConnectionExpiry, func() {
		c.mu.RLock()
		running := c.isRunning
		c.mu.RUnlock()
		if running {
			conn.Close()
		}
	})
	defer connectionExpiry.Stop()

	// Process messages
	for {
		select {
		case <-c.stopChan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn.SetReadDeadline(time.Now().Add(c.config.PingTimeout))
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		candle, err := ParseKlineMessage(message)
		if err != nil {
			// Log error but continue processing
			continue
		}

		// Deduplicate by close_time
		if c.isDuplicate(candle) {
			continue
		}

		// Handle zero-volume candles
		if candle.Volume == 0 {
			candle.Open = c.lastClose
			candle.High = c.lastClose
			candle.Low = c.lastClose
			candle.Close = c.lastClose
		} else {
			c.lastClose = candle.Close
		}

		// Send to channel (non-blocking)
		select {
		case c.candleChan <- candle:
		case <-c.stopChan:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// isDuplicate checks if a candle has already been seen.
// Partial (non-final) updates for the same close_time are allowed to flow through.
// Once the FINAL candle for a close_time has been seen, subsequent duplicates are blocked.
func (c *Client) isDuplicate(candle schemas.Candle) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	closeTimeMs := candle.CloseTime.UnixMilli()
	last, exists := c.seenCandles[closeTimeMs]

	if !exists {
		c.seenCandles[closeTimeMs] = candle
		if len(c.seenCandles) > 100 {
			c.cleanupOldEntries()
		}
		return false
	}

	if last.IsFinal {
		return true
	}

	c.seenCandles[closeTimeMs] = candle
	return false
}

// cleanupOldEntries removes old entries from the deduplication map
func (c *Client) cleanupOldEntries() {
	// Convert map to slice for sorting
	times := make([]int64, 0, len(c.seenCandles))
	for t := range c.seenCandles {
		times = append(times, t)
	}

	// Sort descending and keep only the most recent 50
	if len(times) > 50 {
		sort.Slice(times, func(i, j int) bool {
			return times[i] > times[j]
		})

		for i := 50; i < len(times); i++ {
			delete(c.seenCandles, times[i])
		}
	}
}

// ParseKlineMessage parses a Binance kline message into a Candle struct
func ParseKlineMessage(data []byte) (schemas.Candle, error) {
	var msg BinanceKlineMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Validate required fields
	if msg.EventType != "kline" {
		return schemas.Candle{}, fmt.Errorf("invalid event type: %s", msg.EventType)
	}

	k := msg.Kline

	// Parse numeric fields
	open, err := strconv.ParseFloat(k.Open, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse open price: %w", err)
	}

	high, err := strconv.ParseFloat(k.High, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse high price: %w", err)
	}

	low, err := strconv.ParseFloat(k.Low, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse low price: %w", err)
	}

	close, err := strconv.ParseFloat(k.Close, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse close price: %w", err)
	}

	volume, err := strconv.ParseFloat(k.Volume, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse volume: %w", err)
	}

	// Convert milliseconds timestamps to time.Time
	closeTime := time.UnixMilli(k.CloseTime)
	timestamp := time.UnixMilli(msg.EventTime)

	return schemas.Candle{
		Symbol:    msg.Symbol,
		Interval:  k.Interval,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		CloseTime: closeTime,
		Timestamp: timestamp,
		IsFinal:   k.IsFinal,
	}, nil
}
