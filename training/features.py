"""Feature engineering pipeline for LightGBM model.

Transforms raw OHLCV data into 22 engineered features for model training.
"""

import argparse
import sys
from pathlib import Path
from typing import List

import numpy as np
import pandas as pd
import pandas_ta_classic as ta

# List of expected features for validation
EXPECTED_FEATURES: List[str] = [
    'returns',
    'log_returns',
    'high_low_range',
    'body_size',
    'upper_shadow',
    'lower_shadow',
    'volume',
    'volume_change',
    'sma_5',
    'sma_10',
    'sma_20',
    'sma_5_distance',
    'sma_10_distance',
    'sma_20_distance',
    'rsi',
    'macd_histogram',
    'bb_percent_b',
    'atr',
    'price_momentum',
    'price_acceleration',
    'hour_sin',
    'day_of_week_sin',
]


def load_data(input_path: str) -> pd.DataFrame:
    """Load OHLCV data from CSV file.
    
    Args:
        input_path: Path to input CSV file
        
    Returns:
        DataFrame with timestamp, open, high, low, close, volume columns
        
    Raises:
        FileNotFoundError: If input file doesn't exist
    """
    input_file = Path(input_path)
    if not input_file.exists():
        raise FileNotFoundError(f"Input file not found: {input_path}")
    
    df = pd.read_csv(input_path)
    df['timestamp'] = pd.to_datetime(df['timestamp'], utc=True)
    return df


def calculate_features(df: pd.DataFrame) -> pd.DataFrame:
    """Calculate all 22 features from OHLCV data.
    
    Args:
        df: DataFrame with OHLCV columns
        
    Returns:
        DataFrame with 22 engineered features
    """
    features = pd.DataFrame(index=df.index)
    
    epsilon = 1e-10
    
    # 1. Returns (close - open) / open
    features['returns'] = (df['close'] - df['open']) / df['open']
    
    # 2. Log returns
    features['log_returns'] = np.log(df['close'] / df['open'])
    
    # 3. High-Low range normalized
    features['high_low_range'] = (df['high'] - df['low']) / df['low']
    
    # 4. Body size (abs(close - open) / (high - low + epsilon))
    features['body_size'] = (
        np.abs(df['close'] - df['open']) / (df['high'] - df['low'] + epsilon)
    )
    
    # 5. Upper shadow (high - max(open, close)) / (high - low + epsilon)
    features['upper_shadow'] = (
        (df['high'] - np.maximum(df['open'], df['close'])) /
        (df['high'] - df['low'] + epsilon)
    ).clip(0, 1)

    # 6. Lower shadow (min(open, close) - low) / (high - low + epsilon)
    features['lower_shadow'] = (
        (np.minimum(df['open'], df['close']) - df['low']) /
        (df['high'] - df['low'] + epsilon)
    ).clip(0, 1)
    
    # 7. Volume
    features['volume'] = df['volume']
    
    # 8. Volume change (volume / volume.shift(1) - 1)
    features['volume_change'] = df['volume'] / df['volume'].shift(1) - 1
    
    # 9-11. SMA(5), SMA(10), SMA(20) of close
    features['sma_5'] = df['close'].rolling(window=5).mean()
    features['sma_10'] = df['close'].rolling(window=10).mean()
    features['sma_20'] = df['close'].rolling(window=20).mean()
    
    # 12-14. SMA distances
    features['sma_5_distance'] = features['sma_5'] - df['close']
    features['sma_10_distance'] = features['sma_10'] - df['close']
    features['sma_20_distance'] = features['sma_20'] - df['close']
    
    # 15. RSI(14)
    rsi_result = ta.rsi(df['close'], length=14)
    features['rsi'] = rsi_result
    
    # 16. MACD histogram (12,26,9)
    macd_result = ta.macd(
        df['close'],
        fast=12,
        slow=26,
        signal=9
    )
    features['macd_histogram'] = macd_result['MACDh_12_26_9']
    
    # 17. Bollinger Band %B (20,2)
    bbands_result = ta.bbands(df['close'], length=20, std=2)
    lower_col = f"BBL_20_2.0"
    upper_col = f"BBU_20_2.0"
    middle_col = f"BBM_20_2.0"
    
    features['bb_percent_b'] = (
        (df['close'] - bbands_result[lower_col]) /
        (bbands_result[upper_col] - bbands_result[lower_col] + epsilon)
    )
    
    # 18. ATR(14)
    atr_result = ta.atr(
        high=df['high'],
        low=df['low'],
        close=df['close'],
        length=14
    )
    features['atr'] = atr_result
    
    # 19. Price momentum (close - close.shift(5))
    features['price_momentum'] = df['close'] - df['close'].shift(5)
    
    # 20. Price acceleration (momentum.diff())
    features['price_acceleration'] = features['price_momentum'].diff()
    
    # 21. Hour of day (cyclical encoding)
    hours = df['timestamp'].dt.hour
    features['hour_sin'] = np.sin(2 * np.pi * hours / 24)
    
    # 22. Day of week (cyclical encoding: Monday=0, Sunday=6)
    days = df['timestamp'].dt.dayofweek
    features['day_of_week_sin'] = np.sin(2 * np.pi * days / 7)
    
    return features


def handle_missing_values(df: pd.DataFrame) -> pd.DataFrame:
    """Handle NaN values in features.
    
    Strategy: forward fill, then backward fill, then drop any remaining NaNs.
    
    Args:
        df: DataFrame with potential NaN values
        
    Returns:
        DataFrame with NaN values handled
    """
    # Forward fill
    df = df.ffill()
    
    # Backward fill for any remaining NaNs at the beginning
    df = df.bfill()
    
    # Drop any rows that still have NaN values
    df = df.dropna()
    
    return df


def save_features(df: pd.DataFrame, output_path: str) -> None:
    """Save features to parquet file.
    
    Args:
        df: DataFrame with features
        output_path: Path to output parquet file
    """
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    df.to_parquet(output_path, index=False)


def main():
    """Main entry point for CLI."""
    parser = argparse.ArgumentParser(
        description='Transform OHLCV data into engineered features for LightGBM'
    )
    parser.add_argument(
        '--input',
        type=str,
        default='training/data/btcusdt_1s.csv',
        help='Input CSV file path (default: training/data/btcusdt_1s.csv)'
    )
    parser.add_argument(
        '--output',
        type=str,
        default='training/data/features.parquet',
        help='Output parquet file path (default: training/data/features.parquet)'
    )
    
    args = parser.parse_args()
    
    try:
        print(f"Loading data from {args.input}...")
        df = load_data(args.input)
        print(f"Loaded {len(df)} rows")
        
        print("Calculating features...")
        features = calculate_features(df)
        print(f"Calculated {len(features.columns)} features")
        
        print("Handling missing values...")
        features = handle_missing_values(features)
        print(f"Final dataset: {len(features)} rows, {len(features.columns)} features")
        
        print(f"Saving to {args.output}...")
        save_features(features, args.output)
        print("Done!")
        
        return 0
        
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Error processing features: {e}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
