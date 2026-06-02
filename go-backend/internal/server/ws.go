package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"predictor/internal/schemas"
	"predictor/internal/ticker"
)

const (
	MaxClients = 10
	WriteWait  = 10 * time.Second
	PongWait   = 60 * time.Second
	PingPeriod = (PongWait * 9) / 10

	// MaxMessageSize limits incoming WebSocket message size
	MaxMessageSize = 4096
)

// BufferSnapshotProvider defines the interface for getting buffer snapshots
type BufferSnapshotProvider interface {
	GetSnapshot() []schemas.Candle
}

// Client represents a single WebSocket connection
type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

type WebSocketHub struct {
	clients       map[*Client]bool
	broadcast     chan schemas.Candle
	register      chan *Client
	unregister    chan *Client
	buffer        BufferSnapshotProvider
	candleChan    <-chan schemas.Candle
	tickerManager *ticker.TickerManager
	mu            sync.RWMutex
	running       bool
	stopChan      chan struct{}
}

// NewWebSocketHub creates a new WebSocketHub instance
func NewWebSocketHub(buffer BufferSnapshotProvider, candleChan <-chan schemas.Candle) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan schemas.Candle, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		buffer:     buffer,
		candleChan: candleChan,
		stopChan:   make(chan struct{}),
	}
}

func (h *WebSocketHub) SetTickerManager(tm *ticker.TickerManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tickerManager = tm
}

func (h *WebSocketHub) HandleTimeframeChange(tickerSymbol, timeframe string) error {
	h.mu.RLock()
	tm := h.tickerManager
	h.mu.RUnlock()

	if tm == nil {
		return fmt.Errorf("ticker manager not set")
	}
	return tm.ChangeTimeframe(tickerSymbol, timeframe)
}

// Run starts the hub's main event loop
func (h *WebSocketHub) Run() {
	h.mu.Lock()
	h.running = true
	h.mu.Unlock()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if len(h.clients) >= MaxClients {
				h.mu.Unlock()
				close(client.send)
				client.conn.Close()
				log.Printf("Rejected client connection: max clients (%d) reached", MaxClients)
				continue
			}
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected. Total clients: %d", h.ClientCount())

			h.sendStatusMessage(client, "connected")
			h.sendBufferSnapshot(client)

			h.mu.RLock()
			tm := h.tickerManager
			h.mu.RUnlock()

			if tm != nil {
				tickers := tm.ListTickers()
				msg := schemas.TickerListMessage{Type: "ticker_list", Tickers: tickers}
				if data, err := json.Marshal(msg); err == nil {
					select {
					case client.send <- data:
					default:
					}
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				client.conn.Close()
			}
			h.mu.Unlock()
			log.Printf("Client disconnected. Total clients: %d", h.ClientCount())

		case candle := <-h.candleChan:
			h.broadcastCandle(candle)

		case <-h.stopChan:
			return
		}
	}
}

// Stop gracefully stops the hub
func (h *WebSocketHub) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	h.mu.Unlock()

	close(h.stopChan)

	h.mu.Lock()
	for client := range h.clients {
		close(client.send)
		client.conn.Close()
		delete(h.clients, client)
	}
	h.mu.Unlock()
}

// ClientCount returns the current number of connected clients
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// broadcastCandle sends a candle to all connected clients
func (h *WebSocketHub) broadcastCandle(candle schemas.Candle) {
	klineMsg := schemas.KlineMessage{
		Type:   "kline",
		Candle: candle,
	}

	data, err := json.Marshal(klineMsg)
	if err != nil {
		log.Printf("Error marshaling kline message: %v", err)
		return
	}

	h.broadcastData(data)
}

// broadcastData sends raw data to all connected clients
func (h *WebSocketHub) broadcastData(data []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		select {
		case client.send <- data:
		default:
			h.unregister <- client
		}
	}
}

