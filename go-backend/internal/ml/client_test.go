package ml

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"predictor/internal/schemas"
)

func TestMLClient_Predict(t *testing.T) {
	tests := []struct {
		name           string
		candles        []schemas.Candle
		serverResponse *schemas.PredictionResult
		serverStatus   int
		wantErr        bool
		errContains    string
	}{
		{
			name:    "successful prediction",
			candles: generateTestCandles(60),
			serverResponse: &schemas.PredictionResult{
				Direction:  "UP",
				Confidence: 0.75,
			},
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:    "prediction DOWN",
			candles: generateTestCandles(60),
			serverResponse: &schemas.PredictionResult{
				Direction:  "DOWN",
				Confidence: 0.80,
			},
			serverStatus: http.StatusOK,
			wantErr:      false,
		},
		{
			name:        "wrong number of candles",
			candles:     generateTestCandles(59),
			wantErr:     true,
			errContains: "exactly 60 candles required",
		},
		{
			name:         "ML service unavailable",
			candles:      generateTestCandles(60),
			serverStatus: http.StatusServiceUnavailable,
			wantErr:      true,
			errContains:  "ML service unavailable",
		},
		{
			name:         "invalid input",
			candles:      generateTestCandles(60),
			serverStatus: http.StatusUnprocessableEntity,
			wantErr:      true,
			errContains:  "invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/predict" {
					t.Errorf("Expected path /predict, got %s", r.URL.Path)
				}
				contentType := r.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				if tt.serverStatus != 0 {
					w.WriteHeader(tt.serverStatus)
				}
				if tt.serverResponse != nil {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			client := NewClient(Config{
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			})

			result, err := client.Predict(tt.candles)
			if (err != nil) != tt.wantErr {
				t.Errorf("Predict() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsStr(err.Error(), tt.errContains) {
					t.Errorf("Predict() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Expected non-nil result")
					return
				}
				if result.Direction != tt.serverResponse.Direction {
					t.Errorf("Direction = %v, want %v", result.Direction, tt.serverResponse.Direction)
				}
				if result.Confidence != tt.serverResponse.Confidence {
					t.Errorf("Confidence = %v, want %v", result.Confidence, tt.serverResponse.Confidence)
				}
			}
		})
	}
}

func TestMLClient_Predict_RequestDetails(t *testing.T) {
	var receivedRequest schemas.PredictionInput
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedRequest)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(schemas.PredictionResult{
			Direction:  "UP",
			Confidence: 0.75,
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	candles := generateTestCandles(60)
	_, err := client.Predict(candles)
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	if len(receivedRequest.Candles) != 60 {
		t.Errorf("Expected 60 candles in request, got %d", len(receivedRequest.Candles))
	}

	for i, c := range receivedRequest.Candles {
		if c.Open != candles[i].Open {
			t.Errorf("Candle %d Open mismatch: got %v, want %v", i, c.Open, candles[i].Open)
		}
		if c.High != candles[i].High {
			t.Errorf("Candle %d High mismatch: got %v, want %v", i, c.High, candles[i].High)
		}
		if c.Low != candles[i].Low {
			t.Errorf("Candle %d Low mismatch: got %v, want %v", i, c.Low, candles[i].Low)
		}
		if c.Close != candles[i].Close {
			t.Errorf("Candle %d Close mismatch: got %v, want %v", i, c.Close, candles[i].Close)
		}
		if c.Volume != candles[i].Volume {
			t.Errorf("Candle %d Volume mismatch: got %v, want %v", i, c.Volume, candles[i].Volume)
		}
	}
}

func TestMLClient_Health(t *testing.T) {
	tests := []struct {
		name         string
		serverResp   schemas.HealthResponse
		serverStatus int
		wantHealthy  bool
		wantErr      bool
		errContains  string
	}{
		{
			name: "healthy service",
			serverResp: schemas.HealthResponse{
				Status:       "ok",
				ModelLoaded:  true,
				ModelVersion: "1.0.0",
			},
			serverStatus: http.StatusOK,
			wantHealthy:  true,
			wantErr:      false,
		},
		{
			name: "model not loaded",
			serverResp: schemas.HealthResponse{
				Status:       "error",
				ModelLoaded:  false,
				ModelVersion: "1.0.0",
			},
			serverStatus: http.StatusServiceUnavailable,
			wantHealthy:  false,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.URL.Path != "/health" {
					t.Errorf("Expected path /health, got %s", r.URL.Path)
				}

				w.WriteHeader(tt.serverStatus)
				json.NewEncoder(w).Encode(tt.serverResp)
			}))
			defer server.Close()

			client := NewClient(Config{
				BaseURL: server.URL,
				Timeout: 5 * time.Second,
			})

			healthy, err := client.Health()
			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !containsStr(err.Error(), tt.errContains) {
					t.Errorf("Health() error = %v, should contain %v", err, tt.errContains)
				}
				return
			}
			if healthy != tt.wantHealthy {
				t.Errorf("Health() = %v, want %v", healthy, tt.wantHealthy)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.BaseURL != DefaultMLServiceURL {
		t.Errorf("DefaultConfig() BaseURL = %v, want %v", config.BaseURL, DefaultMLServiceURL)
	}
	if config.Timeout != DefaultTimeout {
		t.Errorf("DefaultConfig() Timeout = %v, want %v", config.Timeout, DefaultTimeout)
	}
}

