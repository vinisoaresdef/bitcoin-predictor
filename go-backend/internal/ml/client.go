package ml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"predictor/internal/schemas"
)

const (
	DefaultMLServiceURL = "http://ml-service:8000"
	DefaultTimeout      = 10 * time.Second
)

type MLClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

type Config struct {
	BaseURL string
	Timeout time.Duration
}

func DefaultConfig() Config {
	baseURL := os.Getenv("ML_SERVICE_URL")
	if baseURL == "" {
		baseURL = DefaultMLServiceURL
	}

	timeout := DefaultTimeout
	if timeoutStr := os.Getenv("ML_TIMEOUT"); timeoutStr != "" {
		if parsed, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(parsed) * time.Second
		}
	}

	return Config{
		BaseURL: baseURL,
		Timeout: timeout,
	}
}

func NewClient(config Config) *MLClient {
	return &MLClient{
		BaseURL: config.BaseURL,
		HTTPClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func NewDefaultClient() *MLClient {
	return NewClient(DefaultConfig())
}

func convertCandles(candles []schemas.Candle) []schemas.CandleData {
	result := make([]schemas.CandleData, len(candles))
	for i, c := range candles {
		result[i] = schemas.CandleData{
			Timestamp: c.Timestamp,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		}
	}
	return result
}

func (c *MLClient) Predict(candles []schemas.Candle) (*schemas.PredictionResult, error) {
	if len(candles) != 60 {
		return nil, fmt.Errorf("exactly 60 candles required, got %d", len(candles))
	}

	requestBody := schemas.PredictionInput{
		Candles: convertCandles(candles),
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/predict", c.BaseURL)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) || isTimeoutError(err) {
			return nil, fmt.Errorf("request timeout")
		}
		return nil, fmt.Errorf("connection failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusServiceUnavailable:
		return nil, fmt.Errorf("ML service unavailable")
	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("invalid input: %s", string(body))
	case http.StatusOK:
	default:
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var prediction schemas.PredictionResult
	if err := json.Unmarshal(body, &prediction); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &prediction, nil
}

func (c *MLClient) Health() (bool, error) {
	url := fmt.Sprintf("%s/health", c.BaseURL)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if os.IsTimeout(err) || isTimeoutError(err) {
			return false, fmt.Errorf("request timeout")
		}
		return false, fmt.Errorf("connection failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusServiceUnavailable:
		return false, nil
	case http.StatusOK:
	default:
		return false, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var health schemas.HealthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	return health.ModelLoaded, nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
}
