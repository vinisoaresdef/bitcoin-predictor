"""Tests for training/features.py

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

from features import (
    load_data,
    calculate_features,
    handle_missing_values,
    save_features,
    main,
    EXPECTED_FEATURES,
)


@pytest.fixture
def sample_ohlcv_data():
    """Create sample OHLCV data for testing."""
    # Create 50 rows of synthetic data (enough for all indicators)
    np.random.seed(42)
    dates = pd.date_range(start='2024-01-01', periods=50, freq='1s', tz='UTC')
    
    # Generate realistic price data with trend
    base_price = 42000.0
    prices = []
    for i in range(50):
        noise = np.random.randn() * 50
        trend = i * 2  # slight upward trend
        prices.append(base_price + trend + noise)
    
    df = pd.DataFrame({
        'timestamp': dates,
        'open': prices,
        'high': [p + abs(np.random.randn() * 30) + 10 for p in prices],
        'low': [p - abs(np.random.randn() * 30) - 10 for p in prices],
        'close': [p + np.random.randn() * 20 for p in prices],
        'volume': np.random.uniform(100, 500, 50),
    })
    
    return df


class TestFeatureCalculation:
    """Test feature calculation functions."""

    def test_all_22_features_present(self, sample_ohlcv_data):
        """Verify all 22 expected columns exist in output."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Check that all expected features are present
        missing = set(EXPECTED_FEATURES) - set(features_df.columns)
        extra = set(features_df.columns) - set(EXPECTED_FEATURES)
        
        assert len(missing) == 0, f"Missing features: {missing}"
        assert len(extra) == 0, f"Unexpected extra features: {extra}"
        assert len(features_df.columns) == 22, \
            f"Expected 22 features, got {len(features_df.columns)}"

    def test_no_nan_in_features(self, sample_ohlcv_data):
        """Verify no NaN values after cleaning."""
        features_df = calculate_features(sample_ohlcv_data)
        features_df = handle_missing_values(features_df)
        
        nan_count = features_df.isna().sum().sum()
        assert nan_count == 0, f"Found {nan_count} NaN values in features"

    def test_feature_ranges_sensible(self, sample_ohlcv_data):
        """Verify features are in expected ranges."""
        features_df = calculate_features(sample_ohlcv_data)
        features_df = handle_missing_values(features_df)
        
        # Returns should be roughly in [-0.5, 0.5] range for normal markets
        assert features_df['returns'].abs().max() < 1.0, \
            "Returns should be less than 100%"
        
        # RSI should be in [0, 100]
        assert features_df['rsi'].min() >= 0, "RSI should be >= 0"
        assert features_df['rsi'].max() <= 100, "RSI should be <= 100"
        
        # BB %B can technically be outside [0, 1] but should be reasonable
        assert features_df['bb_percent_b'].abs().max() < 10, \
            "BB %B should be in reasonable range"
        
        # Cyclical features should be in [-1, 1]
        assert features_df['hour_sin'].min() >= -1, "hour_sin should be >= -1"
        assert features_df['hour_sin'].max() <= 1, "hour_sin should be <= 1"
        assert features_df['day_of_week_sin'].min() >= -1, \
            "day_of_week_sin should be >= -1"
        assert features_df['day_of_week_sin'].max() <= 1, \
            "day_of_week_sin should be <= 1"

    def test_returns_calculation(self, sample_ohlcv_data):
        """Test returns calculation is correct."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Calculate expected returns manually
        expected_returns = (
            sample_ohlcv_data['close'] - sample_ohlcv_data['open']
        ) / sample_ohlcv_data['open']
        
        pd.testing.assert_series_equal(
            features_df['returns'],
            expected_returns,
            check_names=False
        )

    def test_log_returns_calculation(self, sample_ohlcv_data):
        """Test log returns calculation."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Log returns = ln(close/open)
        expected_log_returns = np.log(
            sample_ohlcv_data['close'] / sample_ohlcv_data['open']
        )
        
        pd.testing.assert_series_equal(
            features_df['log_returns'],
            expected_log_returns,
            check_names=False
        )

    def test_volume_change_calculation(self, sample_ohlcv_data):
        """Test volume change calculation."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Volume change = volume / volume.shift(1) - 1
        expected_vol_change = (
            sample_ohlcv_data['volume'] / sample_ohlcv_data['volume'].shift(1) - 1
        )
        
        pd.testing.assert_series_equal(
            features_df['volume_change'],
            expected_vol_change,
            check_names=False
        )

    def test_sma_calculations(self, sample_ohlcv_data):
        """Test SMA calculations."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # SMA(5)
        expected_sma5 = sample_ohlcv_data['close'].rolling(5).mean()
        pd.testing.assert_series_equal(
            features_df['sma_5'],
            expected_sma5,
            check_names=False
        )
        
        # SMA distance features
        pd.testing.assert_series_equal(
            features_df['sma_5_distance'],
            expected_sma5 - sample_ohlcv_data['close'],
            check_names=False
        )

    def test_cyclical_time_features(self, sample_ohlcv_data):
        """Test cyclical encoding of time features."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Extract hour from timestamp
        hours = sample_ohlcv_data['timestamp'].dt.hour
        expected_hour_sin = np.sin(2 * np.pi * hours / 24)
        
        pd.testing.assert_series_equal(
            features_df['hour_sin'],
            expected_hour_sin,
            check_names=False
        )
        
        # Extract day of week (Monday=0, Sunday=6)
        days = sample_ohlcv_data['timestamp'].dt.dayofweek
        expected_day_sin = np.sin(2 * np.pi * days / 7)
        
        pd.testing.assert_series_equal(
            features_df['day_of_week_sin'],
            expected_day_sin,
            check_names=False
        )

    def test_candlestick_features(self, sample_ohlcv_data):
        """Test candlestick pattern features."""
        features_df = calculate_features(sample_ohlcv_data)
        
        # Check that body_size is in [0, 1] range
        assert features_df['body_size'].min() >= 0, "Body size should be >= 0"
        assert features_df['body_size'].max() <= 1, "Body size should be <= 1"
        
        # Check that upper_shadow and lower_shadow are in [0, 1] range
        assert features_df['upper_shadow'].min() >= 0, \
            "Upper shadow should be >= 0"
        assert features_df['upper_shadow'].max() <= 1, \
            "Upper shadow should be <= 1"
        assert features_df['lower_shadow'].min() >= 0, \
            "Lower shadow should be >= 0"
        assert features_df['lower_shadow'].max() <= 1, \
            "Lower shadow should be <= 1"


class TestDataLoading:
    """Test data loading functions."""

    def test_load_data_reads_csv(self, tmp_path):
        """Test that CSV data is loaded correctly."""
        # Create a test CSV
        csv_path = tmp_path / "test_data.csv"
        test_data = pd.DataFrame({
            'timestamp': ['2024-01-01T00:00:00+00:00', '2024-01-01T00:00:01+00:00'],
            'open': [42000.0, 42100.0],
            'high': [42100.0, 42200.0],
            'low': [41900.0, 42000.0],
            'close': [42050.0, 42150.0],
            'volume': [100.0, 150.0],
        })
        test_data.to_csv(csv_path, index=False)
        
        df = load_data(str(csv_path))
        
        assert len(df) == 2
        assert list(df.columns) == ['timestamp', 'open', 'high', 'low', 'close', 'volume']
        assert 'datetime64' in str(df['timestamp'].dtype) and 'UTC' in str(df['timestamp'].dtype)

    def test_load_data_missing_file(self, tmp_path):
        """Test handling of missing file."""
        missing_path = tmp_path / "nonexistent.csv"
        
        with pytest.raises(FileNotFoundError):
            load_data(str(missing_path))


class TestMissingValueHandling:
    """Test NaN handling."""

    def test_forward_fill_then_backward_fill(self):
        """Test that NaNs are forward filled then backward filled."""
        df = pd.DataFrame({
            'returns': [1.0, np.nan, np.nan, 2.0, np.nan],
            'volume': [100, 150, np.nan, 200, 250],
        })
        
        result = handle_missing_values(df)
        
        # After forward fill: [1.0, 1.0, 1.0, 2.0, 2.0]
        # After backward fill: [1.0, 1.0, 1.0, 2.0, 2.0]
        expected_returns = pd.Series([1.0, 1.0, 1.0, 2.0, 2.0], name='returns')
        pd.testing.assert_series_equal(result['returns'], expected_returns)

    def test_remaining_nans_dropped(self):
        """Test that rows with remaining NaNs are dropped."""
        df = pd.DataFrame({
            'a': [1.0, 2.0, np.nan],
            'b': [np.nan, 2.0, 3.0],
        })
        
        result = handle_missing_values(df)
        
        # Row 0: a=1.0, b=nan -> b forward fill not possible, backward fill from row 1
        # Row 1: a=2.0, b=2.0 -> complete
        # Row 2: a=nan, b=3.0 -> a forward fill from row 1
        # After forward then backward: all rows should be complete
        assert result.isna().sum().sum() == 0


class TestFeatureSaving:
    """Test feature saving functionality."""

    def test_save_to_parquet(self, tmp_path):
        """Test that features are saved to parquet format."""
        features_df = pd.DataFrame({
            'returns': [0.01, 0.02, -0.01],
            'volume': [100.0, 150.0, 200.0],
        })
        
        output_path = tmp_path / "features.parquet"
        save_features(features_df, str(output_path))
        
        assert output_path.exists(), "Parquet file should be created"
        
        # Read back and verify
        loaded = pd.read_parquet(output_path)
        pd.testing.assert_frame_equal(features_df, loaded)


class TestCLI:
    """Test command-line interface."""

    def test_cli_with_default_args(self, tmp_path):
        """Test CLI with default arguments."""
        # Create test CSV
        csv_path = tmp_path / "btcusdt_1s.csv"
        test_data = pd.DataFrame({
            'timestamp': pd.date_range('2024-01-01', periods=50, freq='1s', tz='UTC'),
            'open': np.random.uniform(40000, 45000, 50),
            'high': np.random.uniform(40000, 45000, 50),
            'low': np.random.uniform(40000, 45000, 50),
            'close': np.random.uniform(40000, 45000, 50),
            'volume': np.random.uniform(100, 500, 50),
        })
        test_data.to_csv(csv_path, index=False)

        output_path = tmp_path / "features.parquet"

        with patch('sys.argv', [
            'features.py',
            '--input', str(csv_path),
            '--output', str(output_path)
        ]):
            exit_code = main()

        assert exit_code == 0, "CLI should return 0 on success"
        assert output_path.exists(), "Output file should be created"

        # Verify content
        features = pd.read_parquet(output_path)
        assert len(features.columns) == 22

    def test_cli_missing_input_file(self, tmp_path):
        """Test CLI with non-existent input file."""
        missing_path = tmp_path / "nonexistent.csv"
        output_path = tmp_path / "features.parquet"

        with patch('sys.argv', [
            'features.py',
            '--input', str(missing_path),
            '--output', str(output_path)
        ]):
            exit_code = main()

        assert exit_code != 0, "CLI should return non-zero on error"


# Import patch for CLI tests
from unittest.mock import patch
