"""Tests for training/fetch_data.py

TDD approach: Tests written first, implementation follows.
"""

import os
import sys
import pytest
import csv
from datetime import datetime, timezone
from unittest.mock import Mock, patch, MagicMock
from pathlib import Path

# Add training directory to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'training'))

from fetch_data import (
    fetch_klines,
    fetch_all_klines,
    save_to_csv,
    ms_to_iso,
    MAX_RETRIES,
    INITIAL_BACKOFF,
)


class TestFetchDataSavesCsv:
    """Test that fetch_data correctly fetches data and saves to CSV."""

    @patch('fetch_data.requests.get')
    def test_fetch_data_saves_csv(self, mock_get, tmp_path):
        """Mock API → verify CSV created with correct columns."""
        # Setup mock response - simulate 2 candles
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = [
            # Binance kline format: [openTime, open, high, low, close, volume, closeTime, ...]
            [
                1704067200000,  # openTime (2024-01-01 00:00:00 UTC)
                "42000.00",     # open
                "42100.00",     # high
                "41900.00",     # low
                "42050.00",     # close
                "100.5",        # volume
                1704067200999,  # closeTime
                "4221150.00",   # quoteAssetVolume
                100,            # numberOfTrades
                "50.25",        # takerBuyBaseAssetVolume
                "2110575.00",   # takerBuyQuoteAssetVolume
                "0"             # ignore
            ],
            [
                1704067201000,  # openTime (2024-01-01 00:00:01 UTC)
                "42050.00",     # open
                "42150.00",     # high
                "42000.00",     # low
                "42100.00",     # close
                "150.0",        # volume
                1704067201999,  # closeTime
                "6322500.00",   # quoteAssetVolume
                150,            # numberOfTrades
                "75.0",         # takerBuyBaseAssetVolume
                "3161250.00",   # takerBuyQuoteAssetVolume
                "0"             # ignore
            ]
        ]
        mock_get.return_value = mock_response

        # Call function
        output_path = tmp_path / "test_output.csv"
        symbol = "BTCUSDT"
        interval = "1s"
        start_time = 1704067200000
        end_time = 1704067202000

        fetch_all_klines(symbol, interval, start_time, end_time, str(output_path))

        # Verify CSV was created
        assert output_path.exists(), "CSV file should be created"

        # Verify CSV content
        with open(output_path, 'r') as f:
            reader = csv.DictReader(f)
            rows = list(reader)

        # Check columns
        expected_columns = {'timestamp', 'open', 'high', 'low', 'close', 'volume'}
        assert set(reader.fieldnames) == expected_columns, \
            f"Expected columns {expected_columns}, got {set(reader.fieldnames)}"

        # Check data rows
        assert len(rows) == 2, f"Expected 2 rows, got {len(rows)}"

        # Verify first row
        assert rows[0]['timestamp'] == '2024-01-01T00:00:00+00:00'
        assert rows[0]['open'] == '42000.00'
        assert rows[0]['high'] == '42100.00'
        assert rows[0]['low'] == '41900.00'
        assert rows[0]['close'] == '42050.00'
        assert rows[0]['volume'] == '100.5'

        # Verify second row
        assert rows[1]['timestamp'] == '2024-01-01T00:00:01+00:00'
        assert rows[1]['open'] == '42050.00'
        assert rows[1]['close'] == '42100.00'

    @patch('fetch_data.requests.get')
    def test_pagination_handles_multiple_requests(self, mock_get, tmp_path):
        """Test that pagination works when more than 1000 candles needed."""
        # First request returns 1000 candles, second returns 500
        call_count = [0]
        
        def mock_response(*args, **kwargs):
            call_count[0] += 1
            mock_resp = MagicMock()
            mock_resp.status_code = 200
            
            if call_count[0] == 1:
                # First call - return 1000 candles
                candles = []
                base_time = 1704067200000
                for i in range(1000):
                    candles.append([
                        base_time + (i * 1000),  # openTime
                        "42000.00", "42100.00", "41900.00", "42050.00",  # OHLC
                        "100.5",  # volume
                        base_time + (i * 1000) + 999,  # closeTime
                        "0", "0", "0", "0", "0"
                    ])
                mock_resp.json.return_value = candles
            else:
                # Second call - return 2 more candles
                base_time = 1704067200000 + (1000 * 1000)
                mock_resp.json.return_value = [
                    [
                        base_time, "42000.00", "42100.00", "41900.00", "42050.00",
                        "100.5", base_time + 999, "0", "0", "0", "0", "0"
                    ],
                    [
                        base_time + 1000, "42050.00", "42100.00", "42000.00", "42075.00",
                        "200.0", base_time + 1999, "0", "0", "0", "0", "0"
                    ]
                ]
            
            return mock_resp

        mock_get.side_effect = mock_response

        # Request 1002 candles (requires 2 requests due to 1000 limit)
        output_path = tmp_path / "test_pagination.csv"
        symbol = "BTCUSDT"
        interval = "1s"
        start_time = 1704067200000
        end_time = 1704067200000 + (1002 * 1000)

        fetch_all_klines(symbol, interval, start_time, end_time, str(output_path))

        # Verify multiple requests were made
        assert call_count[0] == 2, f"Expected 2 API calls, got {call_count[0]}"

        # Verify CSV has all 1002 candles
        with open(output_path, 'r') as f:
            reader = csv.DictReader(f)
            rows = list(reader)
        
        assert len(rows) == 1002, f"Expected 1002 rows, got {len(rows)}"


