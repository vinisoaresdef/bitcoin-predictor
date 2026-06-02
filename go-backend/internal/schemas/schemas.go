package schemas

import "time"

// Candle represents a single candlestick data point
type Candle struct {
	Symbol    string    `json:"symbol"`
	Interval  string    `json:"interval"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	CloseTime time.Time `json:"close_time"`
	Timestamp time.Time `json:"timestamp"`
	IsFinal   bool      `json:"is_final"`
}

// CandleData represents a single candlestick for ML service input
type CandleData struct {
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

// PredictionResult represents the ML service prediction response
type PredictionResult struct {
	Direction       string    `json:"direction"`
	Confidence      float64   `json:"confidence"`
	PredictedCandle Candle    `json:"predicted_candle"`
	PredictedMA     float64   `json:"predicted_ma"`
}

// PredictionInput represents the ML service /predict endpoint request
type PredictionInput struct {
	Candles []CandleData `json:"candles"`
}

// StatusMessage represents a status update message
type StatusMessage struct {
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// KlineMessage represents a kline/candlestick message from Binance
type KlineMessage struct {
	Type   string `json:"type"`
	Candle Candle `json:"candle"`
}

// PredictionMessage represents a prediction result broadcast to clients
type PredictionMessage struct {
	Type            string    `json:"type"`
	Direction       string    `json:"direction"`
	Confidence      float64   `json:"confidence"`
	PredictedCandle Candle    `json:"predicted_candle"`
	PredictedMA     float64   `json:"predicted_ma"`
	Timestamp       time.Time `json:"timestamp"`
}

// SnapshotMessage sends a batch of candles to the client at once
type SnapshotMessage struct {
	Type    string   `json:"type"`
	Candles []Candle `json:"candles"`
}

// IndicatorMessage represents indicator values
type IndicatorMessage struct {
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	Values          []float64 `json:"values"`
	PredictedValues []float64 `json:"predicted_values"`
}

// TimeframeChangeRequest represents a client request to change timeframe
type TimeframeChangeRequest struct {
	Type      string `json:"type"`
	Timeframe string `json:"timeframe"`
	Ticker    string `json:"ticker,omitempty"`
}

// TimeframeChangeResponse represents the server response to a timeframe change
type TimeframeChangeResponse struct {
	Type      string `json:"type"`
	Timeframe string `json:"timeframe"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// TickerSubscribeRequest represents a client request to subscribe to a ticker
type TickerSubscribeRequest struct {
	Type      string `json:"type"`
	Ticker    string `json:"ticker"`
	Timeframe string `json:"timeframe"`
}

// TickerUnsubscribeRequest represents a client request to unsubscribe from a ticker
type TickerUnsubscribeRequest struct {
	Type   string `json:"type"`
	Ticker string `json:"ticker"`
}

// TickerSubscriptionResponse represents the server response to a subscription request
type TickerSubscriptionResponse struct {
	Type      string `json:"type"`
	Ticker    string `json:"ticker"`
	Timeframe string `json:"timeframe"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// TickerListMessage represents the list of active tickers
type TickerListMessage struct {
	Type    string   `json:"type"`
	Tickers []string `json:"tickers"`
}

// HealthResponse represents the ML service /health endpoint response
type HealthResponse struct {
	Status       string `json:"status"`
	ModelLoaded  bool   `json:"model_loaded"`
	ModelVersion string `json:"model_version"`
}
