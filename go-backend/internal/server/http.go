package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"
)

// HealthResponse represents the health endpoint response
type HealthResponse struct {
	Status        string `json:"status"`
	Binance       string `json:"binance"`
	BufferSize    int    `json:"buffer_size"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// BinanceClient interface for health status
type BinanceClient interface {
	GetStatus() string
	Close() error
}

// Buffer interface for getting buffer size
type Buffer interface {
	Len() int
}

// WSHub interface for closing client connections
type WSHub interface {
	CloseAll() error
}

// Config holds server configuration
type Config struct {
	Addr          string
	FrontendDir   string
	BinanceClient BinanceClient
	Buffer        Buffer
	WSHub         WSHub
	SignalChan    chan os.Signal
}

// HTTPServer wraps http.Server with graceful shutdown capabilities
type HTTPServer struct {
	config    Config
	server    *http.Server
	startTime time.Time
	mu        sync.RWMutex
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(cfg Config) *HTTPServer {
	s := &HTTPServer{
		config:    cfg,
		startTime: time.Now(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleStatic)

	s.server = &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return s
}

// handler returns the http.Handler for testing
func (s *HTTPServer) handler() http.Handler {
	return s.server.Handler
}

// Start starts the HTTP server and blocks until shutdown
func (s *HTTPServer) Start() error {
	if s.config.SignalChan == nil {
		s.config.SignalChan = make(chan os.Signal, 1)
		signal.Notify(s.config.SignalChan, syscall.SIGINT, syscall.SIGTERM)
	}

	shutdownCh := make(chan error, 1)
	go func() {
		sig := <-s.config.SignalChan
		fmt.Printf("Received signal %v, initiating graceful shutdown...\n", sig)
		shutdownCh <- s.Shutdown(context.Background())
	}()

	serverErrCh := make(chan error, 1)
	go func() {
		fmt.Printf("HTTP server starting on %s\n", s.config.Addr)
		serverErrCh <- s.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case err := <-shutdownCh:
		return err
	}
}

// Shutdown gracefully shuts down the server
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.config.BinanceClient != nil {
		if err := s.config.BinanceClient.Close(); err != nil {
			fmt.Printf("Error closing Binance client: %v\n", err)
		}
	}

	if s.config.WSHub != nil {
		if err := s.config.WSHub.CloseAll(); err != nil {
			fmt.Printf("Error closing WS connections: %v\n", err)
		}
	}

	return s.server.Shutdown(shutdownCtx)
}

// handleHealth serves the health endpoint
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	binanceStatus := "disconnected"
	if s.config.BinanceClient != nil {
		binanceStatus = s.config.BinanceClient.GetStatus()
	}

	bufferSize := 0
	if s.config.Buffer != nil {
		bufferSize = s.config.Buffer.Len()
	}

	uptime := time.Since(s.startTime).Seconds()

	response := HealthResponse{
		Status:        "ok",
		Binance:       binanceStatus,
		BufferSize:    bufferSize,
		UptimeSeconds: int64(uptime),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleStatic serves static files from the frontend directory
func (s *HTTPServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	urlPath := r.URL.Path

	cleanPath := path.Clean(urlPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	cleanPath = strings.TrimPrefix(cleanPath, "/")

	var filePath string
	if cleanPath == "" {
		filePath = path.Join(s.config.FrontendDir, "index.html")
	} else {
		filePath = path.Join(s.config.FrontendDir, cleanPath)
	}

	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		http.NotFound(w, r)
		return
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filePath)
}
