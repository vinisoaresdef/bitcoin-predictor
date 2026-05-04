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

// PredictionMessage represents a prediction result
type PredictionMessage struct {
	Type            string `json:"type"`
	Direction       string `json:"direction"`
	Confidence      float64 `json:"confidence"`
	PredictedCandle Candle `json:"predicted_candle"`
	PredictedMA     float64 `json:"predicted_ma"`
	Timestamp       time.Time `json:"timestamp"`
}

// IndicatorMessage represents indicator values
type IndicatorMessage struct {
	Type           string    `json:"type"`
	Name           string    `json:"name"`
	Values         []float64 `json:"values"`
	PredictedValues []float64 `json:"predicted_values"`
}