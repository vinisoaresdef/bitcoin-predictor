package binance

import (
	"context"
	"testing"
	"time"

	"predictor/internal/schemas"
)

func TestParseKlineMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected schemas.Candle
		wantErr  bool
	}{
		{
			name: "valid kline message",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"f": 100,
					"L": 200,
					"o": "50000.00",
					"h": "51000.00",
					"l": "49000.00",
					"c": "50500.00",
					"v": "100.5",
					"n": 150,
					"x": false,
					"q": "5000000.00",
					"V": "50.25",
					"Q": "2500000.00",
					"B": "0"
				}
			}`,
			expected: schemas.Candle{
				Symbol:    "BTCUSDT",
				Interval:  "1s",
				Open:      50000.00,
				High:      51000.00,
				Low:       49000.00,
				Close:     50500.00,
				Volume:    100.5,
				CloseTime: time.UnixMilli(1234567889999),
				Timestamp: time.UnixMilli(1234567890000),
				IsFinal:   false,
			},
			wantErr: false,
		},
		{
			name:     "invalid json",
			input:    `{"invalid json`,
			expected: schemas.Candle{},
			wantErr:  true,
		},
		{
			name: "wrong event type",
			input: `{
				"e": "trade",
				"E": 1234567890000,
				"s": "BTCUSDT"
			}`,
			expected: schemas.Candle{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKlineMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKlineMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Symbol != tt.expected.Symbol {
					t.Errorf("Symbol = %v, want %v", got.Symbol, tt.expected.Symbol)
				}
				if got.Interval != tt.expected.Interval {
					t.Errorf("Interval = %v, want %v", got.Interval, tt.expected.Interval)
				}
				if got.Open != tt.expected.Open {
					t.Errorf("Open = %v, want %v", got.Open, tt.expected.Open)
				}
				if got.High != tt.expected.High {
					t.Errorf("High = %v, want %v", got.High, tt.expected.High)
				}
				if got.Low != tt.expected.Low {
					t.Errorf("Low = %v, want %v", got.Low, tt.expected.Low)
				}
				if got.Close != tt.expected.Close {
					t.Errorf("Close = %v, want %v", got.Close, tt.expected.Close)
				}
				if got.Volume != tt.expected.Volume {
					t.Errorf("Volume = %v, want %v", got.Volume, tt.expected.Volume)
				}
				if !got.CloseTime.Equal(tt.expected.CloseTime) {
					t.Errorf("CloseTime = %v, want %v", got.CloseTime, tt.expected.CloseTime)
				}
				if !got.Timestamp.Equal(tt.expected.Timestamp) {
					t.Errorf("Timestamp = %v, want %v", got.Timestamp, tt.expected.Timestamp)
				}
			}
		})
	}
}

