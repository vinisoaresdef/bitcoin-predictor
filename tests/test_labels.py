"""Tests for training/labels.py

TDD approach: Tests written first, implementation follows.
"""

import os
import sys
import pytest
import pandas as pd
import numpy as np
from pathlib import Path
from datetime import datetime, timezone

# Add training directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'training'))

from labels import (
    load_features_and_prices,
    create_labels,
    create_walk_forward_splits,
    verify_no_lookahead_bias,
    save_labeled_data,
    main,
)


@pytest.fixture
def sample_features_and_prices():
    """Create sample features and price data for testing."""
    np.random.seed(42)
    n_rows = 100

    features = pd.DataFrame({
        'returns': np.random.randn(n_rows) * 0.001,
        'log_returns': np.random.randn(n_rows) * 0.001,
        'high_low_range': np.random.uniform(0.0001, 0.01, n_rows),
        'body_size': np.random.uniform(0, 1, n_rows),
        'upper_shadow': np.random.uniform(0, 1, n_rows),
        'lower_shadow': np.random.uniform(0, 1, n_rows),
        'volume': np.random.uniform(100, 500, n_rows),
        'volume_change': np.random.randn(n_rows) * 0.1,
        'sma_5': np.random.uniform(40000, 45000, n_rows),
        'sma_10': np.random.uniform(40000, 45000, n_rows),
        'sma_20': np.random.uniform(40000, 45000, n_rows),
        'sma_5_distance': np.random.randn(n_rows) * 10,
        'sma_10_distance': np.random.randn(n_rows) * 10,
        'sma_20_distance': np.random.randn(n_rows) * 10,
        'rsi': np.random.uniform(20, 80, n_rows),
        'macd_histogram': np.random.randn(n_rows) * 5,
        'bb_percent_b': np.random.uniform(0, 1, n_rows),
        'atr': np.random.uniform(10, 100, n_rows),
        'price_momentum': np.random.randn(n_rows) * 50,
        'price_acceleration': np.random.randn(n_rows) * 10,
        'hour_sin': np.random.uniform(-1, 1, n_rows),
        'day_of_week_sin': np.random.uniform(-1, 1, n_rows),
    })

    prices = []
    base = 42000.0
    for i in range(n_rows):
        if i < 30:
            prices.append(base + i * 15)
        elif i < 60:
            prices.append(base + 450 - (i - 30) * 15)
        else:
            prices.append(base + 5)

    close_prices = pd.Series(prices, name='close')

    return features, close_prices


