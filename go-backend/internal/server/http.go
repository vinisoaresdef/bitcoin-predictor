package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"predictor/internal/metrics"
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

// WSHub interface for WebSocket hub operations
type WSHub interface {
	CloseAll() error
	HandleConnection(w http.ResponseWriter, r *http.Request)
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
	config       Config
	server       *http.Server
	startTime    time.Time
	mu           sync.RWMutex
	activeConns  atomic.Int64
	metrics      *metrics.Collector
	metricsToken string
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(cfg Config) *HTTPServer {
	s := &HTTPServer{
		config:       cfg,
		startTime:    time.Now(),
		metrics:      metrics.NewCollector(time.Second),
		metricsToken: os.Getenv("METRICS_TOKEN"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/internal/metrics", s.handleMetrics)
	mux.HandleFunc("/", s.handleStatic)

	s.server = &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
		// ConnState tracks open connections with zero per-request overhead.
		ConnState: s.trackConnState,
	}

	return s
}

// trackConnState maintains the open-connection counter via the net/http
// connection lifecycle. New connections increment; closed or hijacked
// (e.g. WebSocket upgrades) decrement. Active/Idle transitions don't change
// the count, so the value reflects currently-open HTTP connections.
func (s *HTTPServer) trackConnState(_ net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.activeConns.Add(1)
	case http.StateHijacked, http.StateClosed:
		s.activeConns.Add(-1)
	}
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

// MetricsResponse is the JSON returned by the internal telemetry endpoint.
type MetricsResponse struct {
	SystemHardware   metrics.SystemHardware   `json:"system_hardware"`
	RuntimeInternals metrics.RuntimeInternals `json:"runtime_internals"`
	ApplicationIO    ApplicationIO            `json:"application_io"`
}

// ApplicationIO holds application-level I/O telemetry owned by the HTTP server.
type ApplicationIO struct {
	ActiveHTTPConnections int64   `json:"active_http_connections"`
	UptimeSeconds         float64 `json:"uptime_seconds"`
}

// handleMetrics serves deep telemetry at /api/internal/metrics.
//
// System and runtime groups come from the cached metrics.Collector (so a burst
// of requests never floods /proc or repeatedly stops the world); the
// application_io group is filled from the server's own live counters.
//
// If the METRICS_TOKEN environment variable is set, the request must present a
// matching token via the Authorization: Bearer <token> header or the
// X-Metrics-Token header. When unset, the endpoint is open (intended to be
// reachable only from inside the cluster / behind an ingress rule).
func (s *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authorizeMetrics(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	system, runtimeStats := s.metrics.Collect()

	response := MetricsResponse{
		SystemHardware:   system,
		RuntimeInternals: runtimeStats,
		ApplicationIO: ApplicationIO{
			ActiveHTTPConnections: s.activeConns.Load(),
			UptimeSeconds:         time.Since(s.startTime).Seconds(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// authorizeMetrics returns true if the request is allowed to read metrics.
func (s *HTTPServer) authorizeMetrics(r *http.Request) bool {
	if s.metricsToken == "" {
		return true
	}
	if h := r.Header.Get("X-Metrics-Token"); h == s.metricsToken {
		return true
	}
	if h := r.Header.Get("Authorization"); strings.TrimPrefix(h, "Bearer ") == s.metricsToken {
		return true
	}
	return false
}

// handleWebSocket upgrades HTTP connections to WebSocket
func (s *HTTPServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.config.WSHub != nil {
		s.config.WSHub.HandleConnection(w, r)
	}
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
