package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"predictor/internal/ml"
	"predictor/internal/schemas"
	"predictor/internal/server"
	"predictor/internal/ticker"
)

type MLClient interface {
	Predict(candles []schemas.Candle) (*schemas.PredictionResult, error)
}

type Application struct {
	tickerManager *ticker.TickerManager
	broadcastChan chan schemas.Candle
	mlClient      MLClient
	wsHub         *server.WebSocketHub
	httpServer    *server.HTTPServer
	httpAddr      string
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	config        Config
}

type Config struct {
	HTTPPort           string
	FrontendDir        string
	BinanceWSURL       string
	BinanceRESTURL     string
	BufferSize         int
	PredictionInterval time.Duration
	EnablePredictions  bool
	MLServiceURL       string
}

func LoadConfig() Config {
	predictionInterval, _ := time.ParseDuration(getEnv("PREDICTION_INTERVAL", "30s"))
	if predictionInterval == 0 {
		predictionInterval = 30 * time.Second
	}

	enablePredictions := getEnv("ENABLE_PREDICTIONS", "true") != "false"

	return Config{
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		FrontendDir:        getEnv("FRONTEND_DIR", "./frontend"),
		BinanceWSURL:       getEnv("BINANCE_WS_URL", "wss://stream.binance.com:9443/ws/btcusdt@kline_1s"),
		BinanceRESTURL:     getEnv("BINANCE_REST_URL", "https://api.binance.com"),
		BufferSize:         1500,
		PredictionInterval: predictionInterval,
		EnablePredictions:  enablePredictions,
		MLServiceURL:       getEnv("ML_SERVICE_URL", ml.DefaultMLServiceURL),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewApplication(config Config) *Application {
	mlConfig := ml.DefaultConfig()
	mlConfig.BaseURL = config.MLServiceURL
	mlClient := ml.NewClient(mlConfig)
	return newApplicationWithMLClient(config, mlClient)
}

func NewApplicationWithML(config Config, mlClient MLClient) *Application {
	return newApplicationWithMLClient(config, mlClient)
}

func newApplicationWithMLClient(config Config, mlClient MLClient) *Application {
	ctx, cancel := context.WithCancel(context.Background())

	tm := ticker.NewManager(config.BufferSize, config.BinanceRESTURL)

	if err := tm.Subscribe("BTCUSDT", "1s"); err != nil {
		log.Printf("Warning: failed to subscribe to BTCUSDT: %v", err)
	}

	broadcastChan := make(chan schemas.Candle, 256)

	bufAdapter := &tickerBufferAdapter{tm: tm}

	wsHub := server.NewWebSocketHub(bufAdapter, broadcastChan)
	wsHub.SetTickerManager(tm)

	httpAddr := ":" + config.HTTPPort
	httpConfig := server.Config{
		Addr:          httpAddr,
		FrontendDir:   config.FrontendDir,
		BinanceClient: &tickerStatusAdapter{tm: tm},
		Buffer:        &tickerBufferLenAdapter{tm: tm},
		WSHub:         &wsHubAdapter{hub: wsHub},
	}

	httpServer := server.NewHTTPServer(httpConfig)

	return &Application{
		tickerManager: tm,
		broadcastChan: broadcastChan,
		mlClient:      mlClient,
		wsHub:         wsHub,
		httpServer:    httpServer,
		httpAddr:      httpAddr,
		ctx:           ctx,
		cancel:        cancel,
		config:        config,
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

	if app.config.EnablePredictions {
		app.wg.Add(1)
		go func() {
			defer app.wg.Done()
			log.Println("Prediction worker started")
			app.predictionWorker()
			log.Println("Prediction worker stopped")
		}()
	}

	log.Printf("HTTP server starting on %s", app.httpAddr)
	if err := app.httpServer.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

func (app *Application) candleBufferConsumer() {
	for {
		select {
		case candle, ok := <-app.tickerManager.OutputChan():
			if !ok {
				return
			}
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

func (app *Application) predictionWorker() {
	if !app.config.EnablePredictions {
		return
	}

	predTicker := time.NewTicker(app.config.PredictionInterval)
	defer predTicker.Stop()

	log.Printf("Prediction worker running with interval: %v", app.config.PredictionInterval)

	for {
		select {
		case <-predTicker.C:
			app.runPrediction()
		case <-app.ctx.Done():
			return
		}
	}
}

func (app *Application) runPrediction() {
	buf, ok := app.tickerManager.GetBuffer("BTCUSDT")
	if !ok {
		log.Println("BTCUSDT buffer not found, skipping prediction")
		return
	}

	const minCandlesForPrediction = 60

	candles := buf.Snapshot()
	if len(candles) < minCandlesForPrediction {
		log.Printf("Buffer has %d candles, need %d for prediction", len(candles), minCandlesForPrediction)
		return
	}

	// Use only the last 60 candles for prediction
	candles = candles[len(candles)-minCandlesForPrediction:]

	// Forward-fill zero volumes to prevent ML service rejection
	var lastVolume float64 = 0.001
	for i := range candles {
		if candles[i].Volume > 0 {
			lastVolume = candles[i].Volume
		} else {
			candles[i].Volume = lastVolume
		}
	}

	result, err := app.mlClient.Predict(candles)
	if err != nil {
		log.Printf("ML prediction error: %v", err)
		app.wsHub.BroadcastStatus("ML unavailable")
		return
	}

	if result == nil {
		log.Println("ML prediction returned nil result")
		app.wsHub.BroadcastStatus("ML unavailable")
		return
	}

	app.wsHub.BroadcastPrediction(result)
	log.Printf("Prediction broadcasted: direction=%s, confidence=%.2f", result.Direction, result.Confidence)
}

func (app *Application) Stop() error {
	log.Println("Initiating graceful shutdown...")

	app.cancel()

	if app.tickerManager != nil {
		app.tickerManager.StopAll()
	}

	app.wsHub.Stop()

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

type tickerBufferAdapter struct {
	tm *ticker.TickerManager
}

func (a *tickerBufferAdapter) GetSnapshot() []schemas.Candle {
	return a.tm.GetDefaultSnapshot()
}

type tickerStatusAdapter struct {
	tm *ticker.TickerManager
}

func (a *tickerStatusAdapter) GetStatus() string {
	if a.tm == nil || len(a.tm.ListTickers()) == 0 {
		return "disconnected"
	}
	return "connected"
}

func (a *tickerStatusAdapter) Close() error {
	if a.tm != nil {
		a.tm.StopAll()
	}
	return nil
}

type tickerBufferLenAdapter struct {
	tm *ticker.TickerManager
}

func (a *tickerBufferLenAdapter) Len() int {
	if a.tm == nil {
		return 0
	}
	buf, ok := a.tm.GetBuffer("BTCUSDT")
	if !ok {
		return 0
	}
	return buf.Len()
}

type wsHubAdapter struct {
	hub *server.WebSocketHub
}

func (a *wsHubAdapter) CloseAll() error {
	a.hub.Stop()
	return nil
}

func (a *wsHubAdapter) HandleConnection(w http.ResponseWriter, r *http.Request) {
	a.hub.HandleConnection(w, r)
}

func main() {
	log.Printf("Predictor Backend Starting...")
	log.Printf("Go version: %s", runtime.Version())

	config := LoadConfig()
	log.Printf("Configuration: HTTP_PORT=%s, BUFFER_SIZE=%d, PREDICTION_INTERVAL=%v, ENABLE_PREDICTIONS=%v",
		config.HTTPPort, config.BufferSize, config.PredictionInterval, config.EnablePredictions)

	app := NewApplication(config)

	if err := app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}

	log.Println("Application exited successfully")
}
