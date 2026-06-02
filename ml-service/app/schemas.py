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
