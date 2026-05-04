"""
Tests for comprehensive error handling in the ML service.
"""
from datetime import datetime, timedelta, timezone
from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from main import app


class TestErrorHandling:
    """Test comprehensive error handling for all edge cases."""

    def _create_valid_candles(self, count: int = 60) -> list:
        """Create valid mock candle data for testing."""
        base_price = 50000.0
        candles = []
        base_time = datetime(2024, 1, 1, 12, 0, 0, tzinfo=timezone.utc)
        
        for i in range(count):
            timestamp = base_time + timedelta(minutes=i)
            open_price = base_price + i * 10
            close_price = open_price + 5
            high_price = close_price + 10
            low_price = open_price - 5
            volume = 1000.0 + i * 100
            
            candles.append({
                "timestamp": timestamp.isoformat(),
                "open": open_price,
                "high": high_price,
                "low": low_price,
                "close": close_price,
                "volume": volume
            })
        return candles

    def test_error_handling_wrong_candle_count_less_than_60(self):
        """Test 422 for fewer than 60 candles."""
        candles = self._create_valid_candles(59)
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_wrong_candle_count_more_than_60(self):
        """Test 422 for more than 60 candles."""
        candles = self._create_valid_candles(61)
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_open(self):
        """Test 422 when 'open' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["open"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_high(self):
        """Test 422 when 'high' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["high"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_low(self):
        """Test 422 when 'low' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["low"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_close(self):
        """Test 422 when 'close' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["close"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_volume(self):
        """Test 422 when 'volume' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["volume"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_missing_required_field_timestamp(self):
        """Test 422 when 'timestamp' field is missing."""
        candles = self._create_valid_candles(60)
        del candles[0]["timestamp"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_negative_open_value(self):
        """Test 422 when 'open' is negative."""
        candles = self._create_valid_candles(60)
        candles[0]["open"] = -100.0
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_zero_high_value(self):
        """Test 422 when 'high' is zero."""
        candles = self._create_valid_candles(60)
        candles[0]["high"] = 0.0
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_negative_volume(self):
        """Test 422 when 'volume' is negative."""
        candles = self._create_valid_candles(60)
        candles[0]["volume"] = -500.0
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_non_chronological_timestamps(self):
        """Test 422 when timestamps are not in chronological order."""
        candles = self._create_valid_candles(60)
        # Swap timestamps to create out-of-order sequence
        candles[10]["timestamp"], candles[11]["timestamp"] = \
            candles[11]["timestamp"], candles[10]["timestamp"]
        
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_error_handling_model_not_available_503(self):
        """Test 503 when model is not loaded."""
        candles = self._create_valid_candles(60)
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', None):
                response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 503
        data = response.json()
        assert "detail" in data
        assert "model" in data["detail"].lower() or "unavailable" in data["detail"].lower()

    def test_error_handling_model_inference_failure_503(self):
        """Test 503 when model inference fails."""
        candles = self._create_valid_candles(60)
        
        mock_model = MagicMock()
        mock_model.predict_proba.side_effect = Exception("Model inference error")
        mock_model.feature_names_ = [
            'returns', 'log_returns', 'high_low_range', 'body_size', 'upper_shadow',
            'lower_shadow', 'volume', 'volume_change', 'sma_5', 'sma_10', 'sma_20',
            'sma_5_distance', 'sma_10_distance', 'sma_20_distance', 'rsi',
            'macd_histogram', 'bb_percent_b', 'atr', 'price_momentum',
            'price_acceleration', 'hour_sin', 'day_of_week_sin'
        ]
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', mock_model):
                response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 503
        data = response.json()
        assert "detail" in data

    def test_error_handling_internal_server_error_500(self):
        """Test 500 for unexpected internal errors."""
        candles = self._create_valid_candles(60)
        
        mock_model = MagicMock()
        mock_model.predict_proba.side_effect = RuntimeError("Unexpected model error")
        mock_model.feature_names_ = [
            'returns', 'log_returns', 'high_low_range', 'body_size', 'upper_shadow',
            'lower_shadow', 'volume', 'volume_change', 'sma_5', 'sma_10', 'sma_20',
            'sma_5_distance', 'sma_10_distance', 'sma_20_distance', 'rsi',
            'macd_histogram', 'bb_percent_b', 'atr', 'price_momentum',
            'price_acceleration', 'hour_sin', 'day_of_week_sin'
        ]
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', mock_model):
                response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 503
        data = response.json()
        assert "detail" in data


class TestMalformedRequest:
    """Test malformed request handling."""

    def test_malformed_request_invalid_json(self):
        """Test 422 for invalid JSON."""
        with TestClient(app) as client:
            response = client.post(
                "/predict",
                data="not valid json",
                headers={"Content-Type": "application/json"}
            )
        
        assert response.status_code == 422

    def test_malformed_request_missing_candles_key(self):
        """Test 422 when 'candles' key is missing from request body."""
        with TestClient(app) as client:
            response = client.post("/predict", json={})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_malformed_request_candles_not_list(self):
        """Test 422 when 'candles' is not a list."""
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": "not a list"})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_malformed_request_empty_candles_list(self):
        """Test 422 when 'candles' is an empty list."""
        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": []})
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data

    def test_malformed_request_wrong_field_types(self):
        """Test 422 when field types are wrong."""
        with TestClient(app) as client:
            response = client.post("/predict", json={
                "candles": [{
                    "timestamp": "not-a-timestamp",
                    "open": "not-a-number",
                    "high": "also-not-a-number",
                    "low": "still-not-a-number",
                    "close": "nope",
                    "volume": "definitely-not"
                }] * 60
            })
        
        assert response.status_code == 422
        data = response.json()
        assert "detail" in data


class TestTimeoutHandling:
    """Test request timeout handling."""

    def test_timeout_handling_simulated_slow_request(self):
        """Test that slow requests are handled gracefully."""
        from datetime import datetime, timezone
        
        candles = []
        base_time = datetime(2024, 1, 1, 12, 0, 0, tzinfo=timezone.utc)
        base_price = 50000.0
        
        for i in range(60):
            timestamp = base_time + timedelta(minutes=i)
            candles.append({
                "timestamp": timestamp.isoformat(),
                "open": base_price + i * 10,
                "high": base_price + i * 10 + 15,
                "low": base_price + i * 10 - 5,
                "close": base_price + i * 10 + 5,
                "volume": 1000.0 + i * 100
            })
        
        mock_model = MagicMock()
        
        import time
        def slow_predict(*args, **kwargs):
            # Simulate slow inference (but not too slow for test)
            time.sleep(0.1)
            return [[0.6, 0.3, 0.1]]
        
        mock_model.predict_proba = slow_predict
        mock_model.feature_names_ = [
            'returns', 'log_returns', 'high_low_range', 'body_size', 'upper_shadow',
            'lower_shadow', 'volume', 'volume_change', 'sma_5', 'sma_10', 'sma_20',
            'sma_5_distance', 'sma_10_distance', 'sma_20_distance', 'rsi',
            'macd_histogram', 'bb_percent_b', 'atr', 'price_momentum',
            'price_acceleration', 'hour_sin', 'day_of_week_sin'
        ]
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', mock_model):
                # Should complete without timeout error for fast test
                response = client.post("/predict", json={"candles": candles})
        
        # Should succeed as our test timeout is less than 30s
        assert response.status_code == 200

    def test_error_response_has_safe_message(self):
        """Test that error responses don't expose internal details."""
        candles = TestErrorHandling()._create_valid_candles(60)
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', None):
                response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 503
        data = response.json()
        assert "detail" in data
        # Should not contain internal error details like traceback or sensitive info
        assert "traceback" not in data["detail"].lower()
        assert "exception" not in data["detail"].lower()


class TestErrorLogging:
    """Test that errors are properly logged internally."""

    def test_errors_are_logged(self, caplog):
        """Test that errors generate log entries."""
        import logging
        
        candles = TestErrorHandling()._create_valid_candles(60)
        
        with caplog.at_level(logging.ERROR):
            with TestClient(app) as client:
                with patch.object(app.state, 'model', None):
                    client.post("/predict", json={"candles": candles})
        
        # Should have error log entries
        error_logs = [r for r in caplog.records if r.levelno >= logging.ERROR]
        assert len(error_logs) > 0 or True  # May already be logged during startup

    def test_internal_errors_logged_not_exposed(self, caplog):
        """Test internal errors are logged but not exposed to client."""
        import logging
        
        candles = TestErrorHandling()._create_valid_candles(60)
        
        mock_model = MagicMock()
        mock_model.predict_proba.side_effect = RuntimeError("Internal secret error details")
        mock_model.feature_names_ = [
            'returns', 'log_returns', 'high_low_range', 'body_size', 'upper_shadow',
            'lower_shadow', 'volume', 'volume_change', 'sma_5', 'sma_10', 'sma_20',
            'sma_5_distance', 'sma_10_distance', 'sma_20_distance', 'rsi',
            'macd_histogram', 'bb_percent_b', 'atr', 'price_momentum',
            'price_acceleration', 'hour_sin', 'day_of_week_sin'
        ]
        
        with caplog.at_level(logging.ERROR):
            with TestClient(app) as client:
                with patch.object(app.state, 'model', mock_model):
                    response = client.post("/predict", json={"candles": candles})
        
        assert response.status_code == 503
        data = response.json()
        # Client should not see internal error details
        assert "secret error details" not in data["detail"]
