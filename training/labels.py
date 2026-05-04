"""Label creation and walk-forward validation for model training.

Creates UP/DOWN/UNCERTAIN labels based on 30-second future price direction
and implements walk-forward validation splits for time-series data.
"""

import argparse
import sys
from pathlib import Path
from typing import List, Tuple

import numpy as np
import pandas as pd
from sklearn.model_selection import TimeSeriesSplit


def load_features_and_prices(features_path: str, prices_path: str) -> Tuple[pd.DataFrame, pd.Series]:
    """Load features from parquet and close prices from CSV.
    
    Args:
        features_path: Path to features parquet file
        prices_path: Path to OHLCV CSV file with close prices
        
    Returns:
        Tuple of (features DataFrame, close prices Series)
        
    Raises:
        FileNotFoundError: If either input file doesn't exist
    """
    features_file = Path(features_path)
    prices_file = Path(prices_path)
    
    if not features_file.exists():
        raise FileNotFoundError(f"Features file not found: {features_path}")
    if not prices_file.exists():
        raise FileNotFoundError(f"Prices file not found: {prices_path}")
    
    features = pd.read_parquet(features_path)
    prices_df = pd.read_csv(prices_path)
    close_prices = prices_df['close']
    
    return features, close_prices


def create_labels(
    features: pd.DataFrame,
    close_prices: pd.Series,
    lookahead: int = 30,
    up_threshold: float = 0.001,
    down_threshold: float = 0.001
) -> pd.DataFrame:
    """Create UP/DOWN/UNCERTAIN labels based on future price direction.
    
    Labels are created based on close[t+lookahead] vs close[t]:
    - UP: close[t+lookahead] > close[t] * (1 + up_threshold)
    - DOWN: close[t+lookahead] < close[t] * (1 - down_threshold)
    - UNCERTAIN: Between thresholds
    
    Args:
        features: DataFrame with engineered features
        close_prices: Series of close prices
        lookahead: Number of periods to look ahead (default: 30)
        up_threshold: Threshold for UP label (default: 0.001 = 0.1%)
        down_threshold: Threshold for DOWN label (default: 0.001 = 0.1%)
        
    Returns:
        DataFrame with features and label column
    """
    result = features.copy()
    
    future_prices = close_prices.shift(-lookahead)
    price_change = future_prices / close_prices - 1
    
    conditions = [
        price_change > up_threshold,
        price_change < -down_threshold
    ]
    choices = ['UP', 'DOWN']
    
    result['label'] = np.select(conditions, choices, default='UNCERTAIN')
    result = result.iloc[:-lookahead].copy()
    
    return result


def verify_no_lookahead_bias(features: pd.DataFrame, close_prices: pd.Series) -> None:
    """Verify that features don't leak future information."""
    future_returns = close_prices.pct_change().shift(-1)
    time_features = {'hour_sin', 'day_of_week_sin'}
    
    for col in features.columns:
        if col in time_features:
            continue
            
        corr = features[col].corr(future_returns)
        
        if abs(corr) > 0.95:
            raise AssertionError(
                f"Feature '{col}' has suspiciously high correlation ({corr:.3f}) "
                f"with future returns - possible lookahead bias"
            )


def create_walk_forward_splits(
    data: pd.DataFrame,
    n_splits: int = 5
) -> List[Tuple[np.ndarray, np.ndarray]]:
    """Create walk-forward validation splits using TimeSeriesSplit.
    
    Each fold: train on past, test on future. No shuffling!
    
    Args:
        data: DataFrame with labeled data
        n_splits: Number of splits (default: 5)
        
    Returns:
        List of (train_indices, test_indices) tuples
    """
    tscv = TimeSeriesSplit(n_splits=n_splits)
    
    splits = []
    for train_idx, test_idx in tscv.split(data):
        splits.append((train_idx, test_idx))
    
    return splits


def save_labeled_data(df: pd.DataFrame, output_path: str) -> None:
    """Save labeled data to parquet file.
    
    Args:
        df: DataFrame with features and label column
        output_path: Path to output parquet file
    """
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    df.to_parquet(output_path, index=False)


def main():
    """Main entry point for CLI."""
    parser = argparse.ArgumentParser(
        description='Create labels and walk-forward validation splits for model training'
    )
    parser.add_argument(
        '--input',
        type=str,
        default='training/data/features.parquet',
        help='Input features parquet file path (default: training/data/features.parquet)'
    )
    parser.add_argument(
        '--prices',
        type=str,
        default='training/data/btcusdt_1s.csv',
        help='Input prices CSV file path (default: training/data/btcusdt_1s.csv)'
    )
    parser.add_argument(
        '--output',
        type=str,
        default='training/data/labeled.parquet',
        help='Output parquet file path (default: training/data/labeled.parquet)'
    )
    parser.add_argument(
        '--lookahead',
        type=int,
        default=30,
        help='Lookahead period for labels in seconds (default: 30)'
    )
    parser.add_argument(
        '--threshold',
        type=float,
        default=0.001,
        help='Price change threshold for UP/DOWN labels (default: 0.001 = 0.1%%)'
    )
    
    args = parser.parse_args()
    
    try:
        print(f"Loading features from {args.input}...")
        print(f"Loading prices from {args.prices}...")
        features, close_prices = load_features_and_prices(args.input, args.prices)
        print(f"Loaded {len(features)} rows of features")
        
        print(f"Creating labels with {args.lookahead}s lookahead...")
        labeled_df = create_labels(
            features,
            close_prices,
            lookahead=args.lookahead,
            up_threshold=args.threshold,
            down_threshold=args.threshold
        )
        print(f"Created {len(labeled_df)} labeled rows")
        
        # Show label distribution
        label_counts = labeled_df['label'].value_counts()
        print("\nLabel distribution:")
        for label, count in label_counts.items():
            pct = count / len(labeled_df) * 100
            print(f"  {label}: {count} ({pct:.1f}%)")
        
        print("\nVerifying no lookahead bias...")
        verify_no_lookahead_bias(features, close_prices)
        print("No lookahead bias detected")

        print("\nCreating walk-forward validation splits...")
        splits = create_walk_forward_splits(labeled_df, n_splits=5)
        print(f"Created {len(splits)} splits:")
        for i, (train_idx, test_idx) in enumerate(splits):
            print(f"  Split {i+1}: train={len(train_idx)}, test={len(test_idx)}")

        print(f"\nSaving labeled data to {args.output}...")
        save_labeled_data(labeled_df, args.output)
        print("Done!")
        
        return 0
        
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except AssertionError as e:
        print(f"Validation error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Error processing labels: {e}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
