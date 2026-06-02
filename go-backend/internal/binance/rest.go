package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"predictor/internal/schemas"
)

const (
	DefaultRESTBaseURL  = "https://api.binance.com"
	MaxKlinesPerRequest = 1000
	MaxHistoryCandles   = 1500
	HistoryHours        = 24
)

var restHTTPClient = &http.Client{Timeout: 15 * time.Second}

// FetchKlines fetches historical kline data from the Binance REST API.
// It paginates automatically for requests larger than 1000 candles.
func FetchKlines(baseURL, symbol, interval string, limit int) ([]schemas.Candle, error) {
	if baseURL == "" || limit <= 0 {
		return nil, nil
	}

	intervalSec := IntervalToSeconds(interval)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(int64(limit)*int64(intervalSec)) * time.Second)

	var allCandles []schemas.Candle

	for len(allCandles) < limit {
		batchLimit := limit - len(allCandles)
		if batchLimit > MaxKlinesPerRequest {
			batchLimit = MaxKlinesPerRequest
		}

		url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&startTime=%d&limit=%d",
			baseURL, symbol, interval, startTime.UnixMilli(), batchLimit)

		resp, err := restHTTPClient.Get(url)
		if err != nil {
			return allCandles, fmt.Errorf("REST request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return allCandles, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return allCandles, fmt.Errorf("Binance API returned status %d: %s", resp.StatusCode, string(body))
		}

		var rawKlines [][]json.RawMessage
		if err := json.Unmarshal(body, &rawKlines); err != nil {
			return allCandles, fmt.Errorf("failed to parse klines: %w", err)
		}

		if len(rawKlines) == 0 {
			break
		}

		for _, kline := range rawKlines {
			candle, err := parseRESTKline(kline, symbol, interval)
			if err != nil {
				log.Printf("Skipping invalid kline: %v", err)
				continue
			}
			allCandles = append(allCandles, candle)
		}

		// Advance startTime past the last kline's close time
		var closeTimeMs int64
		if err := json.Unmarshal(rawKlines[len(rawKlines)-1][6], &closeTimeMs); err == nil {
			startTime = time.UnixMilli(closeTimeMs + 1)
		} else {
			break
		}

		if len(rawKlines) < batchLimit {
			break // no more data available
		}
	}

	return allCandles, nil
}

// HistoryCandleCount returns how many candles to fetch for ~24h of data,
// capped at MaxHistoryCandles.
func HistoryCandleCount(interval string) int {
	sec := IntervalToSeconds(interval)
	if sec <= 0 {
		sec = 60
	}
	count := (HistoryHours * 3600) / sec
	if count > MaxHistoryCandles {
		count = MaxHistoryCandles
	}
	if count < 1 {
		count = 1
	}
	return count
}

// IntervalToSeconds converts a Binance interval string to seconds.
func IntervalToSeconds(interval string) int {
	switch interval {
	case "1s":
		return 1
	case "1m":
		return 60
	case "3m":
		return 180
	case "5m":
		return 300
	case "15m":
		return 900
	case "30m":
		return 1800
	case "1h":
		return 3600
	case "4h":
		return 14400
	case "1d":
		return 86400
	default:
		return 60
	}
}

func parseRESTKline(kline []json.RawMessage, symbol, interval string) (schemas.Candle, error) {
	if len(kline) < 7 {
		return schemas.Candle{}, fmt.Errorf("kline has %d fields, need at least 7", len(kline))
	}

	var openTimeMs, closeTimeMs int64
	var openStr, highStr, lowStr, closeStr, volumeStr string

	if err := json.Unmarshal(kline[0], &openTimeMs); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid open time: %w", err)
	}
	if err := json.Unmarshal(kline[1], &openStr); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid open: %w", err)
	}
	if err := json.Unmarshal(kline[2], &highStr); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid high: %w", err)
	}
	if err := json.Unmarshal(kline[3], &lowStr); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid low: %w", err)
	}
	if err := json.Unmarshal(kline[4], &closeStr); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid close: %w", err)
	}
	if err := json.Unmarshal(kline[5], &volumeStr); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid volume: %w", err)
	}
	if err := json.Unmarshal(kline[6], &closeTimeMs); err != nil {
		return schemas.Candle{}, fmt.Errorf("invalid close time: %w", err)
	}

	open, err := strconv.ParseFloat(openStr, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse open: %w", err)
	}
	high, err := strconv.ParseFloat(highStr, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse high: %w", err)
	}
	low, err := strconv.ParseFloat(lowStr, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse low: %w", err)
	}
	closePrice, err := strconv.ParseFloat(closeStr, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse close: %w", err)
	}
	volume, err := strconv.ParseFloat(volumeStr, 64)
	if err != nil {
		return schemas.Candle{}, fmt.Errorf("failed to parse volume: %w", err)
	}

	return schemas.Candle{
		Symbol:    symbol,
		Interval:  interval,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: time.UnixMilli(closeTimeMs),
		Timestamp: time.UnixMilli(openTimeMs),
		IsFinal:   true,
	}, nil
}
