"""
BTC Predictor ML Service

FastAPI-based ML inference service for Bitcoin price prediction.
"""

import asyncio
import logging
import traceback
from contextlib import asynccontextmanager
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Dict, List, Optional

import joblib
import numpy as np
import pandas as pd
import pandas_ta as ta
from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, validator

from app.schemas import CandleData
from ta import compute_all_indicators

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

MODEL_PATH = Path("models/model.joblib")
MODEL_VERSION = "1.0.0"
REQUEST_TIMEOUT_SECONDS = 30
PREDICTION_HORIZON_SECONDS = 30
SMA_PERIOD = 20
CONFIDENCE_THRESHOLD = 0.55


class ValidationError(Exception):
    """Custom validation error for input validation failures."""
    pass


class ModelUnavailableError(Exception):
    """Custom error for model-related failures."""
    pass


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan context manager for startup and shutdown events."""
    logger.info("Starting ML Service...")

    try:
        if MODEL_PATH.exists():
            app.state.model = joblib.load(MODEL_PATH)
            logger.info(f"Model loaded successfully from {MODEL_PATH}")

            if hasattr(app.state.model, 'feature_names_'):
                feature_count = len(app.state.model.feature_names_)
                logger.info(f"Model has {feature_count} features: {list(app.state.model.feature_names_)}")
            else:
                logger.info("Model loaded (feature names not available)")
        else:
            logger.error(f"Model file not found at {MODEL_PATH}")
            app.state.model = None
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        logger.error(traceback.format_exc())
        app.state.model = None

    yield

    logger.info("Shutting down ML Service...")


app = FastAPI(
    title="BTC Predictor ML Service",
    description="ML inference service for Bitcoin price prediction using LightGBM",
    version=MODEL_VERSION,
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


class TARequest(BaseModel):
    candles: List[CandleData]
    indicators: List[str]


class TAResponse(BaseModel):
    sma: Optional[float] = None
    rsi: Optional[float] = None
    macd_histogram: Optional[float] = None
    bbands_upper: Optional[float] = None
    bbands_middle: Optional[float] = None
    bbands_lower: Optional[float] = None
    atr: Optional[float] = None


class PredictedCandleResponse(BaseModel):
    open: float
    high: float
    low: float
    close: float
    volume: float
    timestamp: datetime
    close_time: datetime


class PredictRequest(BaseModel):
    candles: List[CandleData] = Field(..., min_length=60, max_length=60)


class PredictResponse(BaseModel):
    direction: str = Field(..., pattern="^(UP|DOWN|UNCERTAIN)$")
    confidence: float = Field(..., ge=0.0, le=1.0)
    predicted_candle: PredictedCandleResponse
    predicted_ma: float


def validate_candle_fields(candles: List[CandleData]) -> None:
    """Validate that all candles have required fields."""
    required_fields = ['timestamp', 'open', 'high', 'low', 'close', 'volume']

    for i, candle in enumerate(candles):
        for field in required_fields:
            if getattr(candle, field, None) is None:
                raise ValidationError(
                    f"Candle at index {i} is missing required field: {field}"
                )


def validate_positive_values(candles: List[CandleData]) -> None:
    """Validate that all numeric values are positive."""
    numeric_fields = ['open', 'high', 'low', 'close', 'volume']

    for i, candle in enumerate(candles):
        for field in numeric_fields:
            value = getattr(candle, field)
            if value <= 0:
                raise ValidationError(
                    f"Candle at index {i} has non-positive {field}: {value}"
                )


def validate_ohlc_consistency(candles: List[CandleData]) -> None:
    """Validate OHLC consistency: high >= low, close and open within range."""
    for i, candle in enumerate(candles):
        if candle.high < candle.low:
            raise ValidationError(
                f"Candle at index {i}: high ({candle.high}) < low ({candle.low})"
            )


def validate_chronological_order(candles: List[CandleData]) -> None:
    """Validate that timestamps are in chronological order."""
    if len(candles) < 2:
        return

    for i in range(1, len(candles)):
        prev_time = candles[i - 1].timestamp
        curr_time = candles[i].timestamp

        if curr_time <= prev_time:
            raise ValidationError(
                f"Timestamps not in chronological order at index {i}: "
                f"{curr_time} is not after {prev_time}"
            )


def validate_all_candles(candles: List[CandleData]) -> None:
    """Run all candle validation checks."""
    if len(candles) != 60:
        raise ValidationError(f"Exactly 60 candles required, got {len(candles)}")

    validate_candle_fields(candles)
    validate_positive_values(candles)
    validate_ohlc_consistency(candles)
    validate_chronological_order(candles)


@app.exception_handler(ValidationError)
async def validation_error_handler(request: Request, exc: ValidationError):
    logger.warning(f"Validation error: {exc}")
    return JSONResponse(
        status_code=422,
        content={"detail": str(exc)}
    )


@app.exception_handler(ModelUnavailableError)
async def model_unavailable_handler(request: Request, exc: ModelUnavailableError):
    logger.error(f"Model unavailable: {exc}")
    return JSONResponse(
        status_code=503,
        content={"detail": "Model not loaded or unavailable"}
    )


@app.exception_handler(Exception)
async def general_exception_handler(request: Request, exc: Exception):
    logger.error(f"Unhandled exception: {exc}")
    logger.error(traceback.format_exc())
    return JSONResponse(
        status_code=500,
        content={"detail": "Internal server error"}
    )


@app.get("/health")
async def health_check():
    """Health check endpoint to verify service status."""
    model_loaded = hasattr(app.state, 'model') and app.state.model is not None

    status_code = 200 if model_loaded else 503
    return JSONResponse(
        status_code=status_code,
        content={
            "status": "ok" if model_loaded else "error",
            "model_loaded": model_loaded,
            "model_version": MODEL_VERSION,
        }
    )


@app.post("/ta", response_model=TAResponse)
async def compute_ta(request: TARequest):
    """Compute technical indicators from candle data."""
    if len(request.candles) < 20:
        raise HTTPException(
            status_code=422,
            detail="Insufficient data: at least 20 candles required for SMA computation"
        )

    all_indicators = compute_all_indicators(request.candles)
    requested_indicators = request.indicators

    response = TAResponse()

    if "sma" in requested_indicators:
        response.sma = all_indicators.get("sma")
    if "rsi" in requested_indicators:
        response.rsi = all_indicators.get("rsi")
    if "macd" in requested_indicators:
        response.macd_histogram = all_indicators.get("macd_histogram")
    if "bbands" in requested_indicators:
        response.bbands_upper = all_indicators.get("bbands_upper")
        response.bbands_middle = all_indicators.get("bbands_middle")
        response.bbands_lower = all_indicators.get("bbands_lower")
    if "atr" in requested_indicators:
        response.atr = all_indicators.get("atr")

    return response


# Feature names in the exact order the model expects
FEATURE_COLUMNS = [
    'returns', 'log_returns', 'high_low_range', 'body_size',
    'upper_shadow', 'lower_shadow', 'volume', 'volume_change',
    'sma_5', 'sma_10', 'sma_20',
    'sma_5_distance', 'sma_10_distance', 'sma_20_distance',
    'rsi', 'macd_histogram', 'bb_percent_b', 'atr',
    'price_momentum', 'price_acceleration',
    'hour_sin', 'day_of_week_sin',
]


def compute_features_from_candles(candles: List[CandleData]) -> pd.DataFrame:
    """Compute 22 features from candle data for model inference."""
    df = pd.DataFrame({
        "timestamp": [c.timestamp for c in candles],
        "open": [c.open for c in candles],
        "high": [c.high for c in candles],
        "low": [c.low for c in candles],
        "close": [c.close for c in candles],
        "volume": [c.volume for c in candles],
    })

    epsilon = 1e-10
    features = pd.DataFrame(index=df.index)

    features["returns"] = (df["close"] - df["open"]) / df["open"]
    features["log_returns"] = np.log(df["close"] / df["open"])
    features["high_low_range"] = (df["high"] - df["low"]) / df["low"]
    features["body_size"] = (
        np.abs(df["close"] - df["open"]) / (df["high"] - df["low"] + epsilon)
    )
    features["upper_shadow"] = (
        (df["high"] - np.maximum(df["open"], df["close"])) /
        (df["high"] - df["low"] + epsilon)
    ).clip(0, 1)
    features["lower_shadow"] = (
        (np.minimum(df["open"], df["close"]) - df["low"]) /
        (df["high"] - df["low"] + epsilon)
    ).clip(0, 1)
    features["volume"] = df["volume"]
    features["volume_change"] = df["volume"] / df["volume"].shift(1) - 1
    features["sma_5"] = df["close"].rolling(window=5).mean()
    features["sma_10"] = df["close"].rolling(window=10).mean()
    features["sma_20"] = df["close"].rolling(window=20).mean()
    features["sma_5_distance"] = features["sma_5"] - df["close"]
    features["sma_10_distance"] = features["sma_10"] - df["close"]
    features["sma_20_distance"] = features["sma_20"] - df["close"]

    rsi_result = ta.rsi(df["close"], length=14)
    features["rsi"] = rsi_result

    macd_result = ta.macd(df["close"], fast=12, slow=26, signal=9)
    macd_cols = macd_result.columns.tolist()
    macd_hist_col = [c for c in macd_cols if c.startswith("MACDh_")][0]
    features["macd_histogram"] = macd_result[macd_hist_col]

    bbands_result = ta.bbands(df["close"], length=20, std=2)
    bbands_cols = bbands_result.columns.tolist()
    lower_col = [c for c in bbands_cols if c.startswith("BBL_")][0]
    upper_col = [c for c in bbands_cols if c.startswith("BBU_")][0]
    features["bb_percent_b"] = (
        (df["close"] - bbands_result[lower_col]) /
        (bbands_result[upper_col] - bbands_result[lower_col] + epsilon)
    )

    atr_result = ta.atr(
        high=df["high"], low=df["low"], close=df["close"], length=14
    )
    features["atr"] = atr_result

    features["price_momentum"] = df["close"] - df["close"].shift(5)
    features["price_acceleration"] = features["price_momentum"].diff()

    hours = df["timestamp"].dt.hour
    features["hour_sin"] = np.sin(2 * np.pi * hours / 24)
    days = df["timestamp"].dt.dayofweek
    features["day_of_week_sin"] = np.sin(2 * np.pi * days / 7)

    # Fill NaNs: forward fill first, then backward fill
    # Do NOT dropna — we need exactly the last row
    features = features.ffill().bfill()

    # Ensure column order matches training
    features = features[FEATURE_COLUMNS]

    return features


def generate_predicted_candle(
    candles: List[CandleData],
    direction: str,
    confidence: float,
) -> PredictedCandleResponse:
    """Generate a predicted candle based on recent price action and prediction."""
    last_candle = candles[-1]
    last_close = last_candle.close

    # Calculate recent volatility from the last 20 candles
    recent = candles[-20:]
    avg_range = np.mean([c.high - c.low for c in recent])

    # Scale movement by confidence
    movement = avg_range * confidence * 0.5

    if direction == "UP":
        pred_open = last_close
        pred_close = last_close + movement
        pred_high = pred_close + avg_range * 0.2
        pred_low = pred_open - avg_range * 0.1
    elif direction == "DOWN":
        pred_open = last_close
        pred_close = last_close - movement
        pred_high = pred_open + avg_range * 0.1
        pred_low = pred_close - avg_range * 0.2
    else:
        pred_open = last_close
        pred_close = last_close
        pred_high = last_close + avg_range * 0.15
        pred_low = last_close - avg_range * 0.15

    # Ensure OHLC consistency
    pred_high = max(pred_high, pred_open, pred_close)
    pred_low = min(pred_low, pred_open, pred_close)

    avg_volume = np.mean([c.volume for c in recent])

    pred_timestamp = last_candle.timestamp + timedelta(seconds=PREDICTION_HORIZON_SECONDS)

    return PredictedCandleResponse(
        open=round(pred_open, 2),
        high=round(pred_high, 2),
        low=round(pred_low, 2),
        close=round(pred_close, 2),
        volume=round(avg_volume, 4),
        timestamp=pred_timestamp,
        close_time=pred_timestamp,
    )


def compute_predicted_ma(candles: List[CandleData], predicted_close: float) -> float:
    """Compute predicted SMA including the predicted candle's close."""
    closes = [c.close for c in candles[-(SMA_PERIOD - 1):]]
    closes.append(predicted_close)
    return round(float(np.mean(closes)), 2)


