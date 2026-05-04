"""
BTC Predictor ML Service

FastAPI-based ML inference service for Bitcoin price prediction.
"""

import logging
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Dict, List, Optional

import joblib
import numpy as np
import pandas as pd
import pandas_ta_classic as ta
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field

from app.schemas import CandleData
from ta import compute_all_indicators

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

MODEL_PATH = Path("models/model.joblib")
MODEL_VERSION = "1.0.0"


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
            elif hasattr(app.state.model, 'feature_name_'):
                feature_count = len(app.state.model.feature_name_)
                logger.info(f"Model has {feature_count} features: {list(app.state.model.feature_name_)}")
            else:
                logger.info("Model loaded (feature names not available)")
        else:
            logger.error(f"Model file not found at {MODEL_PATH}")
            app.state.model = None
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        app.state.model = None
    
    yield
    
    logger.info("Shutting down ML Service...")
    if hasattr(app.state, 'model') and app.state.model is not None:
        logger.info("Model cleanup complete")


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


class PredictRequest(BaseModel):
    candles: List[CandleData] = Field(..., min_length=60, max_length=60)


class PredictResponse(BaseModel):
    direction: str = Field(..., pattern="^(UP|DOWN|UNCERTAIN)$")
    confidence: float = Field(..., ge=0.0, le=1.0)
    probabilities: List[float] = Field(..., min_length=3, max_length=3)


@app.get("/health")
async def health_check():
    """Health check endpoint to verify service status."""
    model_loaded = hasattr(app.state, 'model') and app.state.model is not None
    
    if model_loaded:
        return JSONResponse(
            status_code=200,
            content={
                "status": "ok",
                "model_loaded": True,
                "model_version": MODEL_VERSION,
            }
        )
    else:
        return JSONResponse(
            status_code=503,
            content={
                "status": "error",
                "model_loaded": False,
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
    features["macd_histogram"] = macd_result["MACDh_12_26_9"]
    
    bbands_result = ta.bbands(df["close"], length=20, std=2)
    lower_col = "BBL_20_2.0"
    upper_col = "BBU_20_2.0"
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
    
    features = features.ffill().bfill().dropna()
    
    return features


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
    
    features_df = compute_features_from_candles(request.candles)
    
    if len(features_df) == 0:
        raise HTTPException(
            status_code=422,
            detail="Insufficient valid data to compute features"
        )
    
    latest_features = features_df.iloc[-1:].values
    
    model = app.state.model
    probabilities = model.predict_proba(latest_features)[0]
    
    up_prob = probabilities[0]
    down_prob = probabilities[1]
    threshold = 0.55
    
    if up_prob > threshold:
        direction = "UP"
        confidence = float(up_prob)
    elif down_prob > threshold:
        direction = "DOWN"
        confidence = float(down_prob)
    else:
        direction = "UNCERTAIN"
        confidence = float(max(up_prob, down_prob, probabilities[2]))
    
    return PredictResponse(
        direction=direction,
        confidence=confidence,
        probabilities=[float(p) for p in probabilities]
    )


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
