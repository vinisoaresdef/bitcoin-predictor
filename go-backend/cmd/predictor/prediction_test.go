package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"predictor/internal/schemas"
)

type MockMLClient struct {
	mu          sync.RWMutex
	predictFunc func([]schemas.Candle) (*schemas.PredictionResult, error)
	callCount   int
	lastCandles []schemas.Candle
}

func NewMockMLClient() *MockMLClient {
	return &MockMLClient{
		predictFunc: func(candles []schemas.Candle) (*schemas.PredictionResult, error) {
			return &schemas.PredictionResult{
				Direction:  "UP",
				Confidence: 0.72,
			}, nil
		},
	}
}

func (m *MockMLClient) Predict(candles []schemas.Candle) (*schemas.PredictionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	m.lastCandles = make([]schemas.Candle, len(candles))
	copy(m.lastCandles, candles)
	if m.predictFunc != nil {
		return m.predictFunc(candles)
	}
	return nil, errors.New("no predict function set")
}

func (m *MockMLClient) Health() (bool, error) {
	return true, nil
}

func (m *MockMLClient) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCount
}

func (m *MockMLClient) GetLastCandles() []schemas.Candle {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]schemas.Candle, len(m.lastCandles))
	copy(result, m.lastCandles)
	return result
}

func (m *MockMLClient) SetPredictFunc(fn func([]schemas.Candle) (*schemas.PredictionResult, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.predictFunc = fn
}

func createTestApp(t *testing.T, config Config) (*Application, string) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}
	config.FrontendDir = frontendDir
	return NewApplication(config), frontendDir
}

func fillBuffer(app *Application, count int) {
	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		return
	}
	baseTime := time.Now()
	for i := 0; i < count; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      float64(42000 + i),
			High:      float64(42050 + i),
			Low:       float64(41950 + i),
			Close:     float64(42025 + i),
			Volume:    float64(1 + i),
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
		}
		buf.Append(candle)
	}
}

func TestPredictionBroadcast(t *testing.T) {
	mockML := NewMockMLClient()

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 100 * time.Millisecond,
		EnablePredictions:  true,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 60)

	time.Sleep(200 * time.Millisecond)

	srv := httptest.NewServer(http.HandlerFunc(app.wsHub.HandleConnection))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Read initial messages: status + snapshot (batch)
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 2; i++ {
		_, _, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read initial message %d: %v", i, err)
		}
	}

	// Read until we find a prediction message
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	foundPrediction := false
	for i := 0; i < 30; i++ {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		if msgType, ok := msg["type"].(string); ok && msgType == "prediction" {
			foundPrediction = true
			break
		}
	}
	if !foundPrediction {
		t.Error("Expected prediction message but did not receive one")
	}

	if mockML.GetCallCount() < 1 {
		t.Errorf("Expected ML client to be called at least once, got %d calls", mockML.GetCallCount())
	}

	lastCandles := mockML.GetLastCandles()
	if len(lastCandles) != 60 {
		t.Errorf("Expected 60 candles passed to ML client, got %d", len(lastCandles))
	}
}

func TestPredictionBroadcastWithMLError(t *testing.T) {
	mockML := NewMockMLClient()
	mockML.SetPredictFunc(func(candles []schemas.Candle) (*schemas.PredictionResult, error) {
		return nil, errors.New("ML service unavailable")
	})

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 100 * time.Millisecond,
		EnablePredictions:  true,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 60)

	time.Sleep(200 * time.Millisecond)

	srv := httptest.NewServer(http.HandlerFunc(app.wsHub.HandleConnection))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer ws.Close()

	// Read initial messages: status + snapshot (batch)
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 2; i++ {
		_, _, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to read initial message %d: %v", i, err)
		}
	}

	// Read until we find a status message with "unavailable"
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	foundStatus := false
	for i := 0; i < 30; i++ {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		if msgType, ok := msg["type"].(string); ok && msgType == "status" {
			if status, ok := msg["status"].(string); ok && strings.Contains(status, "unavailable") {
				foundStatus = true
				break
			}
		}
	}
	if !foundStatus {
		t.Error("Expected status message with 'unavailable' but did not receive one")
	}
}

func TestPredictionBufferNotFull(t *testing.T) {
	mockML := NewMockMLClient()

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 50 * time.Millisecond,
		EnablePredictions:  true,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 30)

	time.Sleep(200 * time.Millisecond)

	if mockML.GetCallCount() > 0 {
		t.Errorf("ML client should not be called when buffer has < 60 candles, got %d calls", mockML.GetCallCount())
	}
}

func TestPredictionDisabled(t *testing.T) {
	mockML := NewMockMLClient()

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 50 * time.Millisecond,
		EnablePredictions:  false,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 60)

	time.Sleep(200 * time.Millisecond)

	if mockML.GetCallCount() > 0 {
		t.Errorf("ML client should not be called when predictions disabled, got %d calls", mockML.GetCallCount())
	}
}

func TestPredictionGracefulDegradation(t *testing.T) {
	mockML := NewMockMLClient()
	callCount := 0
	mockML.SetPredictFunc(func(candles []schemas.Candle) (*schemas.PredictionResult, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("temporary error")
		}
		return &schemas.PredictionResult{
			Direction:  "DOWN",
			Confidence: 0.85,
		}, nil
	})

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 100 * time.Millisecond,
		EnablePredictions:  true,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 60)

	time.Sleep(300 * time.Millisecond)

	if mockML.GetCallCount() < 2 {
		t.Errorf("Expected at least 2 ML calls (1 fail + 1 retry), got %d", mockML.GetCallCount())
	}
}

func TestPredictionDoesNotBlockCandleStreaming(t *testing.T) {
	mockML := NewMockMLClient()
	mockML.SetPredictFunc(func(candles []schemas.Candle) (*schemas.PredictionResult, error) {
		time.Sleep(500 * time.Millisecond)
		return &schemas.PredictionResult{
			Direction:  "UP",
			Confidence: 0.72,
		}, nil
	})

	config := Config{
		HTTPPort:           "0",
		BinanceWSURL:       "wss://invalid.url.for.testing",
		BufferSize:         60,
		PredictionInterval: 100 * time.Millisecond,
		EnablePredictions:  true,
	}

	app := NewApplicationWithML(config, mockML)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()
	go app.predictionWorker()

	fillBuffer(app, 60)

	time.Sleep(50 * time.Millisecond)

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

	for i := 60; i < 65; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      float64(42000 + i),
			High:      float64(42050 + i),
			Low:       float64(41950 + i),
			Close:     float64(42025 + i),
			Volume:    float64(1 + i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
		}
		buf.Append(candle)
	}

	time.Sleep(100 * time.Millisecond)

	if buf.Len() != 60 {
		t.Errorf("Expected buffer to have 60 candles (max capacity), got %d", buf.Len())
	}
}