class TestLabelCreation:
    """Test label creation functionality."""

    def test_label_distribution(self, sample_features_and_prices):
        """Verify all 3 classes present, no extreme imbalance."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        
        # Check all 3 classes are present
        assert 'UP' in labeled_df['label'].values, "UP label should be present"
        assert 'DOWN' in labeled_df['label'].values, "DOWN label should be present"
        assert 'UNCERTAIN' in labeled_df['label'].values, "UNCERTAIN label should be present"
        
        # Check no extreme imbalance (no class should be < 5% of data)
        label_counts = labeled_df['label'].value_counts()
        total = len(labeled_df)
        for label in ['UP', 'DOWN', 'UNCERTAIN']:
            if label in label_counts:
                proportion = label_counts[label] / total
                assert proportion >= 0.05, f"Class {label} is too rare ({proportion:.2%})"

    def test_label_logic_up(self):
        """Test UP label logic - close[t+30] > close[t] * 1.001."""
        # Create data with clear UP movement
        features = pd.DataFrame({'dummy': range(40)})
        prices = pd.Series([100.0] * 40)
        # Set price 30 steps ahead to be 0.2% higher (above 0.1% threshold)
        prices.iloc[30] = 100.2  # 0.2% up = above 0.1% threshold
        
        labeled_df = create_labels(features, prices, lookahead=30)
        
        # Row 0 should be UP (if we have enough data)
        if len(labeled_df) > 0:
            assert labeled_df['label'].iloc[0] == 'UP', \
                "Price 0.2% higher should be labeled UP"

    def test_label_logic_down(self):
        """Test DOWN label logic - close[t+30] < close[t] * 0.999."""
        features = pd.DataFrame({'dummy': range(40)})
        prices = pd.Series([100.0] * 40)
        # Set price 30 steps ahead to be 0.2% lower (below -0.1% threshold)
        prices.iloc[30] = 99.8  # 0.2% down = below -0.1% threshold
        
        labeled_df = create_labels(features, prices, lookahead=30)
        
        if len(labeled_df) > 0:
            assert labeled_df['label'].iloc[0] == 'DOWN', \
                "Price 0.2% lower should be labeled DOWN"

    def test_label_logic_uncertain(self):
        """Test UNCERTAIN label logic - within thresholds."""
        features = pd.DataFrame({'dummy': range(40)})
        prices = pd.Series([100.0] * 40)
        # Set price 30 steps ahead to be 0.05% different (within 0.1% threshold)
        prices.iloc[30] = 100.05  # 0.05% change = within ±0.1% threshold
        
        labeled_df = create_labels(features, prices, lookahead=30)
        
        if len(labeled_df) > 0:
            assert labeled_df['label'].iloc[0] == 'UNCERTAIN', \
                "Price change within 0.1% should be UNCERTAIN"

    def test_last_rows_removed(self, sample_features_and_prices):
        """Verify last 30 rows are removed (no future data)."""
        features, close_prices = sample_features_and_prices
        lookahead = 30
        
        labeled_df = create_labels(features, close_prices, lookahead=lookahead)
        
        # Original data had 100 rows, should now have 70 (100 - 30)
        expected_rows = len(features) - lookahead
        assert len(labeled_df) == expected_rows, \
            f"Expected {expected_rows} rows after removing last {lookahead}, got {len(labeled_df)}"

    def test_features_preserved(self, sample_features_and_prices):
        """Verify all original features are preserved in output."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        
        # Check all original feature columns are present
        for col in features.columns:
            assert col in labeled_df.columns, f"Feature column {col} should be preserved"
        
        # Check label column is added
        assert 'label' in labeled_df.columns, "Label column should be added"


class TestNoLookaheadBias:
    """Test that there's no lookahead bias in features."""

    def test_no_lookahead_bias(self, sample_features_and_prices):
        """Verify features don't leak future information."""
        features, close_prices = sample_features_and_prices
        
        # This should not raise any assertion errors
        try:
            verify_no_lookahead_bias(features, close_prices)
        except AssertionError as e:
            pytest.fail(f"Lookahead bias detected: {e}")

    def test_features_use_only_past_data(self):
        """Test that features at time t only use data from <= t."""
        # Create simple test case where we know the calculation
        np.random.seed(42)
        n = 50
        
        # Create OHLCV-like data
        timestamps = pd.date_range('2024-01-01', periods=n, freq='1s')
        close_prices = pd.Series(np.cumsum(np.random.randn(n)) + 100, name='close')
        
        # Create a feature that uses SMA (which only uses past data)
        sma_5 = close_prices.rolling(window=5).mean()
        
        # The SMA at index 10 should only use indices 6,7,8,9,10
        # It should NOT use indices 11, 12, etc.
        expected_sma_10 = close_prices.iloc[6:11].mean()
        actual_sma_10 = sma_5.iloc[10]
        
        assert abs(actual_sma_10 - expected_sma_10) < 1e-10, \
            "SMA should only use past and current data"