class TestRateLimitHandling:
    """Test rate limiting and retry logic."""

    @patch('fetch_data.time.sleep')
    @patch('fetch_data.requests.get')
    def test_rate_limit_handling(self, mock_get, mock_sleep, tmp_path):
        """Simulate 429 response → verify backoff and retry."""
        # First two calls return 429, third succeeds
        call_count = [0]

        def mock_response(*args, **kwargs):
            call_count[0] += 1
            mock_resp = MagicMock()

            if call_count[0] <= 2:
                # First two calls fail with rate limit (no Retry-After header)
                mock_resp.status_code = 429
                mock_resp.json.return_value = {"msg": "Rate limit exceeded"}
                mock_resp.headers = {}  # No Retry-After, so exponential backoff used
            else:
                # Third call succeeds
                mock_resp.status_code = 200
                mock_resp.json.return_value = [
                    [
                        1704067200000, "42000.00", "42100.00", "41900.00", "42050.00",
                        "100.5", 1704067200999, "0", "0", "0", "0", "0"
                    ]
                ]

            return mock_resp

        mock_get.side_effect = mock_response

        output_path = tmp_path / "test_rate_limit.csv"
        symbol = "BTCUSDT"
        interval = "1s"
        start_time = 1704067200000
        end_time = 1704067201000

        fetch_all_klines(symbol, interval, start_time, end_time, str(output_path))

        # Verify multiple requests were made (retries happened)
        assert call_count[0] == 3, f"Expected 3 API calls (2 retries), got {call_count[0]}"

        # Verify sleep was called for backoff (exponential: 1s, 2s)
        assert mock_sleep.call_count == 2, \
            f"Expected 2 sleep calls for backoff, got {mock_sleep.call_count}"

        # Verify exponential backoff
        sleep_calls = [call.args[0] for call in mock_sleep.call_args_list]
        assert sleep_calls[0] == pytest.approx(INITIAL_BACKOFF, rel=0.01), \
            f"First backoff should be {INITIAL_BACKOFF}s, got {sleep_calls[0]}"
        assert sleep_calls[1] == pytest.approx(INITIAL_BACKOFF * 2, rel=0.01), \
            f"Second backoff should be {INITIAL_BACKOFF * 2}s, got {sleep_calls[1]}"

        # Verify CSV was still created
        assert output_path.exists(), "CSV should be created after successful retry"

    @patch('fetch_data.requests.get')
    def test_max_retries_exceeded(self, mock_get, tmp_path):
        """Test that exception is raised after max retries exceeded."""
        # All calls return 429
        mock_response = MagicMock()
        mock_response.status_code = 429
        mock_response.json.return_value = {"msg": "Rate limit exceeded"}
        mock_response.headers = {}
        mock_get.return_value = mock_response

        output_path = tmp_path / "test_max_retries.csv"
        
        with pytest.raises(Exception) as exc_info:
            fetch_all_klines("BTCUSDT", "1s", 1704067200000, 1704067201000, str(output_path))
        
        assert "Max retries exceeded" in str(exc_info.value)
        assert mock_get.call_count == MAX_RETRIES, \
            f"Expected {MAX_RETRIES} attempts, got {mock_get.call_count}"


