from datetime import datetime
from typing import Optional

from pydantic import BaseModel


class CandleData(BaseModel):
    """Represents a single candlestick data point."""
    open: float
    high: float
    low: float
    close: float
    volume: float
    timestamp: datetime


class PredictionInput(BaseModel):
    """Input for prediction model."""
    candles: list[CandleData]
    features: Optional[dict] = None


class PredictionOutput(BaseModel):
    """Output from prediction model."""
    direction: str  # UP, DOWN, UNCERTAIN
    confidence: float
    predicted_candle: CandleData
    predicted_ma: float