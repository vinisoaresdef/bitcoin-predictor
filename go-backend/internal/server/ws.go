package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"predictor/internal/schemas"
)

const (
	// MaxClients is the maximum number of concurrent WebSocket connections allowed
	MaxClients = 10
	
	// WriteWait is the time allowed to write a message to the peer
	WriteWait = 10 * time.Second
	
	// PongWait is the time allowed to read the next pong message from the peer
	PongWait = 60 * time.Second
	
	// PingPeriod is the interval between ping messages sent to the peer
	PingPeriod = (PongWait * 9) / 10
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

// WebSocketHub maintains the set of active clients and broadcasts messages
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan schemas.Candle
	register   chan *Client
	unregister chan *Client
	buffer     BufferSnapshotProvider
	candleChan <-chan schemas.Candle
	mu         sync.RWMutex
	running    bool
	stopChan   chan struct{}
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
				// Reject connection if max clients reached
				close(client.send)
				client.conn.Close()
				log.Printf("Rejected client connection: max clients (%d) reached", MaxClients)
				continue
			}
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected. Total clients: %d", h.ClientCount())
			
			// Send initial status message
			h.sendStatusMessage(client, "connected")
			
			// Send buffer snapshot
			h.sendBufferSnapshot(client)

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
			// Broadcast candle to all clients
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
	
	// Close all client connections
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
			// Client's send buffer is full, close the connection
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
		// Buffer full, unregister client
		h.unregister <- client
	}
}

// sendBufferSnapshot sends the current buffer contents to a new client
func (h *WebSocketHub) sendBufferSnapshot(client *Client) {
	if h.buffer == nil {
		return
	}

	snapshot := h.buffer.GetSnapshot()
	for _, candle := range snapshot {
		klineMsg := schemas.KlineMessage{
			Type:   "kline",
			Candle: candle,
		}

		data, err := json.Marshal(klineMsg)
		if err != nil {
			log.Printf("Error marshaling kline message: %v", err)
			continue
		}

		select {
		case client.send <- data:
		default:
			// Buffer full, unregister client
			h.unregister <- client
			return
		}
	}
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

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
