# Feature Engineering Pipeline Learnings

## Task 16: Create feature engineering pipeline with exactly 22 features

### Files Created
- `training/features.py` - Main feature engineering module
- `tests/test_features.py` - Comprehensive test suite

### Features Implemented (22 total)

#### Price-based features (6)
1. returns - (close - open) / open
2. log_returns - ln(close / open)
3. high_low_range - (high - low) / low
4. body_size - abs(close - open) / (high - low + epsilon)
5. upper_shadow - (high - max(open, close)) / (high - low + epsilon), clipped to [0, 1]
6. lower_shadow - (min(open, close) - low) / (high - low + epsilon), clipped to [0, 1]

#### Volume features (2)
7. volume - raw volume
8. volume_change - volume / volume.shift(1) - 1

#### Moving averages (6)
9. sma_5 - 5-period SMA of close
10. sma_10 - 10-period SMA of close
11. sma_20 - 20-period SMA of close
12. sma_5_distance - sma_5 - close
13. sma_10_distance - sma_10 - close
14. sma_20_distance - sma_20 - close

#### Technical indicators (4)
15. rsi - RSI(14) via pandas-ta-classic
16. macd_histogram - MACD histogram (12,26,9) via pandas-ta-classic
17. bb_percent_b - Bollinger Band %B (20,2) via pandas-ta-classic
18. atr - ATR(14) via pandas-ta-classic

#### Momentum features (2)
19. price_momentum - close - close.shift(5)
20. price_acceleration - momentum.diff()

#### Time features (2)
21. hour_sin - sin(2*pi*hour/24) - cyclical hour encoding
22. day_of_week_sin - sin(2*pi*day/7) - cyclical day encoding

### Key Implementation Decisions

#### NaN Handling Strategy
- Forward fill (ffill) to propagate last known values
- Backward fill (bfill) for any remaining NaNs at start
- Drop any rows still containing NaN values
- This preserves maximum data while ensuring clean features

#### Shadow Feature Clipping
Upper and lower shadow calculations can produce negative values with synthetic/random data where high < max(open, close). Added `.clip(0, 1)` to ensure valid [0, 1] range.

#### Parquet Format
Using parquet instead of CSV for features provides:
- Binary compression (smaller file size)
- Type preservation
- Faster read/write
- Better for downstream ML pipelines

### Testing Approach

TDD approach with comprehensive coverage:
- Feature presence validation (22 features)
- NaN handling verification
- Range validation for each feature type
- Calculation accuracy tests
- CLI interface tests
- Data loading/saving tests

### Dependencies
- pandas-ta-classic for technical indicators (RSI, MACD, BBANDS, ATR)
- pyarrow for parquet support
- pytest + pytest-mock for testing

### CLI Usage
```bash
# Default paths
python training/features.py

# Custom paths
python training/features.py --input training/data/btcusdt_1s.csv --output training/data/features.parquet
```

### Verification Results
- Successfully processed 172,800 rows of 1s OHLCV data
- All 22 features present
- Zero NaN values after cleaning
- All features within expected ranges
- 26/26 tests passing

### Performance Notes
- Processing 172k rows takes ~2-3 seconds on modern hardware
- Memory efficient due to pandas vectorized operations
- Parquet output is ~20% the size of equivalent CSV