// sendStatusMessage sends a status message to a specific client
func (h *WebSocketHub) sendStatusMessage(client *Client, status string) {
	statusMsg := schemas.StatusMessage{
		Type:      "status",
		Status:    status,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(statusMsg)
	if err != nil {
		log.Printf("Error marshaling status message: %v", err)
		return
	}

	select {
	case client.send <- data:
	default:
		h.unregister <- client
	}
}

// sendBufferSnapshot sends the current buffer contents to a client as a single batch message.
func (h *WebSocketHub) sendBufferSnapshot(client *Client) {
	var allCandles []schemas.Candle

	h.mu.RLock()
	tm := h.tickerManager
	h.mu.RUnlock()

	if tm != nil {
		for _, tickerSymbol := range tm.ListTickers() {
			allCandles = append(allCandles, tm.GetSnapshot(tickerSymbol)...)
		}
	} else if h.buffer != nil {
		allCandles = h.buffer.GetSnapshot()
	}

	if len(allCandles) == 0 {
		return
	}

	snapshotMsg := schemas.SnapshotMessage{
		Type:    "snapshot",
		Candles: allCandles,
	}

	data, err := json.Marshal(snapshotMsg)
	if err != nil {
		log.Printf("Error marshaling snapshot message: %v", err)
		return
	}

	select {
	case client.send <- data:
	default:
		h.unregister <- client
	}
}

// BroadcastPrediction broadcasts a prediction message to all connected clients
func (h *WebSocketHub) BroadcastPrediction(result *schemas.PredictionResult) {
	predictionMsg := schemas.PredictionMessage{
		Type:            "prediction",
		Direction:       result.Direction,
		Confidence:      result.Confidence,
		PredictedCandle: result.PredictedCandle,
		PredictedMA:     result.PredictedMA,
		Timestamp:       time.Now(),
	}

	data, err := json.Marshal(predictionMsg)
	if err != nil {
		log.Printf("Error marshaling prediction message: %v", err)
		return
	}

	h.broadcastData(data)
}

// BroadcastStatus broadcasts a status message to all connected clients
func (h *WebSocketHub) BroadcastStatus(status string) {
	statusMsg := schemas.StatusMessage{
		Type:      "status",
		Status:    status,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(statusMsg)
	if err != nil {
		log.Printf("Error marshaling status message: %v", err)
		return
	}

	h.broadcastData(data)
}

// upgrader configures the WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleConnection handles WebSocket connections
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleClientMessage(message)
	}
}

// handleClientMessage routes incoming WebSocket messages by type
func (c *Client) handleClientMessage(message []byte) {
	var baseMsg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		log.Printf("Invalid WebSocket message: %v", err)
		return
	}

	switch baseMsg.Type {
	case "timeframe_change":
		c.handleTimeframeChange(message)
	case "ticker_subscribe":
		c.handleTickerSubscribe(message)
	case "ticker_unsubscribe":
		c.handleTickerUnsubscribe(message)
	default:
		log.Printf("Unknown message type: %s", baseMsg.Type)
	}
}

func (c *Client) handleTimeframeChange(message []byte) {
	var req schemas.TimeframeChangeRequest
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("Invalid timeframe_change message: %v", err)
		return
	}

	tickerSymbol := req.Ticker
	if tickerSymbol == "" {
		tickerSymbol = "BTCUSDT"
	}

	response := schemas.TimeframeChangeResponse{
		Type:      "timeframe_change_response",
		Timeframe: req.Timeframe,
	}

	if err := c.hub.HandleTimeframeChange(tickerSymbol, req.Timeframe); err != nil {
		log.Printf("Timeframe change error: %v", err)
		response.Status = "error"
		response.Message = err.Error()
		c.sendJSON(response)
		return
	}

	response.Status = "ok"
	c.sendJSON(response)

	// Send new historical snapshot after successful timeframe change
	c.hub.sendBufferSnapshot(c)
}

func (c *Client) handleTickerSubscribe(message []byte) {
	var req schemas.TickerSubscribeRequest
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("Invalid ticker_subscribe message: %v", err)
		return
	}

	c.hub.mu.RLock()
	tm := c.hub.tickerManager
	c.hub.mu.RUnlock()

	if tm == nil {
		return
	}

	resp := schemas.TickerSubscriptionResponse{
		Type:      "ticker_sub",
		Ticker:    req.Ticker,
		Timeframe: req.Timeframe,
	}

	if err := tm.Subscribe(req.Ticker, req.Timeframe); err != nil {
		resp.Status = "error"
		resp.Message = err.Error()
	} else {
		resp.Status = "ok"
	}

	c.sendJSON(resp)
	c.broadcastTickerList(tm)
}

func (c *Client) handleTickerUnsubscribe(message []byte) {
	var req schemas.TickerUnsubscribeRequest
	if err := json.Unmarshal(message, &req); err != nil {
		log.Printf("Invalid ticker_unsubscribe message: %v", err)
		return
	}

	c.hub.mu.RLock()
	tm := c.hub.tickerManager
	c.hub.mu.RUnlock()

	if tm == nil {
		return
	}

	resp := schemas.TickerSubscriptionResponse{
		Type:   "ticker_sub",
		Ticker: req.Ticker,
	}

	if err := tm.Unsubscribe(req.Ticker); err != nil {
		resp.Status = "error"
		resp.Message = err.Error()
	} else {
		resp.Status = "ok"
	}

	c.sendJSON(resp)
	c.broadcastTickerList(tm)
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("Client send buffer full, dropping message")
	}
}

func (c *Client) broadcastTickerList(tm *ticker.TickerManager) {
	tickers := tm.ListTickers()
	listMsg := schemas.TickerListMessage{Type: "ticker_list", Tickers: tickers}
	data, err := json.Marshal(listMsg)
	if err != nil {
		return
	}

	c.hub.broadcastData(data)
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	pingTicker := time.NewTicker(PingPeriod)
	defer func() {
		pingTicker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
