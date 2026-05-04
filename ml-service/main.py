"""
BTC Predictor ML Service

FastAPI-based ML inference service for Bitcoin price prediction.
"""

import logging
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Dict, List, Optional

import joblib
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from pydantic import BaseModel

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


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