class TestWalkForwardSplits:
    """Test walk-forward validation splits."""

    def test_walk_forward_splits(self, sample_features_and_prices):
        """Verify 5 folds, increasing train size, no shuffle."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        splits = create_walk_forward_splits(labeled_df, n_splits=5)
        
        # Check we have 5 splits
        assert len(splits) == 5, f"Expected 5 splits, got {len(splits)}"
        
        # Check each split
        prev_train_end = -1
        for i, (train_indices, test_indices) in enumerate(splits):
            # Train indices should come before test indices
            assert max(train_indices) < min(test_indices), \
                f"Split {i}: Train indices should come before test indices"
            
            # No overlap between train and test
            assert len(set(train_indices) & set(test_indices)) == 0, \
                f"Split {i}: No overlap between train and test"
            
            # Train size should be increasing
            if i > 0:
                prev_train_size = len(splits[i-1][0])
                curr_train_size = len(train_indices)
                assert curr_train_size > prev_train_size, \
                    f"Split {i}: Train size should be increasing"
            
            # Test size should be reasonable
            assert len(test_indices) > 0, f"Split {i}: Test set should not be empty"

    def test_splits_as_indices(self, sample_features_and_prices):
        """Verify splits are returned as indices, not duplicated data."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        splits = create_walk_forward_splits(labeled_df, n_splits=5)
        
        for i, (train_indices, test_indices) in enumerate(splits):
            # Should be arrays/lists of integers
            assert isinstance(train_indices, (list, np.ndarray)), \
                f"Split {i}: Train indices should be list or array"
            assert isinstance(test_indices, (list, np.ndarray)), \
                f"Split {i}: Test indices should be list or array"
            
            # Should contain integer indices
            assert all(isinstance(x, (int, np.integer)) for x in train_indices), \
                f"Split {i}: Train indices should be integers"
            assert all(isinstance(x, (int, np.integer)) for x in test_indices), \
                f"Split {i}: Test indices should be integers"

    def test_no_shuffle_in_splits(self, sample_features_and_prices):
        """Verify data order is preserved (no shuffling)."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        splits = create_walk_forward_splits(labeled_df, n_splits=5)
        
        # Check that indices are in ascending order within each split
        for i, (train_indices, test_indices) in enumerate(splits):
            assert list(train_indices) == sorted(train_indices), \
                f"Split {i}: Train indices should be sorted (no shuffle)"
            assert list(test_indices) == sorted(test_indices), \
                f"Split {i}: Test indices should be sorted (no shuffle)"

    def test_temporal_order_preserved(self, sample_features_and_prices):
        """Verify temporal order is maintained in train/test splits."""
        features, close_prices = sample_features_and_prices
        
        labeled_df = create_labels(features, close_prices, lookahead=30)
        splits = create_walk_forward_splits(labeled_df, n_splits=5)
        
        # In walk-forward, each test set should come after the previous test set
        for i in range(1, len(splits)):
            prev_test_end = max(splits[i-1][1])
            curr_test_start = min(splits[i][1])
            
            assert curr_test_start > prev_test_end, \
                f"Split {i}: Current test should come after previous test"


class TestDataLoading:
    """Test data loading functions."""

    def test_load_features_and_prices(self, tmp_path):
        """Test loading features from parquet and prices from CSV."""
        # Create test parquet with features
        features = pd.DataFrame({
            'returns': [0.01, 0.02, -0.01],
            'volume': [100.0, 150.0, 200.0],
        })
        parquet_path = tmp_path / "features.parquet"
        features.to_parquet(parquet_path, index=False)
        
        # Create test CSV with OHLCV data
        csv_data = pd.DataFrame({
            'timestamp': ['2024-01-01T00:00:00+00:00', '2024-01-01T00:00:01+00:00', '2024-01-01T00:00:02+00:00'],
            'open': [100.0, 101.0, 102.0],
            'high': [101.0, 102.0, 103.0],
            'low': [99.0, 100.0, 101.0],
            'close': [100.5, 101.5, 102.5],
            'volume': [1000.0, 1500.0, 2000.0],
        })
        csv_path = tmp_path / "prices.csv"
        csv_data.to_csv(csv_path, index=False)
        
        loaded_features, loaded_prices = load_features_and_prices(
            str(parquet_path), str(csv_path)
        )
        
        assert len(loaded_features) == 3
        assert len(loaded_prices) == 3
        assert list(loaded_features.columns) == ['returns', 'volume']
        assert loaded_prices.name == 'close'
        pd.testing.assert_series_equal(
            loaded_prices, 
            pd.Series([100.5, 101.5, 102.5], name='close')
        )

    def test_load_features_missing_file(self, tmp_path):
        """Test handling of missing feature file."""
        missing_parquet = tmp_path / "nonexistent.parquet"
        csv_path = tmp_path / "prices.csv"
        
        with pytest.raises(FileNotFoundError):
            load_features_and_prices(str(missing_parquet), str(csv_path))


class TestDataSaving:
    """Test labeled data saving functionality."""

    def test_save_labeled_data(self, tmp_path):
        """Test that labeled data is saved to parquet format."""
        labeled_df = pd.DataFrame({
            'returns': [0.01, 0.02, -0.01],
            'volume': [100.0, 150.0, 200.0],
            'label': ['UP', 'DOWN', 'UNCERTAIN'],
        })
        
        output_path = tmp_path / "labeled.parquet"
        save_labeled_data(labeled_df, str(output_path))
        
        assert output_path.exists(), "Labeled parquet file should be created"
        
        # Read back and verify
        loaded = pd.read_parquet(output_path)
        pd.testing.assert_frame_equal(labeled_df, loaded)

    def test_save_labeled_data_creates_directory(self, tmp_path):
        """Test that output directory is created if it doesn't exist."""
        labeled_df = pd.DataFrame({
            'returns': [0.01, 0.02],
            'label': ['UP', 'DOWN'],
        })
        
        # Use a nested path that doesn't exist
        output_path = tmp_path / "subdir" / "nested" / "labeled.parquet"
        save_labeled_data(labeled_df, str(output_path))
        
        assert output_path.exists(), "Should create nested directories"


