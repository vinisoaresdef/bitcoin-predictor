# Predictor Platform Schemas

## Wire Format Overview

All inter-service communication uses JSON with Unix timestamps in seconds.

## Go Types (`go-backend/internal/schemas/schemas.go`)

### Candle
```json
{
  "symbol": "BTCUSDT",
  "interval": "1m",
  "open": 50000.0,
  "high": 50100.0,
  "low": 49900.0,
  "close": 50050.0,
  "volume": 100.5,
  "close_time": "2024-01-15T10:30:00Z",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### StatusMessage
```json
{
  "type": "status",
  "status": "connected",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### KlineMessage
```json
{
  "type": "kline",
  "candle": { ... }
}
```

### PredictionMessage
```json
{
  "type": "prediction",
  "direction": "UP",
  "confidence": 0.85,
  "predicted_candle": { ... },
  "predicted_ma": 50100.0,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### IndicatorMessage
```json
{
  "type": "indicator",
  "name": "RSI",
  "values": [30.5, 31.2, 32.1],
  "predicted_values": [33.0, 34.1, 35.2]
}
```

## Python Types (`ml-service/app/schemas.py`)

### CandleData
```python
CandleData(open=50000.0, high=50100.0, low=49900.0, close=50050.0, volume=100.5, timestamp=datetime(...))
```

### PredictionInput
```python
PredictionInput(candles=[CandleData(...), ...], features={"window": 20})
```

### PredictionOutput
```json
{
  "direction": "UP",
  "confidence": 0.85,
  "predicted_candle": { ... },
  "predicted_ma": 50100.0
}
```

## Binance Kline Format (Reference)

Binance WebSocket kline format:
```json
{
  "e": "kline",
  "k": {
    "t": 1672501260000,  // Kline start time (ms)
    "o": "50000.0",
    "h": "50100.0",
    "l": "49900.0",
    "c": "50050.0",
    "v": "100.5"
  }
}
```

Note: Binance uses milliseconds, converted to Unix seconds for internal use.