func TestDefaultConfig_EnvVars(t *testing.T) {
	os.Setenv("ML_SERVICE_URL", "http://custom-ml:9000")
	os.Setenv("ML_TIMEOUT", "30")
	defer os.Unsetenv("ML_SERVICE_URL")
	defer os.Unsetenv("ML_TIMEOUT")

	config := DefaultConfig()
	if config.BaseURL != "http://custom-ml:9000" {
		t.Errorf("DefaultConfig() BaseURL = %v, want http://custom-ml:9000", config.BaseURL)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("DefaultConfig() Timeout = %v, want 30s", config.Timeout)
	}
}

func TestNewDefaultClient(t *testing.T) {
	client := NewDefaultClient()
	if client == nil {
		t.Error("NewDefaultClient() returned nil")
	}
	if client.BaseURL != DefaultMLServiceURL {
		t.Errorf("NewDefaultClient() BaseURL = %v, want %v", client.BaseURL, DefaultMLServiceURL)
	}
	if client.HTTPClient == nil {
		t.Error("NewDefaultClient() HTTPClient is nil")
	}
	if client.HTTPClient.Timeout != DefaultTimeout {
		t.Errorf("NewDefaultClient() Timeout = %v, want %v", client.HTTPClient.Timeout, DefaultTimeout)
	}
}

func TestMLClient_Predict_ConnectionFailed(t *testing.T) {
	client := NewClient(Config{
		BaseURL: "http://localhost:1",
		Timeout: 100 * time.Millisecond,
	})

	candles := generateTestCandles(60)
	_, err := client.Predict(candles)
	if err == nil {
		t.Error("Expected error for connection failure")
	}
	if !containsStr(err.Error(), "connection failed") {
		t.Errorf("Expected 'connection failed' error, got: %v", err)
	}
}

func TestMLClient_Health_ConnectionFailed(t *testing.T) {
	client := NewClient(Config{
		BaseURL: "http://localhost:1",
		Timeout: 100 * time.Millisecond,
	})

	_, err := client.Health()
	if err == nil {
		t.Error("Expected error for connection failure")
	}
	if !containsStr(err.Error(), "connection failed") {
		t.Errorf("Expected 'connection failed' error, got: %v", err)
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      fmt.Errorf("request timeout"),
			expected: true,
		},
		{
			name:     "deadline exceeded",
			err:      fmt.Errorf("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeoutError(tt.err)
			if result != tt.expected {
				t.Errorf("isTimeoutError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func generateTestCandles(count int) []schemas.Candle {
	candles := make([]schemas.Candle, count)
	now := time.Now()
	for i := 0; i < count; i++ {
		candles[i] = schemas.Candle{
			Symbol:    "BTCUSDT",
			Interval:  "1m",
			Open:      50000.0 + float64(i),
			High:      51000.0 + float64(i),
			Low:       49000.0 + float64(i),
			Close:     50500.0 + float64(i),
			Volume:    100.0 + float64(i),
			CloseTime: now.Add(time.Duration(i) * time.Minute),
			Timestamp: now.Add(time.Duration(i) * time.Minute),
		}
	}
	return candles
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