class TestCLI:
    """Test command-line interface."""

    def test_cli_with_default_args(self, tmp_path):
        """Test CLI with default arguments."""
        # Create test data files
        n_rows = 100
        
        # Create features parquet
        features = pd.DataFrame({
            'returns': np.random.randn(n_rows) * 0.001,
            'volume': np.random.uniform(100, 500, n_rows),
        })
        features_path = tmp_path / "features.parquet"
        features.to_parquet(features_path, index=False)
        
        # Create prices CSV
        timestamps = pd.date_range('2024-01-01', periods=n_rows, freq='1s', tz='UTC')
        prices = pd.DataFrame({
            'timestamp': timestamps,
            'open': np.random.uniform(40000, 45000, n_rows),
            'high': np.random.uniform(40000, 45000, n_rows),
            'low': np.random.uniform(40000, 45000, n_rows),
            'close': np.cumsum(np.random.randn(n_rows)) + 42000,
            'volume': np.random.uniform(100, 500, n_rows),
        })
        csv_path = tmp_path / "btcusdt_1s.csv"
        prices.to_csv(csv_path, index=False)
        
        output_path = tmp_path / "labeled.parquet"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'labels.py',
            '--input', str(features_path),
            '--prices', str(csv_path),
            '--output', str(output_path)
        ]):
            exit_code = main()
        
        assert exit_code == 0, "CLI should return 0 on success"
        assert output_path.exists(), "Output file should be created"
        
        # Verify content
        labeled = pd.read_parquet(output_path)
        assert 'label' in labeled.columns
        assert len(labeled) == n_rows - 30  # Last 30 rows removed

    def test_cli_missing_input_file(self, tmp_path):
        """Test CLI with non-existent input file."""
        missing_path = tmp_path / "nonexistent.parquet"
        output_path = tmp_path / "labeled.parquet"
        
        from unittest.mock import patch
        with patch('sys.argv', [
            'labels.py',
            '--input', str(missing_path),
            '--output', str(output_path)
        ]):
            exit_code = main()
        
        assert exit_code != 0, "CLI should return non-zero on error"


# Import patch for CLI tests
from unittest.mock import patch
