"""Fetch historical candle data from Binance REST API for model training."""

import argparse
import csv
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import List, Optional

import requests

# Binance API configuration
BINANCE_API_URL = "https://api.binance.com/api/v3/klines"
MAX_CANDLES_PER_REQUEST = 1000
RATE_LIMIT_REQUESTS_PER_MINUTE = 1200

# Retry configuration
MAX_RETRIES = 3
INITIAL_BACKOFF = 1.0  # seconds


def ms_to_iso(timestamp_ms: int) -> str:
    """Convert millisecond timestamp to ISO format with timezone."""
    dt = datetime.fromtimestamp(timestamp_ms / 1000, tz=timezone.utc)
    return dt.isoformat()


def fetch_klines(
    symbol: str,
    interval: str,
    start_time: int,
    end_time: int,
    limit: int = MAX_CANDLES_PER_REQUEST
) -> List[List]:
    """Fetch klines from Binance API with retry logic.
    
    Args:
        symbol: Trading pair (e.g., BTCUSDT)
        interval: Candle interval (e.g., 1s, 1m, 1h)
        start_time: Start time in milliseconds
        end_time: End time in milliseconds
        limit: Maximum candles per request (max 1000)
    
    Returns:
        List of kline data arrays
    
    Raises:
        Exception: If max retries exceeded
    """
    params = {
        "symbol": symbol,
        "interval": interval,
        "startTime": start_time,
        "endTime": end_time,
        "limit": limit
    }
    
    backoff = INITIAL_BACKOFF
    
    for attempt in range(MAX_RETRIES):
        try:
            response = requests.get(BINANCE_API_URL, params=params, timeout=30)
            
            if response.status_code == 200:
                return response.json()
            elif response.status_code == 429:
                # Rate limited - check for Retry-After header
                retry_after = response.headers.get('Retry-After')
                if retry_after:
                    sleep_time = int(retry_after)
                else:
                    sleep_time = backoff
                
                if attempt < MAX_RETRIES - 1:
                    time.sleep(sleep_time)
                    backoff *= 2  # Exponential backoff
                    continue
                else:
                    raise Exception(
                        f"Max retries exceeded. Rate limited (429). "
                        f"Response: {response.json()}"
                    )
            else:
                # Other API errors
                if attempt < MAX_RETRIES - 1:
                    time.sleep(backoff)
                    backoff *= 2
                    continue
                else:
                    raise Exception(
                        f"Max retries exceeded. API error {response.status_code}. "
                        f"Response: {response.json()}"
                    )
                    
        except requests.exceptions.RequestException as e:
            if attempt < MAX_RETRIES - 1:
                time.sleep(backoff)
                backoff *= 2
                continue
            else:
                raise Exception(f"Max retries exceeded. Request failed: {e}")
    
    return []


def fetch_all_klines(
    symbol: str,
    interval: str,
    start_time: int,
    end_time: int,
    output_path: str
) -> None:
    """Fetch all klines with pagination and save to CSV.
    
    Args:
        symbol: Trading pair
        interval: Candle interval
        start_time: Start time in milliseconds
        end_time: End time in milliseconds
        output_path: Path to output CSV file
    """
    all_klines: List[List] = []
    current_start = start_time
    
    # Ensure output directory exists
    output_file = Path(output_path)
    output_file.parent.mkdir(parents=True, exist_ok=True)
    
    while current_start < end_time:
        klines = fetch_klines(
            symbol=symbol,
            interval=interval,
            start_time=current_start,
            end_time=end_time,
            limit=MAX_CANDLES_PER_REQUEST
        )
        
        if not klines:
            break
        
        all_klines.extend(klines)
        
        # Update start time for next request
        # Last candle's close time + 1ms
        last_candle = klines[-1]
        current_start = last_candle[6] + 1  # closeTime + 1ms
        
        # Safety check - if we got fewer than max, we're done
        if len(klines) < MAX_CANDLES_PER_REQUEST:
            break
        
        # Rate limiting - ensure we don't exceed 1200 req/min
        # Sleep to maintain safe rate
        time.sleep(60.0 / RATE_LIMIT_REQUESTS_PER_MINUTE)
    
    # Save to CSV
    save_to_csv(all_klines, output_path)
    
    print(f"Fetched {len(all_klines)} candles from {symbol}")
    print(f"Time range: {ms_to_iso(start_time)} to {ms_to_iso(end_time)}")
    print(f"Saved to: {output_path}")


def save_to_csv(klines: List[List], output_path: str) -> None:
    """Save klines to CSV file.
    
    Args:
        klines: List of kline arrays from Binance API
        output_path: Path to output CSV file
    """
    fieldnames = ['timestamp', 'open', 'high', 'low', 'close', 'volume']
    
    with open(output_path, 'w', newline='') as csvfile:
        writer = csv.DictWriter(csvfile, fieldnames=fieldnames)
        writer.writeheader()
        
        for kline in klines:
            # Binance kline format:
            # [openTime, open, high, low, close, volume, closeTime, ...]
            writer.writerow({
                'timestamp': ms_to_iso(kline[0]),  # openTime in ISO format
                'open': kline[1],
                'high': kline[2],
                'low': kline[3],
                'close': kline[4],
                'volume': kline[5]
            })


def main():
    """Main entry point for CLI."""
    parser = argparse.ArgumentParser(
        description='Fetch historical BTCUSDT candle data from Binance API'
    )
    parser.add_argument(
        '--days',
        type=int,
        default=30,
        help='Number of days of historical data to fetch (default: 30)'
    )
    parser.add_argument(
        '--output',
        type=str,
        default='training/data/btcusdt_1s.csv',
        help='Output CSV file path (default: training/data/btcusdt_1s.csv)'
    )
    parser.add_argument(
        '--symbol',
        type=str,
        default='BTCUSDT',
        help='Trading pair symbol (default: BTCUSDT)'
    )
    parser.add_argument(
        '--interval',
        type=str,
        default='1s',
        help='Candle interval: 1s, 1m, 5m, 1h, etc (default: 1s)'
    )
    
    args = parser.parse_args()
    
    # Calculate time range
    # Use current time as end time
    end_time_ms = int(time.time() * 1000)
    start_time_ms = end_time_ms - (args.days * 24 * 60 * 60 * 1000)
    
    print(f"Fetching {args.days} days of {args.symbol} {args.interval} candles...")
    
    fetch_all_klines(
        symbol=args.symbol,
        interval=args.interval,
        start_time=start_time_ms,
        end_time=end_time_ms,
        output_path=args.output
    )


if __name__ == '__main__':
    main()
