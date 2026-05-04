# Wave 5 Task 22 Learnings

## Implementation Summary

### POST /predict Endpoint
- **File**: `ml-service/main.py`
- **Request**: `{"candles": [...]}` (60 candles with OHLCV + timestamp)
- **Response**: `{"direction": "UP|DOWN|UNCERTAIN", "confidence": 0.72, "probabilities": [0.72, 0.15, 0.13]}`

### Key Implementation Details

1. **Validation**: Uses Pydantic `Field(min_length=60, max_length=60)` for automatic validation
2. **Feature Engineering**: Implemented `compute_features_from_candles()` function that mirrors `training/features.py` patterns
   - All 22 features computed using pandas-ta-classic
   - Handles missing values with forward/backward fill
3. **Model Inference**: 
   - Uses LightGBM `predict_proba()` to get class probabilities
   - Classes: UP=0, DOWN=1, UNCERTAIN=2
4. **Threshold Logic**:
   - UP: probability[0] > 0.55
   - DOWN: probability[1] > 0.55
   - UNCERTAIN: otherwise
5. **Error Handling**:
   - HTTP 422: <60 candles or insufficient valid data
   - HTTP 503: Model not loaded

### Tests Added
- `test_predict_returns_valid_direction`: Mock 60 candles → verify valid response
- `test_predict_with_insufficient_data`: 59 candles → verify 422 error
- `test_predict_model_not_loaded`: Model=None → verify 503 error

### Technical Notes
- Feature order must match training exactly (satisfied by using same column order as features.py)
- Used `Field(..., pattern="^(UP|DOWN|UNCERTAIN)$")` for response validation
- Mocked model in tests to avoid actual inference during unit tests
- Pydantic validation errors return list format, not string

### Dependencies Used
- FastAPI for web framework
- Pydantic v2 for validation (pattern field constraint)
- pandas-ta-classic for technical indicators
- numpy/pandas for feature computation