class TestUtilityFunctions:
    """Test helper functions."""

    def test_ms_to_iso(self):
        """Test millisecond timestamp to ISO format conversion."""
        # Test known timestamp: 2024-01-01 00:00:00 UTC
        ms = 1704067200000
        expected = "2024-01-01T00:00:00+00:00"
        result = ms_to_iso(ms)
        assert result == expected, f"Expected {expected}, got {result}"

    def test_ms_to_iso_with_timezone(self):
        """Test ISO format includes timezone info."""
        ms = 1704067200000
        result = ms_to_iso(ms)
        assert result.endswith('+00:00'), f"Expected +00:00 timezone, got {result}"

    @patch('fetch_data.requests.get')
    def test_api_error_handling(self, mock_get, tmp_path):
        """Test handling of other API errors (500, 503, etc)."""
        mock_response = MagicMock()
        mock_response.status_code = 500
        mock_response.json.return_value = {"msg": "Internal server error"}
        mock_get.return_value = mock_response

        output_path = tmp_path / "test_error.csv"
        
        with pytest.raises(Exception) as exc_info:
            fetch_all_klines("BTCUSDT", "1s", 1704067200000, 1704067201000, str(output_path))
        
        assert "Max retries exceeded" in str(exc_info.value)


class TestCLI:
    """Test command-line interface."""

    @patch('fetch_data.fetch_all_klines')
    def test_cli_default_args(self, mock_fetch, tmp_path):
        """Test CLI with default arguments."""
        from fetch_data import main

        with patch('sys.argv', ['fetch_data.py']):
            with patch('fetch_data.Path') as mock_path:
                mock_path.return_value = tmp_path / "data"
                main()

        # Verify fetch_all_klines was called with default args (kwargs)
        mock_fetch.assert_called_once()
        kwargs = mock_fetch.call_args.kwargs
        assert kwargs.get('symbol') == "BTCUSDT"
        assert kwargs.get('interval') == "1s"
        assert kwargs.get('output_path') == "training/data/btcusdt_1s.csv"

    @patch('fetch_data.fetch_all_klines')
    def test_cli_custom_args(self, mock_fetch, tmp_path):
        """Test CLI with custom arguments."""
        from fetch_data import main

        custom_output = str(tmp_path / "custom.csv")

        with patch('sys.argv', ['fetch_data.py', '--days', '7', '--output', custom_output]):
            main()

        mock_fetch.assert_called_once()
        kwargs = mock_fetch.call_args.kwargs

        # Verify custom days argument affects time range
        # 7 days = 7 * 24 * 60 * 60 * 1000 ms = 604800000 ms
        end_time = kwargs.get('end_time')
        start_time = kwargs.get('start_time')
        assert end_time - start_time == 7 * 24 * 60 * 60 * 1000
        assert kwargs.get('output_path') == custom_output

    @patch('fetch_data.fetch_all_klines')
    def test_cli_30_days(self, mock_fetch):
        """Test CLI with 30 days argument."""
        from fetch_data import main

        with patch('sys.argv', ['fetch_data.py', '--days', '30']):
            main()

        mock_fetch.assert_called_once()
        kwargs = mock_fetch.call_args.kwargs

        # Verify 30 days time range
        end_time = kwargs.get('end_time')
        start_time = kwargs.get('start_time')
        expected_ms = 30 * 24 * 60 * 60 * 1000
        assert end_time - start_time == expected_ms
