package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

	if app.tickerManager == nil {
		t.Error("tickerManager is nil")
	}

	if app.broadcastChan == nil {
		t.Error("broadcastChan is nil")
	}

	if app.wsHub == nil {
		t.Error("wsHub is nil")
	}

	if app.httpServer == nil {
		t.Error("httpServer is nil")
	}

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Error("BTCUSDT buffer not found")
	} else if buf.Len() != 0 {
		t.Errorf("Expected empty buffer, got %d candles", buf.Len())
	}

	if cap(app.broadcastChan) != 256 {
		t.Errorf("Expected broadcastChan capacity 256, got %d", cap(app.broadcastChan))
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

	app.broadcastChan <- testCandle

	time.Sleep(100 * time.Millisecond)

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

	if buf.Len() != 0 {
		t.Errorf("Expected buffer length 0 (bypassed ticker), got %d", buf.Len())
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

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

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
		buf.Append(candle)
	}

	time.Sleep(100 * time.Millisecond)

	if buf.Len() != 5 {
		t.Errorf("Expected buffer length 5, got %d", buf.Len())
	}

	snapshot := buf.Snapshot()
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

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

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
		buf.Append(candle)
	}

	time.Sleep(100 * time.Millisecond)

	if buf.Len() != 5 {
		t.Errorf("Expected buffer length 5 (capacity), got %d", buf.Len())
	}

	snapshot := buf.Snapshot()
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

	if config.FrontendDir != "./frontend" {
		t.Errorf("Expected default FrontendDir ./frontend, got %s", config.FrontendDir)
	}

	expectedWSURL := "wss://stream.binance.com:9443/ws/btcusdt@kline_1s"
	if config.BinanceWSURL != expectedWSURL {
		t.Errorf("Expected default BinanceWSURL %s, got %s", expectedWSURL, config.BinanceWSURL)
	}

	if config.BufferSize != 1500 {
		t.Errorf("Expected default BufferSize 1500, got %d", config.BufferSize)
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

func TestTickerBufferAdapter(t *testing.T) {
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
	adapter := &tickerBufferAdapter{tm: app.tickerManager}

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

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

	buf.Append(candle)

	snapshot := adapter.GetSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("Expected snapshot length 1, got %d", len(snapshot))
	}

	if snapshot[0].Close != 50500.0 {
		t.Errorf("Expected close price 50500.0, got %f", snapshot[0].Close)
	}
}

func TestTickerStatusAdapter(t *testing.T) {
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
	adapter := &tickerStatusAdapter{tm: app.tickerManager}

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

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

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
			buf.Append(candle)
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
			buf.Append(candle)
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

	if buf.Len() != 100 {
		t.Errorf("Expected buffer length 100, got %d", buf.Len())
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

	app.broadcastChan <- testCandle

	time.Sleep(100 * time.Millisecond)

	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT buffer not found")
	}

	if buf.Len() != 0 {
		t.Errorf("Expected buffer length 0 (bypassed ticker), got %d", buf.Len())
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