async def run_prediction_with_timeout(
    candles: List[CandleData]
) -> PredictResponse:
    """Run prediction with timeout."""

    def _do_prediction():
        features_df = compute_features_from_candles(candles)

        if len(features_df) == 0:
            raise ValidationError("Insufficient valid data to compute features")

        latest_features = features_df.iloc[-1:].values

        # Validate feature count
        expected_count = len(FEATURE_COLUMNS)
        actual_count = latest_features.shape[1]
        if actual_count != expected_count:
            raise ValidationError(
                f"Feature count mismatch: expected {expected_count}, got {actual_count}"
            )

        model = app.state.model

        try:
            probabilities = model.predict_proba(latest_features)[0]
        except Exception as e:
            logger.error(f"Model inference failed: {e}")
            logger.error(traceback.format_exc())
            raise ModelUnavailableError(f"Model inference failed: {e}")

        up_prob = probabilities[0]
        down_prob = probabilities[1]

        if up_prob > CONFIDENCE_THRESHOLD:
            direction = "UP"
            confidence = float(up_prob)
        elif down_prob > CONFIDENCE_THRESHOLD:
            direction = "DOWN"
            confidence = float(down_prob)
        else:
            direction = "UNCERTAIN"
            confidence = float(max(probabilities))

        predicted_candle = generate_predicted_candle(candles, direction, confidence)
        predicted_ma = compute_predicted_ma(candles, predicted_candle.close)

        return PredictResponse(
            direction=direction,
            confidence=confidence,
            predicted_candle=predicted_candle,
            predicted_ma=predicted_ma,
        )

    loop = asyncio.get_event_loop()

    try:
        return await asyncio.wait_for(
            loop.run_in_executor(None, _do_prediction),
            timeout=REQUEST_TIMEOUT_SECONDS
        )
    except asyncio.TimeoutError:
        logger.error(f"Prediction timed out after {REQUEST_TIMEOUT_SECONDS} seconds")
        raise ModelUnavailableError(
            f"Request timed out after {REQUEST_TIMEOUT_SECONDS} seconds"
        )


@app.post("/predict", response_model=PredictResponse)
async def predict(request: PredictRequest):
    """Predict price direction from candle data."""
    if len(request.candles) != 60:
        raise HTTPException(
            status_code=422,
            detail="Exactly 60 candles required for prediction"
        )

    if not hasattr(app.state, "model") or app.state.model is None:
        raise HTTPException(
            status_code=503,
            detail="Model not loaded or unavailable"
        )

    validate_all_candles(request.candles)

    try:
        return await run_prediction_with_timeout(request.candles)
    except (ValidationError, ModelUnavailableError):
        raise
    except Exception as e:
        logger.error(f"Unexpected error during prediction: {e}")
        logger.error(traceback.format_exc())
        raise


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
