"""
Technical Analysis module using pandas-ta-classic.

Provides functions to compute technical indicators from candlestick data.
"""

from typing import Dict, List, Optional

import pandas as pd
import pandas_ta as ta

from app.schemas import CandleData


def compute_sma(candles: List[CandleData], period: int = 20) -> Optional[float]:
    """
    Compute Simple Moving Average (SMA) from candle data.

    Args:
        candles: List of CandleData objects
        period: SMA period (default 20)

    Returns:
        Most recent SMA value, or None if insufficient data
    """
    if len(candles) < period:
        return None

    # Extract close prices
    closes = [c.close for c in candles]
    df = pd.DataFrame({"close": closes})

    # Compute SMA using pandas-ta-classic
    sma_series = ta.sma(df["close"], length=period)

    # Return the most recent non-null SMA value
    return float(sma_series.dropna().iloc[-1])


def compute_all_indicators(candles: List[CandleData]) -> Dict[str, Optional[float]]:
    """
    Compute multiple technical indicators from candle data.

    Computes:
    - SMA(20): Simple Moving Average with period 20
    - RSI(14): Relative Strength Index with period 14
    - MACD(12,26,9): Moving Average Convergence Divergence
    - BBANDS(20,2): Bollinger Bands
    - ATR(14): Average True Range

    Args:
        candles: List of CandleData objects

    Returns:
        Dictionary with indicator names as keys and computed values as values.
        Values may be None if insufficient data for that indicator.
    """
    if not candles:
        return {
            "sma": None,
            "rsi": None,
            "macd_histogram": None,
            "bbands_upper": None,
            "bbands_middle": None,
            "bbands_lower": None,
            "atr": None,
        }

    # Create DataFrame from candle data
    df = pd.DataFrame({
        "open": [c.open for c in candles],
        "high": [c.high for c in candles],
        "low": [c.low for c in candles],
        "close": [c.close for c in candles],
        "volume": [c.volume for c in candles],
    })

    result: Dict[str, Optional[float]] = {}

    # SMA(20) - requires 20 candles
    if len(candles) >= 20:
        sma_series = ta.sma(df["close"], length=20)
        result["sma"] = float(sma_series.dropna().iloc[-1])
    else:
        result["sma"] = None

    # RSI(14) - requires 14 candles
    if len(candles) >= 14:
        rsi_series = ta.rsi(df["close"], length=14)
        result["rsi"] = float(rsi_series.dropna().iloc[-1])
    else:
        result["rsi"] = None

    # MACD(12,26,9) - requires 35 candles (26 for MACD + 9 for signal)
    if len(candles) >= 35:
        macd_result = ta.macd(df["close"], fast=12, slow=26, signal=9)
        if macd_result is not None:
            macd_cols = macd_result.columns.tolist()
            histogram_col = next((c for c in macd_cols if c.startswith("MACDh_")), None)
            if histogram_col and histogram_col in macd_result.columns:
                hist_series = macd_result[histogram_col].dropna()
                if len(hist_series) > 0:
                    result["macd_histogram"] = float(hist_series.iloc[-1])
                else:
                    result["macd_histogram"] = None
            else:
                result["macd_histogram"] = None
        else:
            result["macd_histogram"] = None
    else:
        result["macd_histogram"] = None

    # Bollinger Bands(20,2) - requires 20 candles
    if len(candles) >= 20:
        bbands_result = ta.bbands(df["close"], length=20, std=2)
        if bbands_result is not None:
            # BBANDS returns DataFrame with columns like: BBL_20_2.0, BBM_20_2.0, BBU_20_2.0
            lower_col = next((c for c in bbands_result.columns if "BBL" in c), None)
            middle_col = next((c for c in bbands_result.columns if "BBM" in c), None)
            upper_col = next((c for c in bbands_result.columns if "BBU" in c), None)

            if lower_col:
                result["bbands_lower"] = float(bbands_result[lower_col].dropna().iloc[-1])
            else:
                result["bbands_lower"] = None

            if middle_col:
                result["bbands_middle"] = float(bbands_result[middle_col].dropna().iloc[-1])
            else:
                result["bbands_middle"] = None

            if upper_col:
                result["bbands_upper"] = float(bbands_result[upper_col].dropna().iloc[-1])
            else:
                result["bbands_upper"] = None
        else:
            result["bbands_lower"] = None
            result["bbands_middle"] = None
            result["bbands_upper"] = None
    else:
        result["bbands_lower"] = None
        result["bbands_middle"] = None
        result["bbands_upper"] = None

    # ATR(14) - requires high, low, close and 14 candles
    if len(candles) >= 14:
        atr_series = ta.atr(df["high"], df["low"], df["close"], length=14)
        if atr_series is not None:
            result["atr"] = float(atr_series.dropna().iloc[-1])
        else:
            result["atr"] = None
    else:
        result["atr"] = None

    return result
