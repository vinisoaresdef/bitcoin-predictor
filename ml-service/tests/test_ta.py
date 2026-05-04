"""Tests for TA computation module."""

from datetime import datetime

import pytest
from fastapi.testclient import TestClient

from app.schemas import CandleData
from main import app
from ta import compute_all_indicators, compute_sma


@pytest.fixture
def client():
    return TestClient(app)


@pytest.fixture
def sample_candles():
    """Generate 50 sample candles with trending price data."""
    candles = []
    base_price = 42000.0
    for i in range(50):
        close = base_price + i * 100 + (i % 5) * 50
        candle = CandleData(
            open=close - 50,
            high=close + 100,
            low=close - 100,
            close=close,
            volume=1000.0 + i * 100,
            timestamp=datetime(2024, 1, 1, 12, 0, 0),
        )
        candles.append(candle)
    return candles


@pytest.fixture
def insufficient_candles():
    """Generate only 10 candles (insufficient for SMA20)."""
    candles = []
    base_price = 42000.0
    for i in range(10):
        close = base_price + i * 100
        candle = CandleData(
            open=close - 50,
            high=close + 100,
            low=close - 100,
            close=close,
            volume=1000.0 + i * 100,
            timestamp=datetime(2024, 1, 1, 12, 0, 0),
        )
        candles.append(candle)
    return candles


def test_sma_computation(sample_candles):
    """Verify SMA calculation matches expected values."""
    sma = compute_sma(sample_candles, period=20)

    assert sma is not None
    assert isinstance(sma, float)
    assert sma > 0

    # Manual calculation verification
    last_20_closes = [c.close for c in sample_candles[-20:]]
    expected_sma = sum(last_20_closes) / 20
    assert sma == pytest.approx(expected_sma, rel=1e-10)


def test_sma_insufficient_data(insufficient_candles):
    """SMA should return None when insufficient data."""
    sma = compute_sma(insufficient_candles, period=20)
    assert sma is None


def test_compute_all_indicators(sample_candles):
    """Verify all indicators are computed correctly."""
    indicators = compute_all_indicators(sample_candles)

    assert "sma" in indicators
    assert "rsi" in indicators
    assert "macd_histogram" in indicators
    assert "bbands_upper" in indicators
    assert "bbands_middle" in indicators
    assert "bbands_lower" in indicators
    assert "atr" in indicators

    # All should be computed since we have 30 candles
    assert indicators["sma"] is not None
    assert indicators["rsi"] is not None
    assert indicators["macd_histogram"] is not None
    assert indicators["bbands_upper"] is not None
    assert indicators["bbands_middle"] is not None
    assert indicators["bbands_lower"] is not None
    assert indicators["atr"] is not None

    # Type checks
    assert isinstance(indicators["sma"], float)
    assert isinstance(indicators["rsi"], float)
    assert isinstance(indicators["macd_histogram"], float)
    assert isinstance(indicators["bbands_upper"], float)
    assert isinstance(indicators["bbands_middle"], float)
    assert isinstance(indicators["bbands_lower"], float)
    assert isinstance(indicators["atr"], float)


def test_compute_all_indicators_empty():
    """Test with empty candles list."""
    indicators = compute_all_indicators([])

    assert indicators["sma"] is None
    assert indicators["rsi"] is None
    assert indicators["macd_histogram"] is None
    assert indicators["bbands_upper"] is None
    assert indicators["bbands_middle"] is None
    assert indicators["bbands_lower"] is None
    assert indicators["atr"] is None


def test_ta_endpoint(client, sample_candles):
    """POST to /ta should return all requested indicators."""
    request_data = {
        "candles": [
            {
                "open": c.open,
                "high": c.high,
                "low": c.low,
                "close": c.close,
                "volume": c.volume,
                "timestamp": c.timestamp.isoformat(),
            }
            for c in sample_candles
        ],
        "indicators": ["sma", "rsi", "macd", "bbands", "atr"],
    }

    response = client.post("/ta", json=request_data)

    assert response.status_code == 200
    data = response.json()

    assert "sma" in data
    assert "rsi" in data
    assert "macd_histogram" in data
    assert "bbands_upper" in data
    assert "bbands_middle" in data
    assert "bbands_lower" in data
    assert "atr" in data

    # All should have values
    assert data["sma"] is not None
    assert data["rsi"] is not None
    assert data["macd_histogram"] is not None
    assert data["bbands_upper"] is not None
    assert data["bbands_middle"] is not None
    assert data["bbands_lower"] is not None
    assert data["atr"] is not None


def test_ta_endpoint_partial_indicators(client, sample_candles):
    """POST to /ta with partial indicator list."""
    request_data = {
        "candles": [
            {
                "open": c.open,
                "high": c.high,
                "low": c.low,
                "close": c.close,
                "volume": c.volume,
                "timestamp": c.timestamp.isoformat(),
            }
            for c in sample_candles
        ],
        "indicators": ["sma", "rsi"],
    }

    response = client.post("/ta", json=request_data)

    assert response.status_code == 200
    data = response.json()

    # Only requested indicators should be present
    assert data["sma"] is not None
    assert data["rsi"] is not None
    # Others should be None (not computed)
    assert data["macd_histogram"] is None


def test_ta_endpoint_insufficient_data(client, insufficient_candles):
    """POST to /ta with <20 candles should return 422 error."""
    request_data = {
        "candles": [
            {
                "open": c.open,
                "high": c.high,
                "low": c.low,
                "close": c.close,
                "volume": c.volume,
                "timestamp": c.timestamp.isoformat(),
            }
            for c in insufficient_candles
        ],
        "indicators": ["sma"],
    }

    response = client.post("/ta", json=request_data)

    assert response.status_code == 422
    data = response.json()
    assert "detail" in data
    assert "Insufficient data" in data["detail"]
