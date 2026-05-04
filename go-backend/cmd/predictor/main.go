package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"predictor/internal/binance"
	"predictor/internal/buffer"
	"predictor/internal/schemas"
	"predictor/internal/server"
)

type Application struct {
	buffer        *buffer.RingBuffer
	candleChan    chan schemas.Candle
	broadcastChan chan schemas.Candle
	binanceClient *binance.Client
	wsHub         *server.WebSocketHub
	httpServer    *server.HTTPServer
	httpAddr      string
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

type Config struct {
	HTTPPort     string
	FrontendDir  string
	BinanceWSURL string
	BufferSize   int
}

func LoadConfig() Config {
	return Config{
		HTTPPort:     getEnv("HTTP_PORT", "8080"),
		FrontendDir:  getEnv("FRONTEND_DIR", "./frontend/dist"),
		BinanceWSURL: getEnv("BINANCE_WS_URL", binance.DefaultBinanceWSURL),
		BufferSize:   60,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type bufferAdapter struct {
	buffer *buffer.RingBuffer
}

func (b *bufferAdapter) GetSnapshot() []schemas.Candle {
	return b.buffer.Snapshot()
}

func NewApplication(config Config) *Application {
	ctx, cancel := context.WithCancel(context.Background())

	candleChan := make(chan schemas.Candle, 256)
	broadcastChan := make(chan schemas.Candle, 256)
	buf := buffer.New(config.BufferSize)
	bufAdapter := &bufferAdapter{buffer: buf}

	binanceConfig := binance.DefaultConfig()
	binanceConfig.WSURL = config.BinanceWSURL

	client := binance.NewClient(binanceConfig, candleChan)
	wsHub := server.NewWebSocketHub(bufAdapter, broadcastChan)

	httpAddr := ":" + config.HTTPPort
	httpConfig := server.Config{
		Addr:          httpAddr,
		FrontendDir:   config.FrontendDir,
		BinanceClient: &binanceClientAdapter{client: client},
		Buffer:        buf,
		WSHub:         &wsHubAdapter{hub: wsHub},
	}

	httpServer := server.NewHTTPServer(httpConfig)

	return &Application{
		buffer:        buf,
		candleChan:    candleChan,
		broadcastChan: broadcastChan,
		binanceClient: client,
		wsHub:         wsHub,
		httpServer:    httpServer,
		httpAddr:      httpAddr,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (app *Application) Start() error {
	log.Println("Starting Predictor application...")

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		log.Println("WebSocket hub started")
		app.wsHub.Run()
		log.Println("WebSocket hub stopped")
	}()

	time.Sleep(50 * time.Millisecond)

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		log.Println("Candle buffer consumer started")
		app.candleBufferConsumer()
		log.Println("Candle buffer consumer stopped")
	}()

	if err := app.binanceClient.Start(app.ctx); err != nil {
		return fmt.Errorf("failed to start Binance client: %w", err)
	}
	log.Println("Binance WebSocket client started")

	log.Printf("HTTP server starting on %s", app.httpAddr)
	if err := app.httpServer.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

func (app *Application) candleBufferConsumer() {
	for {
		select {
		case candle, ok := <-app.candleChan:
			if !ok {
				return
			}
			app.buffer.Append(candle)
			select {
			case app.broadcastChan <- candle:
			case <-app.ctx.Done():
				return
			}
		case <-app.ctx.Done():
			return
		}
	}
}

func (app *Application) Stop() error {
	log.Println("Initiating graceful shutdown...")

	app.cancel()

	if err := app.binanceClient.Stop(); err != nil {
		log.Printf("Error stopping Binance client: %v", err)
	}

	app.wsHub.Stop()
	close(app.candleChan)
	close(app.broadcastChan)

	done := make(chan struct{})
	go func() {
		app.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All goroutines stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for goroutines, forcing shutdown")
	}

	log.Println("Shutdown complete")
	return nil
}

func (app *Application) Run() error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Start()
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
		return app.Stop()
	case err := <-errChan:
		if err != nil {
			return err
		}
		return app.Stop()
	}
}

type binanceClientAdapter struct {
	client *binance.Client
}

func (a *binanceClientAdapter) GetStatus() string {
	return "connected"
}

func (a *binanceClientAdapter) Close() error {
	return a.client.Stop()
}

type wsHubAdapter struct {
	hub *server.WebSocketHub
}

func (a *wsHubAdapter) CloseAll() error {
	a.hub.Stop()
	return nil
}

func main() {
	log.Printf("Predictor Backend Starting...")
	log.Printf("Go version: %s", runtime.Version())

	config := LoadConfig()
	log.Printf("Configuration: HTTP_PORT=%s, BUFFER_SIZE=%d", config.HTTPPort, config.BufferSize)

	app := NewApplication(config)

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	log.Println("Application exited successfully")
}

func getFrontendDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "./frontend/dist"
	}

	exeDir := filepath.Dir(exe)
	frontendDir := filepath.Join(exeDir, "frontend", "dist")

	if _, err := os.Stat(frontendDir); err == nil {
		return frontendDir
	}

	return "./frontend/dist"
}
