package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"predictor/internal/schemas"
)

// mockBuffer implements a simple buffer for testing WebSocket functionality
type mockBuffer struct {
	mu      sync.RWMutex
	candles []schemas.Candle
}

func newMockBuffer() *mockBuffer {
	return &mockBuffer{
		candles: make([]schemas.Candle, 0),
	}
}

func (b *mockBuffer) GetSnapshot() []schemas.Candle {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snapshot := make([]schemas.Candle, len(b.candles))
	copy(snapshot, b.candles)
	return snapshot
}

func (b *mockBuffer) AddCandle(candle schemas.Candle) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.candles = append(b.candles, candle)
}

// mockUpgrader creates a test WebSocket upgrader
func mockUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
}

// TestNewClientReceivesBuffer verifies that a new WebSocket client receives the current buffer snapshot
func TestNewClientReceivesBuffer(t *testing.T) {
	// Create mock buffer with some initial candles
	buffer := newMockBuffer()
	buffer.AddCandle(schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	})
	buffer.AddCandle(schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50500.0,
		High:      51500.0,
		Low:       50000.0,
		Close:     51000.0,
		Volume:    150.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	})

	// Create WebSocket hub
	candleChan := make(chan schemas.Candle, 10)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Give the server time to send messages
	time.Sleep(100 * time.Millisecond)

	// Read messages from WebSocket
	var statusMsg schemas.StatusMessage
	err = ws.ReadJSON(&statusMsg)
	if err != nil {
		t.Fatalf("Failed to read status message: %v", err)
	}

	// Verify status message
	if statusMsg.Type != "status" {
		t.Errorf("Expected type 'status', got '%s'", statusMsg.Type)
	}
	if statusMsg.Status != "connected" {
		t.Errorf("Expected status 'connected', got '%s'", statusMsg.Status)
	}
	if statusMsg.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Read buffer snapshot candles
	for i := 0; i < 2; i++ {
		var klineMsg schemas.KlineMessage
		err = ws.ReadJSON(&klineMsg)
		if err != nil {
			t.Fatalf("Failed to read kline message %d: %v", i, err)
		}
		if klineMsg.Type != "kline" {
			t.Errorf("Expected type 'kline', got '%s'", klineMsg.Type)
		}
		if klineMsg.Candle.Symbol != "BTCUSDT" {
			t.Errorf("Expected symbol 'BTCUSDT', got '%s'", klineMsg.Candle.Symbol)
		}
	}
}

// TestBroadcastToAllClients verifies that candles are broadcast to all connected clients
func TestBroadcastToAllClients(t *testing.T) {
	buffer := newMockBuffer()
	candleChan := make(chan schemas.Candle, 10)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	// Connect 3 clients
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	clients := make([]*websocket.Conn, 3)
	for i := 0; i < 3; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		clients[i] = ws
		defer ws.Close()
	}

	// Wait for initial status messages
	time.Sleep(100 * time.Millisecond)
	for i, ws := range clients {
		var statusMsg schemas.StatusMessage
		ws.SetReadDeadline(time.Now().Add(time.Second))
		err := ws.ReadJSON(&statusMsg)
		if err != nil {
			t.Fatalf("Client %d failed to read status: %v", i, err)
		}
		ws.SetReadDeadline(time.Time{})
	}

	// Send a candle through the channel
	testCandle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      52000.0,
		High:      53000.0,
		Low:       51500.0,
		Close:     52500.0,
		Volume:    200.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}

	candleChan <- testCandle

	// Wait for broadcast
	time.Sleep(100 * time.Millisecond)

	// Verify all clients received the candle
	for i, ws := range clients {
		ws.SetReadDeadline(time.Now().Add(time.Second))
		var klineMsg schemas.KlineMessage
		err := ws.ReadJSON(&klineMsg)
		if err != nil {
			t.Fatalf("Client %d failed to read broadcast: %v", i, err)
		}
		ws.SetReadDeadline(time.Time{})

		if klineMsg.Type != "kline" {
			t.Errorf("Client %d: Expected type 'kline', got '%s'", i, klineMsg.Type)
		}
		if klineMsg.Candle.Close != 52500.0 {
			t.Errorf("Client %d: Expected close price 52500.0, got %f", i, klineMsg.Candle.Close)
		}
	}
}

// TestClientDisconnect verifies that disconnected clients are properly removed
func TestClientDisconnect(t *testing.T) {
	buffer := newMockBuffer()
	candleChan := make(chan schemas.Candle, 10)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	// Connect a client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Wait for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Verify client is connected (check client count)
	if hub.ClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", hub.ClientCount())
	}

	// Disconnect client
	ws.Close()

	// Wait for disconnect to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify client is removed
	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", hub.ClientCount())
	}

	// Send a candle - should not panic or error
	testCandle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      54000.0,
		High:      55000.0,
		Low:       53500.0,
		Close:     54500.0,
		Volume:    300.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}

	// This should not block or panic even with no clients
	select {
	case candleChan <- testCandle:
		// Success
		time.Sleep(100 * time.Millisecond)
	case <-time.After(time.Second):
		t.Error("Sending candle blocked unexpectedly")
	}
}

// TestConcurrentClients verifies that multiple clients can connect and disconnect concurrently
func TestConcurrentClients(t *testing.T) {
	buffer := newMockBuffer()
	candleChan := make(chan schemas.Candle, 100)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	var wg sync.WaitGroup
	numClients := 5
	clients := make([]*websocket.Conn, numClients)

	// Concurrent connections
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Errorf("Client %d failed to connect: %v", idx, err)
				return
			}
			clients[idx] = ws
		}(i)
	}
	wg.Wait()

	// Wait for all connections to establish
	time.Sleep(200 * time.Millisecond)

	// Verify all clients connected
	if hub.ClientCount() != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, hub.ClientCount())
	}

	// Concurrent disconnections
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if clients[idx] != nil {
				clients[idx].Close()
			}
		}(i)
	}
	wg.Wait()

	// Wait for all disconnects to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify all clients disconnected
	if hub.ClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}

// TestMaxClients verifies that the hub enforces a maximum client limit
func TestMaxClients(t *testing.T) {
	buffer := newMockBuffer()
	candleChan := make(chan schemas.Candle, 10)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Try to connect 12 clients (max is 10)
	clients := make([]*websocket.Conn, 0)
	for i := 0; i < 12; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			// Expected for clients beyond the limit
			continue
		}
		clients = append(clients, ws)
		defer ws.Close()
	}

	time.Sleep(100 * time.Millisecond)

	// Verify we have at most 10 clients
	if hub.ClientCount() > 10 {
		t.Errorf("Expected at most 10 clients, got %d", hub.ClientCount())
	}
}

// TestMessageFormat verifies that messages are properly formatted as JSON
func TestMessageFormat(t *testing.T) {
	buffer := newMockBuffer()
	buffer.AddCandle(schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	})

	candleChan := make(chan schemas.Candle, 10)
	hub := NewWebSocketHub(buffer, candleChan)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleConnection(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// Read raw message and verify it's valid JSON
	ws.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	ws.SetReadDeadline(time.Time{})

	var statusMsg schemas.StatusMessage
	if err := json.Unmarshal(message, &statusMsg); err != nil {
		t.Fatalf("Status message is not valid JSON: %v", err)
	}

	// Verify required fields
	if statusMsg.Type == "" {
		t.Error("Status message missing 'type' field")
	}
	if statusMsg.Status == "" {
		t.Error("Status message missing 'status' field")
	}
	if statusMsg.Timestamp.IsZero() {
		t.Error("Status message missing 'timestamp' field")
	}
}
