package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type MockBinanceClient struct {
	status string
}

func (m *MockBinanceClient) GetStatus() string {
	return m.status
}

func (m *MockBinanceClient) Close() error {
	return nil
}

type MockBuffer struct {
	size int
}

func (m *MockBuffer) Len() int {
	return m.size
}

func TestHealthEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	indexPath := filepath.Join(frontendDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	binanceClient := &MockBinanceClient{status: "connected"}
	buffer := &MockBuffer{size: 42}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: binanceClient,
		Buffer:        buffer,
	})

	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	var healthResp HealthResponse
	if err := json.Unmarshal(body, &healthResp); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if healthResp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", healthResp.Status)
	}
	if healthResp.Binance != "connected" {
		t.Errorf("Expected binance 'connected', got '%s'", healthResp.Binance)
	}
	if healthResp.BufferSize != 42 {
		t.Errorf("Expected buffer_size 42, got %d", healthResp.BufferSize)
	}
	if healthResp.UptimeSeconds < 0 {
		t.Errorf("Expected uptime_seconds >= 0, got %d", healthResp.UptimeSeconds)
	}
}

func TestServeIndexHTML(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	htmlContent := "<!DOCTYPE html><html><head></head><body>Test</body></html>"
	indexPath := filepath.Join(frontendDir, "index.html")
	if err := os.WriteFile(indexPath, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: &MockBinanceClient{},
		Buffer:        &MockBuffer{},
	})

	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	expectedTypes := []string{"text/html", "text/html; charset=utf-8"}
	hasCorrectType := false
	for _, expected := range expectedTypes {
		if contentType == expected {
			hasCorrectType = true
			break
		}
	}
	if !hasCorrectType {
		t.Errorf("Expected Content-Type to contain text/html, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}
	if string(body) != htmlContent {
		t.Errorf("Expected body to match index.html content, got %s", string(body))
	}
}

func TestGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	indexPath := filepath.Join(frontendDir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	binanceClient := &MockBinanceClientWithClose{}
	wsHub := &MockWSHubWithClose{}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: binanceClient,
		Buffer:        &MockBuffer{},
		WSHub:         wsHub,
	})

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	shutdownCh := make(chan error, 1)
	go func() {
		shutdownCh <- server.Shutdown(context.Background())
	}()

	select {
	case err := <-shutdownCh:
		if err != nil {
			t.Errorf("Shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown timed out")
	}

	if !binanceClient.closed.Load() {
		t.Error("Binance client Close() was not called")
	}
	if !wsHub.closed.Load() {
		t.Error("WSHub CloseAll() was not called")
	}
}

type MockBinanceClientWithClose struct {
	closed atomic.Bool
}

func (m *MockBinanceClientWithClose) GetStatus() string {
	return "connected"
}

func (m *MockBinanceClientWithClose) Close() error {
	m.closed.Store(true)
	return nil
}

type MockWSHubWithClose struct {
	closed atomic.Bool
}

func (m *MockWSHubWithClose) CloseAll() error {
	m.closed.Store(true)
	return nil
}

func TestServeStaticFiles(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	cssDir := filepath.Join(frontendDir, "css")
	jsDir := filepath.Join(frontendDir, "js")

	for _, dir := range []string{frontendDir, cssDir, jsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cssDir, "style.css"), []byte("body { color: red; }"), 0644); err != nil {
		t.Fatalf("Failed to create style.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jsDir, "app.js"), []byte("console.log('test');"), 0644); err != nil {
		t.Fatalf("Failed to create app.js: %v", err)
	}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: &MockBinanceClient{},
		Buffer:        &MockBuffer{},
	})

	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	tests := []struct {
		path         string
		expectedBody string
		contentType  string
	}{
		{"/css/style.css", "body { color: red; }", "text/css"},
		{"/js/app.js", "console.log('test');", "application/javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(ts.URL + tt.path)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read body: %v", err)
			}
			if string(body) != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, string(body))
			}
		})
	}
}

func TestSignalHandling(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	sigCh := make(chan os.Signal, 1)

	binanceClient := &MockBinanceClientWithClose{}
	wsHub := &MockWSHubWithClose{}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: binanceClient,
		Buffer:        &MockBuffer{},
		WSHub:         wsHub,
		SignalChan:    sigCh,
	})

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error (expected on shutdown): %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	sigCh <- syscall.SIGINT

	done := make(chan bool)
	go func() {
		for i := 0; i < 50; i++ {
			if binanceClient.closed.Load() && wsHub.closed.Load() {
				done <- true
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		done <- false
	}()

	select {
	case success := <-done:
		if !success {
			t.Fatal("Shutdown did not complete within timeout")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out waiting for shutdown")
	}

	if !binanceClient.closed.Load() {
		t.Error("Binance client was not closed on signal")
	}
	if !wsHub.closed.Load() {
		t.Error("WSHub was not closed on signal")
	}
}

func TestNoDirectoryListing(t *testing.T) {
	tmpDir := t.TempDir()
	frontendDir := filepath.Join(tmpDir, "frontend")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatalf("Failed to create frontend dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	server := NewHTTPServer(Config{
		Addr:          ":0",
		FrontendDir:   frontendDir,
		BinanceClient: &MockBinanceClient{},
		Buffer:        &MockBuffer{},
	})

	ts := httptest.NewServer(server.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/css/")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404 for directory request, got %d", resp.StatusCode)
	}
}
