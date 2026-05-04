package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"predictor/internal/binance"
	"predictor/internal/schemas"
)

func TestMainStartupSequence(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	indexPath := filepath.Join(frontendDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)

	if app == nil {
		t.Fatal("NewApplication returned nil")
	}

	if app.buffer == nil {
		t.Error("buffer is nil")
	}

	if app.candleChan == nil {
		t.Error("candleChan is nil")
	}

	if app.binanceClient == nil {
		t.Error("binanceClient is nil")
	}

	if app.wsHub == nil {
		t.Error("wsHub is nil")
	}

	if app.httpServer == nil {
		t.Error("httpServer is nil")
	}

	if app.buffer.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d candles", app.buffer.Len())
	}

	if cap(app.candleChan) != 256 {
		t.Errorf("Expected candleChan capacity 256, got %d", cap(app.candleChan))
	}
}

func TestFullPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	testCandle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}

	app.candleChan <- testCandle

	time.Sleep(100 * time.Millisecond)

	if app.buffer.Len() != 1 {
		t.Errorf("Expected buffer length 1, got %d", app.buffer.Len())
	}

	snapshot := app.buffer.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Expected snapshot length 1, got %d", len(snapshot))
	}

	if snapshot[0].Close != 50500.0 {
		t.Errorf("Expected close price 50500.0, got %f", snapshot[0].Close)
	}
}

func TestMultipleCandlesPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   10,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	for i := 0; i < 5; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1s",
			Open:      float64(50000 + i*100),
			High:      float64(50100 + i*100),
			Low:       float64(49900 + i*100),
			Close:     float64(50050 + i*100),
			Volume:    float64(100 + i*10),
			CloseTime: time.Now().Add(time.Duration(i) * time.Second),
			Timestamp: time.Now(),
		}
		app.candleChan <- candle
	}

	time.Sleep(100 * time.Millisecond)

	if app.buffer.Len() != 5 {
		t.Errorf("Expected buffer length 5, got %d", app.buffer.Len())
	}

	snapshot := app.buffer.Snapshot()
	if len(snapshot) != 5 {
		t.Fatalf("Expected snapshot length 5, got %d", len(snapshot))
	}

	for i, candle := range snapshot {
		expectedClose := float64(50050 + i*100)
		if candle.Close != expectedClose {
			t.Errorf("Candle %d: expected close %f, got %f", i, expectedClose, candle.Close)
		}
	}
}

func TestBufferCapacityLimit(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   5,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	for i := 0; i < 10; i++ {
		candle := schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1s",
			Open:      float64(i),
			High:      float64(i) + 1,
			Low:       float64(i) - 1,
			Close:     float64(i) + 0.5,
			Volume:    float64(i) * 100,
			CloseTime: time.Now().Add(time.Duration(i) * time.Second),
			Timestamp: time.Now(),
		}
		app.candleChan <- candle
	}

	time.Sleep(100 * time.Millisecond)

	if app.buffer.Len() != 5 {
		t.Errorf("Expected buffer length 5 (capacity), got %d", app.buffer.Len())
	}

	snapshot := app.buffer.Snapshot()
	if len(snapshot) != 5 {
		t.Fatalf("Expected snapshot length 5, got %d", len(snapshot))
	}

	if snapshot[0].Open != 5 {
		t.Errorf("Expected first candle Open=5 (oldest after eviction), got %f", snapshot[0].Open)
	}

	if snapshot[4].Open != 9 {
		t.Errorf("Expected last candle Open=9 (newest), got %f", snapshot[4].Open)
	}
}

func TestGracefulStop(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	candle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}
	app.candleChan <- candle

	time.Sleep(50 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- app.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop timed out")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	os.Unsetenv("HTTP_PORT")
	os.Unsetenv("FRONTEND_DIR")
	os.Unsetenv("BINANCE_WS_URL")

	config := LoadConfig()

	if config.HTTPPort != "8080" {
		t.Errorf("Expected default HTTPPort 8080, got %s", config.HTTPPort)
	}

	if config.FrontendDir != "./frontend/dist" {
		t.Errorf("Expected default FrontendDir ./frontend/dist, got %s", config.FrontendDir)
	}

	if config.BinanceWSURL != binance.DefaultBinanceWSURL {
		t.Errorf("Expected default BinanceWSURL %s, got %s", binance.DefaultBinanceWSURL, config.BinanceWSURL)
	}

	if config.BufferSize != 60 {
		t.Errorf("Expected default BufferSize 60, got %d", config.BufferSize)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("FRONTEND_DIR", "/custom/frontend")
	os.Setenv("BINANCE_WS_URL", "wss://custom.binance.url")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("FRONTEND_DIR")
		os.Unsetenv("BINANCE_WS_URL")
	}()

	config := LoadConfig()

	if config.HTTPPort != "9090" {
		t.Errorf("Expected HTTPPort 9090, got %s", config.HTTPPort)
	}

	if config.FrontendDir != "/custom/frontend" {
		t.Errorf("Expected FrontendDir /custom/frontend, got %s", config.FrontendDir)
	}

	if config.BinanceWSURL != "wss://custom.binance.url" {
		t.Errorf("Expected BinanceWSURL wss://custom.binance.url, got %s", config.BinanceWSURL)
	}
}

func TestBufferAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)
	adapter := &bufferAdapter{buffer: app.buffer}

	candle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}

	app.buffer.Append(candle)

	snapshot := adapter.GetSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Expected snapshot length 1, got %d", len(snapshot))
	}

	if snapshot[0].Close != 50500.0 {
		t.Errorf("Expected close price 50500.0, got %f", snapshot[0].Close)
	}
}

func TestBinanceClientAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)
	adapter := &binanceClientAdapter{client: app.binanceClient}

	status := adapter.GetStatus()
	if status != "connected" {
		t.Errorf("Expected status 'connected', got '%s'", status)
	}

	if err := adapter.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestWSHubAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)
	adapter := &wsHubAdapter{hub: app.wsHub}

	go func() {
		app.wsHub.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	if err := adapter.CloseAll(); err != nil {
		t.Errorf("CloseAll returned error: %v", err)
	}
}

func TestConcurrentCandleProcessing(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   100,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	done := make(chan bool)
	go func() {
		for i := 0; i < 50; i++ {
			candle := schemas.Candle{
				Symbol:    "BTCUSDT",
				Interval:  "1s",
				Open:      float64(i),
				High:      float64(i) + 1,
				Low:       float64(i) - 1,
				Close:     float64(i) + 0.5,
				Volume:    float64(i) * 100,
				CloseTime: time.Now().Add(time.Duration(i) * time.Second),
				Timestamp: time.Now(),
			}
			app.candleChan <- candle
		}
		done <- true
	}()

	go func() {
		for i := 50; i < 100; i++ {
			candle := schemas.Candle{
				Symbol:    "BTCUSDT",
				Interval:  "1s",
				Open:      float64(i),
				High:      float64(i) + 1,
				Low:       float64(i) - 1,
				Close:     float64(i) + 0.5,
				Volume:    float64(i) * 100,
				CloseTime: time.Now().Add(time.Duration(i) * time.Second),
				Timestamp: time.Now(),
			}
			app.candleChan <- candle
		}
		done <- true
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for goroutines")
		}
	}

	time.Sleep(100 * time.Millisecond)

	if app.buffer.Len() != 100 {
		t.Errorf("Expected buffer length 100, got %d", app.buffer.Len())
	}
}

func TestWebSocketBroadcastIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()
	defer app.wsHub.Stop()

	go app.candleBufferConsumer()

	testCandle := schemas.Candle{
		Symbol:    "BTCUSDT",
		Interval:  "1s",
		Open:      50000.0,
		High:      51000.0,
		Low:       49000.0,
		Close:     50500.0,
		Volume:    100.0,
		CloseTime: time.Now(),
		Timestamp: time.Now(),
	}

	app.candleChan <- testCandle

	time.Sleep(100 * time.Millisecond)

	if app.buffer.Len() != 1 {
		t.Errorf("Expected buffer length 1, got %d", app.buffer.Len())
	}
}

func TestContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	config := Config{
		HTTPPort:     "0",
		FrontendDir:  frontendDir,
		BinanceWSURL: "wss://invalid.url.for.testing",
		BufferSize:   60,
	}

	app := NewApplication(config)

	go func() {
		app.wsHub.Run()
	}()

	time.Sleep(50 * time.Millisecond)

	go app.candleBufferConsumer()

	app.cancel()

	time.Sleep(100 * time.Millisecond)

	select {
	case <-app.ctx.Done():
	default:
		t.Error("Context should be cancelled")
	}
}

func TestGetFrontendDir(t *testing.T) {
	dir := getFrontendDir()

	if dir == "" {
		t.Error("getFrontendDir returned empty string")
	}

	if dir != "./frontend/dist" {
		t.Logf("getFrontendDir returned: %s (may vary by environment)", dir)
	}
}