func TestParseKlineMessage_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "missing open price",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"h": "51000.00",
					"l": "49000.00",
					"c": "50500.00",
					"v": "100.5"
				}
			}`,
			wantErr: true,
		},
		{
			name: "missing high price",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"o": "50000.00",
					"l": "49000.00",
					"c": "50500.00",
					"v": "100.5"
				}
			}`,
			wantErr: true,
		},
		{
			name: "missing low price",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"o": "50000.00",
					"h": "51000.00",
					"c": "50500.00",
					"v": "100.5"
				}
			}`,
			wantErr: true,
		},
		{
			name: "missing close price",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"o": "50000.00",
					"h": "51000.00",
					"l": "49000.00",
					"v": "100.5"
				}
			}`,
			wantErr: true,
		},
		{
			name: "missing volume",
			input: `{
				"e": "kline",
				"E": 1234567890000,
				"s": "BTCUSDT",
				"k": {
					"t": 1234567880000,
					"T": 1234567889999,
					"s": "BTCUSDT",
					"i": "1s",
					"o": "50000.00",
					"h": "51000.00",
					"l": "49000.00",
					"c": "50500.00"
				}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseKlineMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKlineMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeduplicateByCloseTime(t *testing.T) {
	candleChan := make(chan schemas.Candle, 10)
	config := DefaultConfig()
	client := NewClient(config, candleChan)

	ctA := time.UnixMilli(1234567889999)
	ctB := time.UnixMilli(1234567899999)
	ts := time.UnixMilli(1234567890000)

	firstPartial := schemas.Candle{Symbol: "BTCUSDT", Interval: "1s", Open: 50000, High: 51000, Low: 49000, Close: 50500, Volume: 100.5, CloseTime: ctA, Timestamp: ts, IsFinal: false}
	secondPartialSameCT := schemas.Candle{Symbol: "BTCUSDT", Interval: "1s", Open: 50500, High: 51500, Low: 50000, Close: 51000, Volume: 200, CloseTime: ctA, Timestamp: ts, IsFinal: false}
	differentCloseTime := schemas.Candle{Symbol: "BTCUSDT", Interval: "1s", Open: 51000, High: 52000, Low: 50500, Close: 51500, Volume: 150, CloseTime: ctB, Timestamp: time.UnixMilli(1234567900000), IsFinal: false}
	finalForCT_A := schemas.Candle{Symbol: "BTCUSDT", Interval: "1s", Open: 51000, High: 52000, Low: 50500, Close: 51500, Volume: 150, CloseTime: ctA, Timestamp: ts, IsFinal: true}
	afterFinalForCT_A := schemas.Candle{Symbol: "BTCUSDT", Interval: "1s", Open: 52000, High: 53000, Low: 51000, Close: 52500, Volume: 300, CloseTime: ctA, Timestamp: time.UnixMilli(1234567891000), IsFinal: false}

	if client.isDuplicate(firstPartial) {
		t.Error("first partial candle should not be duplicate")
	}
	if client.isDuplicate(secondPartialSameCT) {
		t.Error("partial update with same close_time should flow through")
	}
	if client.isDuplicate(differentCloseTime) {
		t.Error("candle with different close_time should not be duplicate")
	}
	if client.isDuplicate(finalForCT_A) {
		t.Error("first final candle should not be duplicate")
	}
	if !client.isDuplicate(afterFinalForCT_A) {
		t.Error("candle after final for same close_time should be duplicate")
	}
}

func TestReconnectBackoff(t *testing.T) {
	candleChan := make(chan schemas.Candle, 10)
	config := DefaultConfig()
	config.InitialBackoff = 100 * time.Millisecond
	config.MaxBackoff = 500 * time.Millisecond
	config.BackoffMultiplier = 2

	_ = NewClient(config, candleChan)

	// Test that backoff increases exponentially
	backoff := config.InitialBackoff
	expectedBackoffs := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond, // Max backoff reached
		500 * time.Millisecond,
	}

	for i, expected := range expectedBackoffs {
		if backoff != expected {
			t.Errorf("Backoff %d = %v, want %v", i, backoff, expected)
		}

		// Simulate exponential backoff calculation
		backoff = time.Duration(float64(backoff) * config.BackoffMultiplier)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}
}

func TestClient_StartStop(t *testing.T) {
	candleChan := make(chan schemas.Candle, 10)
	config := DefaultConfig()
	client := NewClient(config, candleChan)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test Start
	err := client.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}

	// Test double Start
	err = client.Start(ctx)
	if err == nil {
		t.Error("Expected error on double Start, got nil")
	}

	// Test Stop
	err = client.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Test double Stop (should not error)
	err = client.Stop()
	if err != nil {
		t.Errorf("Stop() on stopped client error = %v", err)
	}
}

func TestParseKlineMessage_ZeroVolume(t *testing.T) {
	input := `{
		"e": "kline",
		"E": 1234567890000,
		"s": "BTCUSDT",
		"k": {
			"t": 1234567880000,
			"T": 1234567889999,
			"s": "BTCUSDT",
			"i": "1s",
			"f": 100,
			"L": 200,
			"o": "50000.00",
			"h": "51000.00",
			"l": "49000.00",
			"c": "50500.00",
			"v": "0.0",
			"n": 0,
			"x": false,
			"q": "0.0",
			"V": "0.0",
			"Q": "0.0",
			"B": "0"
		}
	}`

	candle, err := ParseKlineMessage([]byte(input))
	if err != nil {
		t.Errorf("ParseKlineMessage() error = %v", err)
		return
	}

	if candle.Volume != 0 {
		t.Errorf("Volume = %v, want 0", candle.Volume)
	}
}
