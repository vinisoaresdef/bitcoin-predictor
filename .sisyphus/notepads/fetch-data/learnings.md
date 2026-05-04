# Fetch Data Implementation - Learnings

## Successful Approaches

### TDD Pattern
- Writing tests first forced clear API design
- Mock-based testing allowed rapid iteration without hitting real API
- Parameterized tests for CLI arguments provided good coverage

### Rate Limiting Strategy
- Implemented exponential backoff (1s, 2s, 4s) for 429 responses
- Used Retry-After header when provided by API
- Added inter-request sleep (60/1200 = 0.05s) to stay under 1200 req/min limit

### Pagination Implementation
- Used `closeTime + 1ms` as next `startTime` to avoid duplicate candles
- Safety check: break if fewer than 1000 candles returned
- This handles the edge case where API returns exactly 1000 candles

### Binance API Specifics
- Kline endpoint: `/api/v3/klines?symbol={}&interval={}&startTime={}&endTime={}&limit=1000`
- Response format: 12-element array [openTime, open, high, low, close, volume, closeTime, ...]
- Timestamps in milliseconds since epoch
- Maximum 1000 candles per request
- Rate limit: 1200 request weight per minute

## Testing Patterns

### Mock Strategy
```python
@patch('fetch_data.requests.get')
def test_api(self, mock_get):
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = [...]
    mock_get.return_value = mock_response
```

### CLI Testing with kwargs
When mocking functions called via argparse, use:
```python
kwargs = mock_fetch.call_args.kwargs
assert kwargs.get('symbol') == "BTCUSDT"
```

## Files Created
- `training/fetch_data.py` - Main implementation
- `tests/test_fetch_data.py` - Comprehensive test suite
- `training/data/` - Directory for CSV output
- `.gitignore` - Excludes CSV files from git
- Updated `requirements.txt` - Added requests, pytest, pytest-mock
