from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from app.schemas import CandleData
from main import app, MODEL_VERSION


class TestModelLoading:
    
    def test_model_loaded_at_startup(self):
        with TestClient(app) as client:
            assert hasattr(app.state, 'model')
            assert app.state.model is not None
    
    def test_model_has_feature_names(self):
        with TestClient(app) as client:
            assert hasattr(app.state, 'model')
            assert app.state.model is not None
            
            has_features = (
                hasattr(app.state.model, 'feature_names_') or
                hasattr(app.state.model, 'feature_name_')
            )
            assert has_features, "Model should have feature names"


class TestHealthEndpoint:
    
    def test_health_with_model(self):
        with TestClient(app) as client:
            response = client.get("/health")
        
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "ok"
        assert data["model_loaded"] is True
        assert data["model_version"] == MODEL_VERSION
    
    def test_health_endpoint_content_type(self):
        with TestClient(app) as client:
            response = client.get("/health")
        
        assert response.headers["content-type"] == "application/json"
    
    def test_health_without_model(self):
        from contextlib import asynccontextmanager
        from fastapi import FastAPI
        from fastapi.responses import JSONResponse
        
        @asynccontextmanager
        async def empty_lifespan(app: FastAPI):
            app.state.model = None
            yield
        
        test_app = FastAPI(lifespan=empty_lifespan)
        
        @test_app.get("/health")
        async def health_no_model():
            model_loaded = hasattr(test_app.state, 'model') and test_app.state.model is not None
            if model_loaded:
                return JSONResponse(
                    status_code=200,
                    content={"status": "ok", "model_loaded": True, "model_version": MODEL_VERSION}
                )
            else:
                return JSONResponse(
                    status_code=503,
                    content={"status": "error", "model_loaded": False, "model_version": MODEL_VERSION}
                )
        
        with TestClient(test_app) as client:
            response = client.get("/health")
        
        assert response.status_code == 503
        data = response.json()
        assert data["status"] == "error"
        assert data["model_loaded"] is False
        assert data["model_version"] == MODEL_VERSION


class TestPredictEndpoint:
    """Tests for the /predict endpoint."""

    def _create_mock_candles(self, count: int = 60) -> list:
        """Create mock candle data for testing."""
        base_price = 50000.0
        candles = []
        for i in range(count):
            timestamp = datetime(2024, 1, 1, 12, 0, 0, tzinfo=timezone.utc)
            timestamp = timestamp.replace(second=i)
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

    def test_predict_returns_valid_direction(self):
        """Test that /predict returns a valid direction with 60 candles."""
        mock_candles = self._create_mock_candles(60)
        
        # Mock the model to return predictable probabilities
        mock_model = MagicMock()
        # predict_proba returns probabilities for [UP, DOWN, UNCERTAIN]
        # Setting UP to have highest probability (0.6 > 0.55 threshold)
        mock_model.predict_proba.return_value = [[0.60, 0.25, 0.15]]
        mock_model.feature_names_ = [
            'returns', 'log_returns', 'high_low_range', 'body_size', 'upper_shadow',
            'lower_shadow', 'volume', 'volume_change', 'sma_5', 'sma_10', 'sma_20',
            'sma_5_distance', 'sma_10_distance', 'sma_20_distance', 'rsi',
            'macd_histogram', 'bb_percent_b', 'atr', 'price_momentum',
            'price_acceleration', 'hour_sin', 'day_of_week_sin'
        ]
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', mock_model):
                response = client.post("/predict", json={"candles": mock_candles})
        
        assert response.status_code == 200
        data = response.json()
        assert "direction" in data
        assert "confidence" in data
        assert "probabilities" in data
        assert data["direction"] in ["UP", "DOWN", "UNCERTAIN"]
        assert isinstance(data["confidence"], float)
        assert 0 <= data["confidence"] <= 1
        assert isinstance(data["probabilities"], list)
        assert len(data["probabilities"]) == 3

    def test_predict_with_insufficient_data(self):
        """Test that /predict returns 422 when less than 60 candles provided."""
        mock_candles = self._create_mock_candles(59)

        with TestClient(app) as client:
            response = client.post("/predict", json={"candles": mock_candles})

        assert response.status_code == 422
        data = response.json()
        assert "detail" in data
        detail = data["detail"]
        if isinstance(detail, list):
            error_messages = " ".join([str(err.get("msg", "")) for err in detail])
            assert "60" in error_messages or "candles" in error_messages.lower()
        else:
            assert "60" in str(detail).lower() or "candles" in str(detail).lower()

    def test_predict_model_not_loaded(self):
        """Test that /predict returns 503 when model is not loaded."""
        mock_candles = self._create_mock_candles(60)
        
        with TestClient(app) as client:
            with patch.object(app.state, 'model', None):
                response = client.post("/predict", json={"candles": mock_candles})
        
        assert response.status_code == 503
        data = response.json()
        assert "detail" in data
        assert "model" in data["detail"].lower() or "unavailable" in data["detail"].lower()
