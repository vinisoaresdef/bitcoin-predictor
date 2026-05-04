# Predictor Platform — Real-Time Crypto Prediction

## TL;DR

> **Quick Summary**: Build a local crypto prediction platform with real-time candlestick streaming from Binance (Stage 1) and 30-second price direction prediction using LightGBM (Stage 2), rendered with TradingView Lightweight Charts.
> 
> **Deliverables**:
> - Go backend ingesting Binance websocket kline data, broadcasting to frontend via local websocket
> - Frontend HTML/JS with TradingView Lightweight Charts showing real + predicted candlesticks
> - Python FastAPI ML microservice with LightGBM classification (UP/DOWN/UNCERTAIN)
> - Historical model training pipeline (Stage 1.5)
> - Docker Compose orchestration (3 services)
> 
> **Estimated Effort**: Large (~30 tasks across 6 waves)
> **Parallel Execution**: YES — 5 waves of parallel tasks
> **Critical Path**: Go Backend → Frontend → Model Training → ML Service → Integration → Verification

---

## Context

### Original Request
Build a two-stage local crypto prediction platform: Stage 1 streams real-time candlestick data from a websocket provider and plots it on a minimal chart. Stage 2 predicts the next 30 seconds of price direction using ML, showing predicted candles and indicators semi-transparently alongside real data.

### Interview Summary
**Key Discussions**:
- **Backend Language**: Go (Recommended) — goroutines, channels, low memory, single binary
- **Data Provider**: Binance API — free, unlimited websocket, real-time `@kline_1s` streams
- **Initial Asset Scope**: Bitcoin/Crypto only (BTCUSDT hardcoded), expand later
- **Infrastructure**: Docker Compose with 3 containers from Day 1
- **Test Strategy**: TDD — `go test`, `pytest`, Playwright for E2E
- **ML Model**: LightGBM binary classification (UP/DOWN direction with confidence threshold)
- **News Sentiment**: Skipped for MVP (free real-time news APIs are limited)
- **Plan Structure**: Single comprehensive plan covering both stages as parallel waves

**Research Findings**:
- **Binance**: `btcusdt@kline_1s` confirmed — 1s interval, 1000ms update speed. But: 24h connection lifetime, 5 msg/s rate limit, may skip candles when no trades occur
- **TradingView Lightweight Charts**: v5.2.0, Apache 2.0, 35KB. Does NOT support per-candle opacity — requires dual `CandlestickSeries` with RGBA colors for predicted candles
- **TA Library**: `pandas-ta-classic` (62 native patterns, 200+ indicators, no C deps) replaces the initially considered `numta` (AI-generated, unproven, 23 stars)
- **LightGBM**: ~15ms inference (single, idle CPU), P99 ~50-100ms under load. Must use time-based walk-forward validation (NEVER random shuffle — causes look-ahead bias)
- **Docker**: Installed on host (v29.4.0 + Compose v5.1.3). Go NOT installed — added to prerequisites
- **Historical Data**: Stage 2 requires 30 days of 1s kline data for training — this was missing from the original architecture. Inserted as Stage 1.5

### Metis Review
**Critical Gaps Identified** (all addressed in this plan):
- **Gap 1**: `numta` is AI-generated with zero community validation → Replaced with `pandas-ta-classic`
- **Gap 2**: No model training pipeline → Inserted Stage 1.5 (historical data fetch + feature engineering + walk-forward validation + serialization)
- **Gap 3**: Per-candle opacity unsupported → Dual CandlestickSeries with RGBA colors + WhitespaceData gap
- **Gap 4**: 5m timeframe undefined for prediction → Removed from MVP, added to Future Work
- **Gap 5**: No feature vector defined → Concrete 22-feature spec in Stage 1.5
- **Gap 6**: Assumed `localhost` for Docker networking → Use service names (`ml-service:8000`, `go-backend:8080`)
- **Gap 7**: Assumed unlimited Binance websocket → Added reconnection logic, rate limit handling, connection expiry management
- **Gap 8**: No startup state defined (first 60s with no data) → Progressive "Collecting data X/60" state

---

## Work Objectives

### Core Objective
Build a local Docker-based platform that streams real-time Bitcoin candlestick data from Binance, displays it on a TradingView chart, and predicts the next 30-second price direction (UP/DOWN/UNCERTAIN) using a LightGBM classification model trained on historical data.

### Concrete Deliverables
- `go-backend/` — Go module with Binance WS client, internal WS server, HTTP server, ML service HTTP client
- `frontend/` — HTML/JS with TradingView Lightweight Charts, served by Go backend
- `ml-service/` — Python FastAPI with LightGBM model, pandas-ta-classic indicators, `/predict` and `/health` endpoints
- `training/` — Python script: fetch 30 days of 1s klines, engineer 22 features, train LightGBM, serialize to `model.txt`
- `docker-compose.yml` — 3 services (go-backend, ml-service) + build contexts
- `.env` — Port configuration, Binance API keys (optional for public data)

### Definition of Done
- [ ] `docker compose up` starts all 3 services with no errors
- [ ] `curl http://localhost:8080/health` → `{"status":"ok","binance":"connected"}`
- [ ] Frontend shows real BTCUSDT candlesticks updating every 1-2 seconds
- [ ] After 70 seconds: predicted candles appear (semi-transparent) + predicted SMA (dotted)
- [ ] `pytest ml-service/tests/` → all pass
- [ ] `go test ./...` → all pass (no race conditions)
- [ ] `docker compose down` → clean shutdown, exit code 0 within 10s

### Must Have
- Real-time BTCUSDT 1s candlestick streaming from Binance
- Thread-safe rolling buffer (60 candles for prediction window)
- TradingView candlestick chart with real-time updates
- LightGBM model that classifies next 30s as UP/DOWN/UNCERTAIN
- Dual CandlestickSeries overlay (real solid + predicted semi-transparent)
- 1 SMA(20) indicator line (solid) + 1 predicted SMA(20) line (dotted)
- Docker Compose orchestration with service-name networking
- Health endpoints on all services
- Graceful degradation when ML service is unavailable

### Must NOT Have (Guardrails)
- ❌ `numta` library — use `pandas-ta-classic` instead
- ❌ Per-candle opacity — use dual CandlestickSeries with RGBA colors
- ❌ 5-minute timeframe — removed from MVP (undefined prediction semantics)
- ❌ Asset dropdown / multi-asset support — hardcoded BTCUSDT only
- ❌ Trend lines, support/resistance lines — Phase 3 only
- ❌ News sentiment analysis — Phase 3 only
- ❌ `localhost` in inter-service code — use Docker service names
- ❌ Random `train_test_split` for model validation — use `TimeSeriesSplit` walk-forward
- ❌ More than 2 indicators on chart (1 SMA real + 1 SMA predicted)
- ❌ Any deployment config beyond local Docker

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: No (greenfield project — setup is part of the plan)
- **Automated tests**: TDD (RED → GREEN → REFACTOR)
- **Framework**: `go test` (with `-race` flag), `pytest` (Python), Playwright (frontend E2E)

### QA Policy
Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Frontend/UI**: Playwright — Navigate, wait for elements, assert DOM, screenshot
- **CLI/API**: Bash (curl) — Send requests, assert status + response fields
- **Go backend**: `go test -race` + `websocat` for websocket verification
- **ML service**: `pytest` + `curl` for endpoint verification

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 — Prerequisites & Setup (START IMMEDIATELY):
├── Task 1: Install Go on host
├── Task 2: Project scaffolding (directories, go.mod, requirements.txt, docker-compose.yml, .env)
└── Task 3: JSON schemas + shared data types (Go structs, Python Pydantic)

Wave 1 — Go Backend (after Wave 0):
├── Task 4: Binance WebSocket client (connect, parse @kline_1s, reconnect with backoff)
├── Task 5: Thread-safe candle buffer (ring buffer, max 60 candles, eviction)
├── Task 6: Internal WebSocket server (broadcast to frontend clients)
├── Task 7: HTTP server (serve frontend, /health endpoint, graceful shutdown)
└── Task 8: Wire everything together (main.go, signal handling, startup sequence)

Wave 2 — Frontend (can run PARALLEL with Wave 1 after Task 3):
├── Task 9: HTML page skeleton + dark theme CSS + status overlay
├── Task 10: TradingView chart initialization (CandlestickSeries, time axis, styling)
├── Task 11: Frontend WebSocket client (connect, reconnect, parse JSON messages)
├── Task 12: Real-time chart update loop (series.update(), auto-scroll)
├── Task 13: Startup state + status indicators (collecting data, connected, ML status)
└── Task 14: Real SMA(20) line series rendering

Wave 3 — Model Training Pipeline (PARALLEL with Waves 1 + 2):
├── Task 15: Historical data fetch from Binance REST API (30 days, 1s klines)
├── Task 16: Feature engineering pipeline (22 features from OHLCV)
├── Task 17: Label creation + walk-forward validation (TimeSeriesSplit)
└── Task 18: LightGBM training + serialization + metrics report

Wave 4 — ML Service (after Waves 1 + 3):
├── Task 19: FastAPI project + Dockerfile (lifespan, ORJSONResponse, uvicorn config)
├── Task 20: Model loading at startup (/health endpoint with model_loaded flag)
├── Task 21: TA computation (SMA via pandas-ta-classic, pattern recognition)
├── Task 22: /predict endpoint (input validation, inference, confidence threshold)
├── Task 23: Predicted candle construction from direction + confidence
└── Task 24: Error handling + graceful degradation (422, 503, timeout)

Wave 5 — Integration (after Waves 2 + 4):
├── Task 25: Go ↔ ML service HTTP client (timeout, retry, error handling)
├── Task 26: Prediction broadcast to frontend websocket (new message type)
├── Task 27: Frontend dual CandlestickSeries (real + predicted overlay with RGBA)
├── Task 28: Frontend predicted SMA line (dotted LineStyle)
├── Task 29: Confidence threshold rendering (UP=green, DOWN=red, UNCERTAIN=gray)
└── Task 30: ML-unavailable graceful degradation (frontend shows "Prediction unavailable")

Wave FINAL — Verification (after ALL tasks — 4 parallel reviews, then user okay):
├── Task F1: Plan Compliance Audit (oracle)
├── Task F2: Code Quality Review (unspecified-high)
├── Task F3: Real QA Execution (unspecified-high + playwright)
└── Task F4: Scope Fidelity Check (deep)
```

**Critical Path**: Task 1 → 3 → 4 → 5 → 8 → 7 → 11 → 12 → 27 → F1-F4
**Parallel Speedup**: ~60% faster than sequential
**Max Concurrent**: 6 tasks in Wave 2, 4 in Wave 3, 6 in Wave 4

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 4 | 0 |
| 2 | — | 4,8,10,20 | 0 |
| 3 | 2 | 4,5,6,22 | 0 |
| 4 | 1,3 | 8 | 1 |
| 5 | 3 | 6,8 | 1 |
| 6 | 3,5 | 8,11 | 1 |
| 7 | 2 | 8,11 | 1 |
| 8 | 4,5,6,7 | 19,25 | 1 |
| 9 | 2 | 10,13 | 2 |
| 10 | 9 | 12,14,27 | 2 |
| 11 | 6,7 | 12,13 | 2 |
| 12 | 10,11 | 14,27 | 2 |
| 13 | 9,11 | 30 | 2 |
| 14 | 10,12 | 28 | 2 |
| 15 | — | 16 | 3 |
| 16 | 15 | 17 | 3 |
| 17 | 16 | 18 | 3 |
| 18 | 17 | 20 | 3 |
| 19 | 2 | 20,22 | 4 |
| 20 | 18,19 | 22 | 4 |
| 21 | 3 | 22 | 4 |
| 22 | 3,19,20,21 | 25 | 4 |
| 23 | 22 | — | 4 |
| 24 | 19,20,22 | 30 | 4 |
| 25 | 3,22 | 26 | 5 |
| 26 | 6,25 | 27,28 | 5 |
| 27 | 10,26 | 29 | 5 |
| 28 | 14,26 | — | 5 |
| 29 | 27 | — | 5 |
| 30 | 13,24,27 | — | 5 |
| F1-F4 | ALL | — | FINAL |

### Agent Dispatch Summary

| Wave | Count | Tasks → Agent Profiles |
|------|-------|----------------------|
| 0 | 3 | T1 → `quick`, T2 → `quick`, T3 → `quick` |
| 1 | 5 | T4-T8 → `unspecified-high` |
| 2 | 6 | T9-T14 → `visual-engineering` (T9-14) EXCEPT T11 → `unspecified-low` |
| 3 | 4 | T15-T17 → `unspecified-high`, T18 → `deep` |
| 4 | 6 | T19-T21,T24 → `unspecified-high`, T22-T23 → `deep` |
| 5 | 6 | T25-T26 → `unspecified-high`, T27-T30 → `visual-engineering` |
| FINAL | 4 | F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high` + `playwright`, F4 → `deep` |

---

## TODOs

- [x] 1. **Install Go and verify prerequisites**

  **What to do**:
  - Check if Go is installed: `go version` — if missing, install Go 1.24+ via `sudo apt install golang-go` or official tarball
  - Verify Docker: `docker --version` (must be >= 27.0) and `docker compose version` (must be >= 2.0)
  - Verify python3: `python3 --version` (must be >= 3.11)
  - Create `.sisyphus/evidence/` directory
  - Document installed versions in a `PREREQUISITES.md` file

  **Must NOT do**:
  - Do NOT install Go via snap (conflicts with Docker volumes)
  - Do NOT change system-wide Python — use Docker for Python isolation

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple system tool verification and installation — no complex logic
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**: None relevant

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 2, 3)
  - **Blocks**: Task 4 (Binance WS client needs Go)
  - **Blocked By**: None (can start immediately)

  **References**:
  - Go install guide: `https://go.dev/doc/install` — Official Go installation steps for Linux
  - Docker Compose docs: `https://docs.docker.com/compose/` — Verify `docker compose` (not `docker-compose` v1)

  **Acceptance Criteria**:

  **TDD — Test first (RED → GREEN → REFACTOR):**
  - [ ] No code to test — infrastructure verification only
  - [ ] Verify: `go version` returns version >= 1.24
  - [ ] Verify: `docker compose version` succeeds

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go is installed and usable
    Tool: Bash
    Preconditions: Fresh OS state
    Steps:
      1. Run: go version
      2. Run: go env GOPATH
    Expected Result: Version >= 1.24.0 printed, GOPATH is valid path
    Evidence: .sisyphus/evidence/task-1-go-version.txt

  Scenario: Docker Compose is functional
    Tool: Bash
    Preconditions: Docker daemon running
    Steps:
      1. Run: docker compose version
      2. Run: docker compose ls
    Expected Result: Compose version printed, no errors from ls
    Evidence: .sisyphus/evidence/task-1-docker-version.txt
  ```

  **Commit**: YES (groups with Task 2, 3)
  - Message: `chore: project scaffolding and prerequisites`
  - Files: `PREREQUISITES.md`, `.env`, `docker-compose.yml`

---

- [x] 2. **Project scaffolding and Docker Compose setup**

  **What to do**:
  - Create directory structure:
    ```
    go-backend/
      cmd/predictor/main.go
      internal/
        binance/      # Binance WS client
        buffer/       # Candle ring buffer
        server/       # HTTP + WS server
        mlclient/     # ML service HTTP client
    frontend/
      index.html
      css/style.css
      js/
        chart.js      # TradingView chart init
        ws-client.js  # WebSocket client
        app.js        # Main orchestration
    ml-service/
      app/
        main.py       # FastAPI entry point
        model.py      # LightGBM loading + inference
        schemas.py    # Pydantic models
        indicators.py # pandas-ta integration
      tests/
      Dockerfile
    training/
      fetch_data.py   # Binance REST historical fetch
      features.py     # Feature engineering
      train.py        # LightGBM training
      validate.py     # Walk-forward validation
    ```
  - Create `go.mod`: `module github.com/vinicius-soares/predictor`
  - Create `requirements.txt`: `fastapi`, `uvicorn[standard]`, `lightgbm`, `pandas-ta-classic`, `pandas`, `numpy`, `orjson`, `pydantic`, `httpx`
  - Create `docker-compose.yml` with 3 services:
    - `go-backend`: build from `go-backend/Dockerfile`, port `8080:8080`, depends on `ml-service`
    - `ml-service`: build from `ml-service/Dockerfile`, port `8000:8000`
    - Network: `predictor-net` (bridge)
  - Create `.env` with: `PORT_BACKEND=8080`, `PORT_ML=8000`, `BINANCE_WS_URL=wss://stream.binance.com:9443/ws`, `BINANCE_REST_URL=https://api.binance.com`

  **Must NOT do**:
  - Do NOT create files outside the specified directory tree
  - Do NOT add any business logic files — scaffolding only

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: File creation and configuration — straightforward, no complex logic

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 1, 3)
  - **Blocks**: All subsequent tasks (directory structure is the foundation)
  - **Blocked By**: None

  **References**:
  - Go project layout standard: `https://github.com/golang-standards/project-layout` — Standard Go project directory structure
  - Docker Compose spec: `https://docs.docker.com/reference/compose-file/` — Compose file format reference
  - A similar Go + Docker project (if exists in codebase): None (greenfield)

  **Acceptance Criteria**:
  - [ ] All directories in the tree exist
  - [ ] `go.mod` contains module declaration
  - [ ] `requirements.txt` contains 9 packages listed above
  - [ ] `docker-compose.yml` validates: `docker compose config` succeeds
  - [ ] `.env` file exists with all 4 variables

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker Compose config is valid
    Tool: Bash
    Preconditions: docker-compose.yml and .env exist
    Steps:
      1. Run: docker compose config
    Expected Result: Parsed YAML output with all 3 services, no errors
    Evidence: .sisyphus/evidence/task-2-compose-config.txt

  Scenario: Directory structure matches spec
    Tool: Bash
    Steps:
      1. Run: ls -R go-backend/ frontend/ ml-service/ training/
    Expected Result: All directories from the tree above exist
    Evidence: .sisyphus/evidence/task-2-dir-tree.txt
  ```

  **Commit**: YES (groups with Task 1, 3)
  - Message: `chore: project scaffolding and prerequisites`
  - Files: All scaffolding files

---

- [x] 3. **JSON schemas and shared data types**

  **What to do**:
  - Define Go structs in `go-backend/internal/schemas/schemas.go`:
    - `Candle`: `{Symbol, Interval, Open, High, Low, Close, Volume, CloseTime, Timestamp}`
    - `StatusMessage`: `{Type:"status", Status, Timestamp}`
    - `KlineMessage`: `{Type:"kline", Candle}`
    - `PredictionMessage`: `{Type:"prediction", Direction, Confidence, PredictedCandle, PredictedMA, Timestamp}`
    - `IndicatorMessage`: `{Type:"indicator", Name, Values:[float64], PredictedValues:[float64]}`
  - Define Pydantic models in `ml-service/app/schemas.py`:
    - `PredictionInput`: `{candles: list[CandleData], features: list[str]}`
    - `PredictionOutput`: `{direction: Literal["UP","DOWN","UNCERTAIN"], confidence: float, predicted_candle: CandleData, predicted_ma: float}`
  - Define JSON message format spec as doc comments in Go code
  - Create `schemas/README.md` documenting the wire format between all 3 services

  **Must NOT do**:
  - Do NOT include business logic — only type definitions
  - Do NOT add validation logic beyond type annotations

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Type definitions and documentation — straightforward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 1, 2)
  - **Blocks**: Task 4 (Binance client uses Candle), Task 6 (WS server uses message types), Task 22 (ML uses Pydantic)
  - **Blocked By**: Task 2 (needs directory structure)

  **References**:
  - Binance kline message format: Study `{"e":"kline","E":...,"k":{"t":...,"o":"...","h":"...","l":"...","c":"...","v":"..."}}` pattern
  - Go JSON tags: `json:"field_name"` struct tags for marshaling
  - Pydantic v2 docs: `https://docs.pydantic.dev/latest/` — Model definition syntax

  **Acceptance Criteria**:
  - [ ] `schemas.go` compiles: `cd go-backend && go build ./internal/schemas/`
  - [ ] `schemas.py` imports without errors: `cd ml-service && python3 -c "from app.schemas import PredictionInput, PredictionOutput"`
  - [ ] `schemas/README.md` documents all 5 message types with JSON examples

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go structs roundtrip through JSON
    Tool: Bash (go test)
    Steps:
      1. Write test: marshal Candle → JSON → unmarshal → compare
      2. Run: go test ./internal/schemas/ -run TestCandleJSON
    Expected Result: PASS. JSON roundtrip preserves all float values within 0.0001
    Evidence: .sisyphus/evidence/task-3-go-schemas.txt

  Scenario: Pydantic models validate correctly
    Tool: Bash (python3)
    Preconditions: schemas.py exists
    Steps:
      1. Run: python3 -c "from app.schemas import PredictionInput; p=PredictionInput(candles=[], features=[]); print(p.model_dump_json())"
    Expected Result: Valid JSON output, no validation errors
    Evidence: .sisyphus/evidence/task-3-pydantic.txt
  ```

  **Commit**: YES (groups with Task 1, 2)
  - Message: `chore: project scaffolding and prerequisites`

- [x] 4. **Binance WebSocket client** [TDD]

  **What to do**:
  - Create `go-backend/internal/binance/client.go`
  - Connect to `wss://stream.binance.com:9443/ws/btcusdt@kline_1s` using `gorilla/websocket` or `nhooyr.io/websocket`
  - Parse incoming JSON messages into `schemas.Candle`
  - Handle reconnection: exponential backoff (1s → 2s → 4s → 8s → max 30s) on disconnect
  - Implement ping/pong: respond to Binance pings within 60 seconds
  - Handle connection expiry: proactively reconnect at 23.5 hours
  - Handle zero-volume candles: emit candle with volume=0, OHLC = last close
  - Handle duplicate candles: deduplicate by `close_time` key
  - Emit parsed candles to a Go channel: `chan<- schemas.Candle`
  - Write tests FIRST (TDD):
    - `TestParseKlineMessage` — valid JSON → correct Candle struct
    - `TestParseKlineMessage_MissingFields` — partial JSON → error
    - `TestDeduplicateByCloseTime` — duplicate close_time → only first emitted
    - `TestReconnectBackoff` — mock disconnect → backoff sequence verified

  **Must NOT do**:
  - Do NOT import any ML library
  - Do NOT compute any technical indicators
  - Do NOT hardcode timeouts without making them configurable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Network programming with reconnection logic, concurrency patterns — moderate complexity
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 5, 6, 7)
  - **Blocks**: Task 8 (wiring)
  - **Blocked By**: Task 3 (needs Candle schema)

  **References**:
  - Binance WebSocket docs: `https://binance-docs.github.io/apidocs/spot/en/#websocket-market-streams` — Official kline stream spec
  - `gorilla/websocket` examples: `https://github.com/gorilla/websocket/tree/main/examples` — WebSocket client patterns
  - Go channel patterns: Study how goroutines communicate via channels in Go concurrency model

  **Acceptance Criteria**:
  - [ ] `TestParseKlineMessage`: PASS
  - [ ] `TestParseKlineMessage_MissingFields`: PASS
  - [ ] `TestDeduplicateByCloseTime`: PASS
  - [ ] `TestReconnectBackoff`: PASS
  - [ ] `go test -race ./internal/binance/`: PASS (no data races)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Connects to real Binance and receives candles
    Tool: Bash (go test with integration build tag)
    Preconditions: Internet connection available
    Steps:
      1. Run: go test -tags=integration ./internal/binance/ -run TestRealBinanceConnection -timeout 30s
      2. Test connects to wss://stream.binance.com:9443/ws/btcusdt@kline_1s
      3. Reads at least 3 candles within 10 seconds
    Expected Result: 3+ candles received, all fields non-zero, close_time increments
    Evidence: .sisyphus/evidence/task-4-binance-live.txt

  Scenario: Reconnects after simulated disconnect
    Tool: Bash (go test)
    Steps:
      1. Run: go test ./internal/binance/ -run TestReconnect -v
    Expected Result: Test verifies backoff sequence (1s, 2s, 4s, 8s) then connection restored
    Evidence: .sisyphus/evidence/task-4-reconnect.txt
  ```

  **Commit**: YES (groups with Tasks 5-8)
  - Message: `feat(backend): Binance websocket streaming and internal broadcast`
  - Files: `go-backend/internal/binance/`

---

- [x] 5. **Thread-safe candle ring buffer** [TDD]

  **What to do**:
  - Create `go-backend/internal/buffer/buffer.go`
  - Implement thread-safe ring buffer with:
    - Configurable max size (default 60 for 60-second window)
    - `Append(candle schemas.Candle)` — adds candle, evicts oldest if at capacity
    - `Snapshot() []schemas.Candle` — returns copy of all candles in order
    - `Len() int` — current number of candles
    - `IsFull() bool` — true when buffer has max candles
  - Use `sync.RWMutex` for thread safety (multiple readers, single writer)
  - Write tests FIRST (TDD):
    - `TestAppendWithinCapacity` — append 30, expect Len=30
    - `TestAppendExceedsCapacity` — append 61 to max=60, expect Len=60, oldest evicted
    - `TestSnapshotReturnsCopy` — modify returned slice, buffer unchanged
    - `TestConcurrentReadWrite` — goroutines reading + writing concurrently, no races

  **Must NOT do**:
  - Do NOT use `container/ring` (returns nil elements, not zero-value)
  - Do NOT expose internal slice directly — always return copies

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Concurrent data structure with thread safety — moderate complexity

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 4, 6, 7)
  - **Blocks**: Task 8 (wiring), Task 12 (frontend needs buffer)
  - **Blocked By**: Task 3 (needs Candle schema)

  **References**:
  - Go `sync.RWMutex`: `https://pkg.go.dev/sync#RWMutex` — Read-write mutex for concurrent access patterns
  - Ring buffer patterns in Go: Classic circular buffer with head/tail pointers

  **Acceptance Criteria**:
  - [ ] `TestAppendWithinCapacity`: PASS
  - [ ] `TestAppendExceedsCapacity`: PASS
  - [ ] `TestSnapshotReturnsCopy`: PASS
  - [ ] `TestConcurrentReadWrite`: PASS
  - [ ] `go test -race ./internal/buffer/`: PASS (no data races)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Buffer evicts oldest candle at capacity
    Tool: Bash (go test)
    Steps:
      1. Run: go test ./internal/buffer/ -run TestAppendExceedsCapacity -v
    Expected Result: PASS. After 61 appends to max=60: Len=60, first candle is the 2nd one appended
    Evidence: .sisyphus/evidence/task-5-buffer-evict.txt

  Scenario: 10 goroutines read while 1 writes — no race
    Tool: Bash (go test -race)
    Steps:
      1. Run: go test -race ./internal/buffer/ -run TestConcurrent -v
    Expected Result: PASS. No "WARNING: DATA RACE" in output
    Evidence: .sisyphus/evidence/task-5-no-race.txt
  ```

  **Commit**: YES (groups with Tasks 4-8)
  - Message: `feat(backend): Binance websocket streaming and internal broadcast`

---

- [x] 6. **Internal WebSocket server (broadcast to frontend)** [TDD]

  **What to do**:
  - Create `go-backend/internal/server/ws.go`
  - Implement WebSocket server that:
    - Accepts connections on `/ws` endpoint
    - On new connection: sends current buffer snapshot + status message
    - Broadcasts new candles to ALL connected clients when candle channel receives data
    - Handles client disconnect gracefully (remove from client list)
    - Supports up to 10 concurrent clients
  - Message format (JSON):
    - `{"type":"status","status":"connected|reconnecting|disconnected","timestamp":...}`
    - `{"type":"kline","symbol":"BTCUSDT","interval":"1s","open":...,"high":...,"low":...,"close":...,"volume":...,"close_time":...,"timestamp":...}`
  - Write tests FIRST (TDD):
    - `TestNewClientReceivesBuffer` — new WS client gets snapshot
    - `TestBroadcastToAllClients` — candle broadcast → all clients receive
    - `TestClientDisconnect` — remove client, no broadcast errors
    - `TestConcurrentClients` — 5 clients connect/disconnect concurrently

  **Must NOT do**:
  - Do NOT send prediction messages yet (Wave 5)
  - Do NOT use `localhost` in any WebSocket URL (use config)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Concurrent WebSocket broadcasting with client management

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 4, 5, 7)
  - **Blocks**: Task 8 (wiring), Task 11 (frontend WS client)
  - **Blocked By**: Tasks 3, 5 (needs schemas + buffer)

  **References**:
  - `gorilla/websocket` server examples: Hub pattern for broadcasting to multiple clients
  - Go channel fan-out pattern: Multiple goroutines reading from same channel

  **Acceptance Criteria**:
  - [ ] `TestNewClientReceivesBuffer`: PASS
  - [ ] `TestBroadcastToAllClients`: PASS
  - [ ] `TestClientDisconnect`: PASS
  - [ ] `TestConcurrentClients`: PASS
  - [ ] `go test -race ./internal/server/`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Frontend receives candle via websocket
    Tool: Bash (websocat)
    Preconditions: Go backend running (localhost:8080)
    Steps:
      1. Run: timeout 10 websocat ws://localhost:8080/ws 2>&1 | head -5
      2. Parse first JSON message
    Expected Result: First message is {"type":"status","status":"connected",...}
    Expected Result: Within 5 seconds, receives {"type":"kline",...} message
    Evidence: .sisyphus/evidence/task-6-ws-messages.txt

  Scenario: Two clients both receive same candle
    Tool: Bash (websocat)
    Steps:
      1. Start 2 websocat clients in parallel
      2. Compare received candle close_time values
    Expected Result: Both clients receive candles with same close_time (broadcast works)
    Evidence: .sisyphus/evidence/task-6-broadcast.txt
  ```

  **Commit**: YES (groups with Tasks 4-8)

---

- [x] 7. **HTTP server (frontend serving + /health + graceful shutdown)** [TDD]

  **What to do**:
  - Create `go-backend/internal/server/http.go`
  - Serve static files from `frontend/` directory at `/`:
    - `GET /` → `frontend/index.html`
    - `GET /css/*` → `frontend/css/`
    - `GET /js/*` → `frontend/js/`
  - Implement `/health` endpoint:
    - `GET /health` → `{"status":"ok","binance":"connected|connecting|reconnecting|disconnected","buffer_size":<int>,"uptime_seconds":<int>}`
  - Implement graceful shutdown:
    - Listen for SIGINT/SIGTERM
    - Close Binance WS connection
    - Close all client WS connections
    - Stop accepting new HTTP requests
    - Exit with code 0 within 10 seconds
  - Write tests FIRST:
    - `TestHealthEndpoint` — GET /health returns 200 with valid JSON
    - `TestServeIndexHTML` — GET / returns HTML with correct Content-Type
    - `TestGracefulShutdown` — signal → all connections closed → exit 0

  **Must NOT do**:
  - Do NOT embed frontend files with `//go:embed` in MVP (serve from filesystem for easy dev iteration)
  - Do NOT serve directory listings

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: HTTP server with signal handling, multi-endpoint routing

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 4, 5, 6)
  - **Blocks**: Task 8 (wiring), Task 11 (frontend connects to this)
  - **Blocked By**: None directly (can serve dummy frontend during dev)

  **References**:
  - Go `net/http` package: Standard library HTTP server
  - Signal handling: `os/signal`, `syscall.SIGINT`, `syscall.SIGTERM`
  - Graceful shutdown pattern: `http.Server.Shutdown()` with context timeout

  **Acceptance Criteria**:
  - [ ] `TestHealthEndpoint`: PASS
  - [ ] `TestServeIndexHTML`: PASS
  - [ ] `TestGracefulShutdown`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Health endpoint returns correct status
    Tool: Bash (curl)
    Preconditions: Go backend running
    Steps:
      1. Run: curl -s http://localhost:8080/health | python3 -m json.tool
    Expected Result: JSON with keys: status, binance, buffer_size, uptime_seconds
    Expected Result: HTTP status 200
    Evidence: .sisyphus/evidence/task-7-health.json

  Scenario: Frontend HTML is served
    Tool: Bash (curl)
    Steps:
      1. Run: curl -s -I http://localhost:8080/
    Expected Result: Content-Type: text/html; charset=utf-8
    Expected Result: HTTP status 200
    Evidence: .sisyphus/evidence/task-7-frontend.txt

  Scenario: Graceful shutdown on SIGTERM
    Tool: Bash (docker compose)
    Preconditions: All services running
    Steps:
      1. Run: time docker compose stop go-backend
      2. Check exit code
    Expected Result: Container stops within 10 seconds, exit code 0
    Evidence: .sisyphus/evidence/task-7-shutdown.txt
  ```

  **Commit**: YES (groups with Tasks 4-8)

---

- [x] 8. **Wire everything together in main.go** [TDD]

  **What to do**:
  - Create `go-backend/cmd/predictor/main.go`
  - Initialize components in order:
    1. Create candle buffer (max 60)
    2. Start Binance WS client (goroutine, writes to candle channel)
    3. Start candle consumer (goroutine: reads candle channel → appends to buffer → triggers WS broadcast)
    4. Start HTTP server (serves frontend, /health, /ws)
  - Wire signal handling: SIGINT/SIGTERM → graceful shutdown sequence
  - Use `sync.WaitGroup` to track goroutines
  - Write integration test:
    - `TestMainStartupSequence` — all components initialize without error
    - `TestFullPipeline` — Binance candle → buffer → WS broadcast → client receives

  **Must NOT do**:
  - Do NOT add ML client logic yet (Wave 5)
  - Do NOT add any prediction-related code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Orchestration of concurrent components — moderate complexity

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential — depends on Tasks 4-7)
  - **Parallel Group**: Wave 1 final
  - **Blocks**: Task 19 (ML service needs backend running), Task 25 (integration)
  - **Blocked By**: Tasks 4, 5, 6, 7

  **References**:
  - Go `sync.WaitGroup`: Coordinating goroutine lifecycle
  - Go project layout: `cmd/` pattern for main entry points
  - Previous tasks in this wave: All internal packages must be importable

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/predictor/`: Compiles without errors
  - [ ] `TestMainStartupSequence`: PASS
  - [ ] `TestFullPipeline`: PASS
  - [ ] `go test -race ./cmd/...`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full pipeline from Binance to frontend client
    Tool: Bash (docker compose + websocat)
    Preconditions: docker compose up (all services)
    Steps:
      1. Run: timeout 15 websocat ws://localhost:8080/ws 2>&1
    Expected Result: Receives status message + at least 3 kline messages
    Expected Result: All kline messages have valid OHLCV fields
    Evidence: .sisyphus/evidence/task-8-full-pipeline.txt

  Scenario: Backend starts without ML service
    Tool: Bash (docker compose)
    Steps:
      1. Run: docker compose up go-backend (only)
      2. Run: curl -s http://localhost:8080/health
    Expected Result: Health returns status=ok, binance status is present
    Evidence: .sisyphus/evidence/task-8-no-ml.txt
  ```

  **Commit**: YES (groups with Tasks 4-8)
  - Message: `feat(backend): Binance websocket streaming and internal broadcast`

- [x] 9. **HTML page skeleton + dark theme CSS + status overlay** [Playwright TDD]

  **What to do**:
  - Create `frontend/index.html`:
    - Minimal structure: `<div id="chart-container">` (occupies 75%+ viewport), `<div id="status-bar">` (bottom, fixed), `<div id="header">` (asset name + timeframe label)
    - Load dependencies: TradingView Lightweight Charts (CDN or bundled), custom JS files
    - Meta viewport for proper scaling
  - Create `frontend/css/style.css`:
    - Dark theme (#1a1a2e background, #e0e0e0 text, #16213e containers)
    - Loader overlay: semi-transparent overlay with "Connecting..." text + spinner
    - Status bar styles: green (connected), yellow (reconnecting/collecting), red (disconnected/error)
    - No unnecessary UI elements — just chart + header + status
  - Write Playwright test FIRST:
    - `test_page_loads`: Navigate to page, assert chart container exists, assert dark background
    - `test_status_bar_visible`: Assert status bar at bottom with connecting state

  **Must NOT do**:
  - Do NOT add any buttons, dropdowns, or interactive controls beyond the chart
  - Do NOT add a light theme toggle
  - Do NOT add any TradingView chart code (Task 10)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: UI/UX design, dark theme, CSS layout — visual engineering domain

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 10, 11, 12)
  - **Blocks**: Task 13 (status indicators need HTML elements), Task 14 (chart renders in container)
  - **Blocked By**: Task 2 (needs frontend/ directory)

  **References**:
  - Dark theme color palettes for financial dashboards: TradingView dark theme, Bloomberg Terminal aesthetic
  - CSS Grid/Flexbox for full-viewport chart layout

  **Acceptance Criteria**:
  - [ ] `test_page_loads`: PASS (Playwright)
  - [ ] `test_status_bar_visible`: PASS (Playwright)
  - [ ] Page validates as valid HTML5 (`<!DOCTYPE html>`)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Page loads with dark theme and chart container
    Tool: Playwright
    Preconditions: Go backend serving frontend on localhost:8080
    Steps:
      1. Navigate to http://localhost:8080/
      2. Assert: element #chart-container exists and has height > 400px
      3. Assert: body background-color is dark (#1a1a2e or similar)
      4. Assert: no JavaScript console errors
    Expected Result: Dark page with large chart area, no errors
    Failure Indicators: White background, missing container, JS errors
    Evidence: .sisyphus/evidence/task-9-screenshot.png

  Scenario: Status bar shows connecting state on load
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080/
      2. Assert: element #status-bar contains text "Connecting" or "Collecting"
    Expected Result: Status bar visible with initial state text
    Evidence: .sisyphus/evidence/task-9-status.png
  ```

  **Commit**: YES (groups with Tasks 9-14)
  - Message: `feat(frontend): candlestick chart with real-time updates`
  - Pre-commit: `npx playwright test frontend-tests/`

---

- [x] 10. **TradingView chart initialization** [Playwright TDD]

  **What to do**:
  - Create `frontend/js/chart.js`:
    - Initialize `createChart()` with dark theme config:
      - Background: `#1a1a2e`, text color: `#e0e0e0`, grid lines: `#2a2a4e`
    - Add `CandlestickSeries` with:
      - Up color: `#26a69a` (green), down color: `#ef5350` (red)
      - Border invisible, wick colors match
    - Configure time scale: 60-second window, right offset for streaming
    - Configure price scale: auto-scale, no margin
    - Export `chart` and `candlestickSeries` for use by other modules
  - Write Playwright test FIRST:
    - `test_chart_canvas_exists`: Assert `<canvas>` element inside chart container

  **Must NOT do**:
  - Do NOT add any data to the chart yet (Task 12 handles real-time updates)
  - Do NOT add predicted candle series yet (Task 27)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: TradingView chart configuration, financial visualization — visual domain

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9, 11, 12)
  - **Blocks**: Task 12 (needs chart instance), Task 14 (needs series), Task 27 (overlay)
  - **Blocked By**: Task 9 (needs HTML container)

  **References**:
  - TradingView Lightweight Charts docs: `https://tradingview.github.io/lightweight-charts/docs` — API reference for `createChart`, `CandlestickSeries`
  - Dark theme chart config: `layout.background`, `grid.vertLines.color`, `grid.horzLines.color`

  **Acceptance Criteria**:
  - [ ] `test_chart_canvas_exists`: PASS (Playwright)
  - [ ] Chart renders with dark background (visual check via Playwright screenshot)
  - [ ] No JavaScript errors in console related to chart init

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Chart canvas renders on page
    Tool: Playwright
    Preconditions: Frontend served by Go backend
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 3 seconds for chart init
      3. Assert: canvas element exists inside #chart-container
      4. Assert: canvas has non-zero width and height
    Expected Result: Dark chart area with canvas element rendered
    Evidence: .sisyphus/evidence/task-10-chart.png
  ```

  **Commit**: YES (groups with Tasks 9-14)

---

- [x] 11. **Frontend WebSocket client** [Playwright TDD]

  **What to do**:
  - Create `frontend/js/ws-client.js`:
    - Connect to `ws://localhost:8080/ws` (configurable)
    - Parse incoming JSON messages by `type` field:
      - `status` → update status bar via callback
      - `kline` → invoke candle update callback with parsed candle data
      - `prediction` → invoke prediction callback (placeholder for Wave 5)
      - `indicator` → invoke indicator callback (placeholder for Wave 5)
    - Implement auto-reconnect on disconnect:
      - Backoff: 1s → 2s → 4s → 8s → max 30s
      - On reconnect: emit "reconnected" event, clear and re-receive buffer
    - Show "Reconnecting..." overlay during disconnect
    - Export: `connect(url)`, `onCandle(callback)`, `onStatus(callback)`, `onPrediction(callback)`
  - Write Playwright test FIRST (using mock WS server):
    - `test_parses_kline_message`: Send mock kline JSON → callback invoked with correct data
    - `test_handles_reconnect`: Simulate disconnect → verify reconnect attempt

  **Must NOT do**:
  - Do NOT implement chart update logic here (callback pattern — chart.js handles it)
  - Do NOT hardcode WS URL without making it configurable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: WebSocket client with reconnect logic — straightforward JS

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9, 10, 12)
  - **Blocks**: Task 12 (chart needs WS data), Task 13 (status updates)
  - **Blocked By**: Task 6 (WS server must exist for integration testing)

  **References**:
  - WebSocket API (MDN): `https://developer.mozilla.org/en-US/docs/Web/API/WebSocket` — Browser WebSocket client
  - Reconnection pattern: Exponential backoff with jitter

  **Acceptance Criteria**:
  - [ ] `test_parses_kline_message`: PASS
  - [ ] `test_handles_reconnect`: PASS
  - [ ] WebSocket connects to real backend (Playwright integration test)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Receives real kline data via WebSocket
    Tool: Playwright (with page.evaluate to inspect state)
    Preconditions: Go backend running with Binance connected
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 10 seconds
      3. Evaluate: window.__candleCount (must be set by app.js)
      4. Assert: candle count >= 3
    Expected Result: At least 3 candles received and stored
    Evidence: .sisyphus/evidence/task-11-candles-received.txt

  Scenario: Shows reconnecting overlay on disconnect
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait for connected state
      3. Stop Go backend (docker compose stop go-backend)
      4. Wait 5 seconds
      5. Assert: overlay with "Reconnecting" text is visible
    Expected Result: Reconnecting UI appears within 5 seconds of backend stop
    Evidence: .sisyphus/evidence/task-11-reconnecting.png
  ```

  **Commit**: YES (groups with Tasks 9-14)

---

- [x] 12. **Real-time chart update loop** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/chart.js` or create `frontend/js/app.js`:
    - Subscribe to WS client's `onCandle` callback
    - On each kline message:
      - Convert `close_time` to TradingView time format (Unix seconds)
      - Call `candlestickSeries.update({ time, open, high, low, close })`
      - If the candle `close_time` is new (not seen before), auto-scroll chart to keep "now" at right edge
    - Track received candle count for QA verification: `window.__candleCount`
  - Handle initial buffer: on WS connect, receive full buffer snapshot and call `candlestickSeries.setData()` for historical candles
  - Write Playwright test FIRST:
    - `test_candles_rendered_within_10_seconds`: Wait 10s, assert candle count > 3
    - `test_candle_count_increases`: Check count at t=10s vs t=20s, expect increase

  **Must NOT do**:
  - Do NOT call `setData()` repeatedly — use `update()` for streaming
  - Do NOT render any predictions or indicators yet

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Real-time chart rendering, TradingView API integration — visual domain

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9, 10, 11)
  - **Blocks**: Task 14 (indicator overlay), Task 27 (predicted overlay)
  - **Blocked By**: Tasks 10, 11 (needs chart instance + WS data)

  **References**:
  - TradingView `series.update()` vs `series.setData()`: `update()` for streaming single points, `setData()` for bulk
  - TradingView time format: Unix timestamp in seconds (not milliseconds)

  **Acceptance Criteria**:
  - [ ] `test_candles_rendered_within_10_seconds`: PASS
  - [ ] `test_candle_count_increases`: PASS
  - [ ] Chart scrolls to follow live data (visual check)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Live candles stream onto chart
    Tool: Playwright
    Preconditions: Go backend running, Binance connected
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 15 seconds
      3. Take screenshot
      4. Evaluate: window.__candleCount
    Expected Result: At least 5 candles visible on chart, count >= 5
    Evidence: .sisyphus/evidence/task-12-live-candles.png

  Scenario: Chart auto-scrolls with new data
    Tool: Playwright
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 30 seconds
      3. Assert: chart time scale shows recent timestamps (within last 30s)
    Expected Result: Chart visible range shows recent data, not stale
    Evidence: .sisyphus/evidence/task-12-autoscroll.png
  ```

  **Commit**: YES (groups with Tasks 9-14)

---

- [x] 13. **Startup state and status indicators** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/app.js`:
    - Subscribe to WS client's `onStatus` callback
    - Map status strings to UI states:
      - `connecting` → "Connecting to Binance..." (yellow)
      - `connected` (buffer < 60) → "Collecting data (X/60)" (yellow) with progress
      - `connected` (buffer >= 60) → "Live" (green) with dot indicator
      - `reconnecting` → "Reconnecting..." (yellow, pulsing)
      - `disconnected` → "Disconnected" (red)
      - `ml_unavailable` → "Prediction unavailable" (orange — Wave 5)
      - `ml_error` → "Prediction error" (red — Wave 5)
    - Update `#status-bar` text and CSS class
  - Write Playwright test FIRST:
    - `test_status_shows_collecting`: Assert "Collecting data" text before 60s
    - `test_status_shows_live`: Wait 70s, assert "Live" or green indicator

  **Must NOT do**:
  - Do NOT add prediction status handlers yet (placeholder only)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: UI state management, status bar rendering — visual domain

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9-12)
  - **Blocks**: Task 30 (ML-unavailable state)
  - **Blocked By**: Tasks 9, 11 (needs HTML elements + WS callbacks)

  **References**:
  - Status dot pulse animation (CSS): `@keyframes pulse` for live indicator

  **Acceptance Criteria**:
  - [ ] `test_status_shows_collecting`: PASS
  - [ ] `test_status_shows_live`: PASS
  - [ ] Status transitions are smooth (no flickering)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Status transitions through states on startup
    Tool: Playwright (+ video recording)
    Preconditions: Fresh start (docker compose up)
    Steps:
      1. Navigate to http://localhost:8080/
      2. Screenshot at t=0s (expect "Connecting")
      3. Screenshot at t=5s (expect "Collecting data" with count < 60)
      4. Screenshot at t=65s (expect "Live")
    Expected Result: Three distinct states captured, transitions clean
    Evidence: .sisyphus/evidence/task-13-status-t0.png
             .sisyphus/evidence/task-13-status-t5.png
             .sisyphus/evidence/task-13-status-t65.png
  ```

  **Commit**: YES (groups with Tasks 9-14)

---

- [x] 14. **Real SMA(20) line series rendering** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/app.js`:
    - Compute SMA(20) from candle buffer closes on each new candle
    - Add a `LineSeries` to the chart for SMA:
      - Color: `#ff9800` (orange)
      - Line width: 1px
      - Line style: `LineStyle.Solid`
    - Update SMA series with `lineSeries.update({ time, value })`
  - SMA formula: sum of last 20 closes / 20 (simple moving average)
  - Only render SMA when buffer has >= 20 candles
  - Write Playwright test FIRST:
    - `test_sma_line_visible_after_20_candles`: Wait for 20+ candles, assert line series data exists

  **Must NOT do**:
  - Do NOT compute SMA on the server side (client-side only for MVP)
  - Do NOT add predicted SMA yet (Task 28)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Chart indicator overlay, line series rendering — visual domain

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9-13)
  - **Blocks**: Task 28 (predicted SMA overlay)
  - **Blocked By**: Tasks 10, 12 (needs chart + candle data)

  **References**:
  - TradingView `LineSeries`: `chart.addSeries(LineSeries, options)` API
  - SMA calculation: `sum(closes[-20:]) / 20`

  **Acceptance Criteria**:
  - [ ] `test_sma_line_visible_after_20_candles`: PASS
  - [ ] SMA updates with each new candle (visual check: line moves)
  - [ ] SMA not rendered before 20 candles (no zero/NaN values)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SMA line appears and updates
    Tool: Playwright
    Preconditions: 25+ candles in buffer
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait until buffer >= 20
      3. Take screenshot
      4. Assert: orange line visible on chart (line series rendered)
    Expected Result: Orange SMA line overlaying candlestick chart
    Evidence: .sisyphus/evidence/task-14-sma-line.png
  ```

  **Commit**: YES (groups with Tasks 9-14)
  - Message: `feat(frontend): candlestick chart with real-time updates`

- [x] 15. **Historical data fetch from Binance REST API** [TDD]

  **What to do**:
  - Create `training/fetch_data.py`:
    - Fetch 30 days of 1s klines from Binance REST API:
      - Endpoint: `GET /api/v3/klines?symbol=BTCUSDT&interval=1s&limit=1000`
    - Loop with pagination: use `endTime` parameter to fetch earlier candles
    - Data saved as parquet or CSV: `training/data/btcusdt_1s_30d.parquet`
    - Validate data quality:
      - No gaps > 5 seconds (log warnings for gaps)
      - All OHLCV fields non-null
      - Timestamps are monotonically increasing
    - Expected volume: ~2.6M rows (30 days × 86400 seconds)
  - Write tests FIRST:
    - `test_fetch_single_batch`: Fetch 1 batch (1000 candles), verify shape and types
    - `test_fetch_multi_batch`: Fetch 2 batches, verify no duplicate close_times
    - `test_data_validation`: Inject gap artificially, verify warning logged

  **Must NOT do**:
  - Do NOT download during test runs — use mock/patch for unit tests
  - Do NOT train any model (Task 18)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Data pipeline with pagination, validation, large dataset handling

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 16, 17, 18)
  - **Blocks**: Task 16 (needs raw data for features)
  - **Blocked By**: None directly (runs offline)

  **References**:
  - Binance REST API klines: `https://binance-docs.github.io/apidocs/spot/en/#kline-candlestick-data` — Official docs for GET /api/v3/klines
  - Pandas parquet I/O: `pd.read_parquet()`, `df.to_parquet()` for efficient storage

  **Acceptance Criteria**:
  - [ ] `test_fetch_single_batch`: PASS
  - [ ] `test_fetch_multi_batch`: PASS
  - [ ] `test_data_validation`: PASS
  - [ ] Integration: `python3 fetch_data.py --days 1` downloads ~86K rows with valid data

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Fetches 1 day of data correctly
    Tool: Bash (python3)
    Preconditions: Internet connection
    Steps:
      1. Run: python3 training/fetch_data.py --days 1 --output training/data/test_1d.parquet
      2. Run: python3 -c "import pandas as pd; df=pd.read_parquet('training/data/test_1d.parquet'); print(len(df), df.columns.tolist())"
    Expected Result: ~86400 rows with columns: open_time, open, high, low, close, volume, close_time, ...
    Evidence: .sisyphus/evidence/task-15-fetch-output.txt

  Scenario: Handles API errors gracefully
    Tool: Bash
    Steps:
      1. Simulate bad symbol: python3 -c "from fetch_data import fetch_klines; fetch_klines('INVALID', '1s', 10)"
    Expected Result: Raises ValueError or logs error, does not crash with traceback
    Evidence: .sisyphus/evidence/task-15-error-handling.txt
  ```

  **Commit**: YES (groups with Tasks 15-18)
  - Message: `feat(training): historical data fetch and LightGBM model training`

---

- [x] 16. **Feature engineering pipeline (22 features)** [TDD]

  **What to do**:
  - Create `training/features.py`:
    - Engineer exactly 22 features from raw OHLCV data:
      1-5. Returns: 1s, 3s, 5s, 15s, 30s (close[t] / close[t-n] - 1)
      6-10. Log returns: ln(close[t] / close[t-n]) for same periods
      11. Volatility: std(returns_1s, window=30)
      12. Volume ratio: volume[t] / sma(volume, 20)
      13-14. High-Low ratio: (high-low)/close, rolling mean over 10 periods
      15. RSI(14): using pandas-ta-classic
      16. MACD histogram: using pandas-ta-classic
      17-18. Two candlestick patterns from pandas-ta-classic: CDLDOJI, CDLENGULFING (binary 0/1)
      19. SMA(20) deviation: (close - sma_20) / sma_20
      20. Price position: (close - low_20) / (high_20 - low_20) [0-1 range]
      21. Candle body ratio: abs(close-open) / (high-low) per candle
      22. Trade count proxy: volume / sma(volume, 10) [activity ratio]
    - Ensure NO look-ahead bias: feature at time t uses ONLY data from indices <= t
    - Handle NaN/Inf: forward-fill, then drop remaining NaN rows
  - Write tests FIRST:
    - `test_feature_count`: Verify output has exactly 22 feature columns
    - `test_no_lookahead`: For random index i, feature row i uses only data from indices <= i
    - `test_no_nan_inf`: Output has zero NaN or Inf values

  **Must NOT do**:
  - Do NOT use random shuffle at any point in the pipeline
  - Do NOT include the target/label column in features (separate step — Task 17)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Feature engineering for financial ML, pandas-ta integration, look-ahead bias prevention

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 15, 17, 18)
  - **Blocks**: Task 17 (needs features for labeling), Task 18 (needs features for training)
  - **Blocked By**: Task 15 (needs raw data)

  **References**:
  - `pandas-ta-classic`: `df.ta.rsi(length=14)`, `df.ta.macd()`, `df.ta.cdl_pattern(name="doji")`
  - Look-ahead bias prevention: `df.shift(-n)` is FORBIDDEN for features. Use `df.shift(n)` to lag.

  **Acceptance Criteria**:
  - [ ] `test_feature_count`: PASS
  - [ ] `test_no_lookahead`: PASS
  - [ ] `test_no_nan_inf`: PASS
  - [ ] Integration: `python3 features.py --input data/test_1d.parquet --output data/features_1d.parquet` produces valid output

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Feature matrix has correct shape and no look-ahead
    Tool: Bash (python3 + pytest)
    Steps:
      1. Run: pytest training/tests/test_features.py -v
      2. Verify: test_no_lookahead checks that feature[t] doesn't peek at future data
    Expected Result: All 3 tests pass, especially look-ahead check
    Evidence: .sisyphus/evidence/task-16-feature-tests.txt

  Scenario: Features are computable on small test dataset
    Tool: Bash
    Steps:
      1. Run: python3 -c "
    from features import engineer_features
    import pandas as pd
    import numpy as np
    df = pd.DataFrame({'open': np.random.randn(200)*100+50000, 'high': ..., 'low': ..., 'close': ..., 'volume': ...})
    features = engineer_features(df)
    print(features.shape, features.columns.tolist())
    "
    Expected Result: Shape (~180, 22), all 22 column names printed
    Evidence: .sisyphus/evidence/task-16-feature-output.txt
  ```

  **Commit**: YES (groups with Tasks 15-18)

---

- [x] 17. **Label creation + walk-forward validation** [TDD]

  **What to do**:
  - Create `training/labels.py` and `training/validate.py`:
    - **Labels**: For each timestamp t, label = 1 if close[t+30] > close[t] (UP), else 0 (DOWN)
      - This is the 30-second-forward direction
      - Drop last 30 rows (no future data available)
    - **Walk-forward validation**:
      - Use `sklearn.model_selection.TimeSeriesSplit(n_splits=5)`
      - Each split: train on earlier data, test on later data
      - NEVER random shuffle
    - Validate splits: for each split, max(train_indices) < min(test_indices)
  - Write tests FIRST:
    - `test_label_direction`: Manual candles where close rises → label=1, falls → label=0
    - `test_no_future_leak`: Verify labels don't use data beyond t+30
    - `test_timeseriessplit_order`: All test indices are after all train indices for every split

  **Must NOT do**:
  - Do NOT use `train_test_split` (random) — use ONLY `TimeSeriesSplit`
  - Do NOT stratify by label (destroys temporal order)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Time-series specific ML validation, temporal leakage prevention

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 15, 16, 18)
  - **Blocks**: Task 18 (needs labeled data for training)
  - **Blocked By**: Task 16 (needs feature matrix)

  **References**:
  - `sklearn.model_selection.TimeSeriesSplit`: Time-aware cross-validation
  - Label construction: `(df['close'].shift(-30) > df['close']).astype(int)` then drop last 30

  **Acceptance Criteria**:
  - [ ] `test_label_direction`: PASS
  - [ ] `test_no_future_leak`: PASS
  - [ ] `test_timeseriessplit_order`: PASS
  - [ ] Label distribution is roughly balanced (40-60% either class)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Labels are temporally valid
    Tool: Bash (pytest)
    Steps:
      1. Run: pytest training/tests/test_labels.py -v
      2. Check test_no_future_leak: ensures label[t] doesn't use close[t+31] or beyond
    Expected Result: All tests pass, temporal integrity confirmed
    Evidence: .sisyphus/evidence/task-17-label-tests.txt

  Scenario: Walk-forward splits are strictly ordered
    Tool: Bash (pytest)
    Steps:
      1. Run: pytest training/tests/test_validate.py -v
    Expected Result: test_timeseriessplit_order confirms no temporal leakage
    Evidence: .sisyphus/evidence/task-17-validation-tests.txt
  ```

  **Commit**: YES (groups with Tasks 15-18)

---

- [x] 18. **LightGBM training + serialization + metrics** [TDD]

  **What to do**:
  - Create `training/train.py`:
    - Load features and labels
    - Train LightGBM classifier with conservative params:
      - `objective='binary'`, `metric='auc'`
      - `num_leaves=15`, `max_depth=6`, `learning_rate=0.05`
      - `lambda_l1=1.0`, `lambda_l2=1.0` (high regularization to prevent overfitting)
      - `min_data_in_leaf=100`, `feature_fraction=0.8`
    - Walk-forward validation: train on fold 1, evaluate on fold 2, etc.
    - Report metrics per fold:
      - AUC (target: > 0.52, realistic: 0.53-0.58 for crypto)
      - Accuracy, Precision, Recall
    - Serialize model: `model.booster_.save_model('training/models/model.txt')`
    - Also save feature names: `training/models/feature_names.json`
  - Write tests FIRST:
    - `test_model_trains_without_error`: Train on small synthetic data, verify no exceptions
    - `test_model_serialization_roundtrip`: Save → load → predict → compare (identical outputs)
    - `test_auc_above_random`: On any real fold, AUC > 0.50 (better than coin flip)

  **Must NOT do**:
  - Do NOT use `random_state` with shuffle-based methods
  - Do NOT output predictions to any file other than model.txt + feature_names.json

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: ML model training with validation, hyperparameter reasoning, serialization — requires deep understanding

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 15, 16, 17)
  - **Blocks**: Task 20 (ML service loads model)
  - **Blocked By**: Task 17 (needs labeled data)

  **References**:
  - LightGBM docs: `https://lightgbm.readthedocs.io/en/latest/Parameters.html` — Parameter reference
  - `model.booster_.save_model()`: Native LightGBM serialization format

  **Acceptance Criteria**:
  - [ ] `test_model_trains_without_error`: PASS
  - [ ] `test_model_serialization_roundtrip`: PASS
  - [ ] `test_auc_above_random`: PASS
  - [ ] `model.txt` file exists and is > 10KB (not empty)
  - [ ] `feature_names.json` lists exactly 22 feature names

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Model trains on 30 days of data and achieves AUC > 0.52
    Tool: Bash (python3)
    Preconditions: 30-day parquet file exists
    Steps:
      1. Run: python3 training/train.py --data training/data/btcusdt_1s_30d.parquet --output training/models/
      2. Check exit code and stdout for metrics
    Expected Result: AUC per fold printed, mean AUC > 0.52, model.txt created
    Evidence: .sisyphus/evidence/task-18-training-metrics.txt

  Scenario: Serialized model produces identical predictions
    Tool: Bash (pytest)
    Steps:
      1. Run: pytest training/tests/test_train.py::test_model_serialization_roundtrip -v
    Expected Result: PASS — identical predictions from original and reloaded model
    Evidence: .sisyphus/evidence/task-18-serialization.txt
  ```

  **Commit**: YES (groups with Tasks 15-18)
  - Message: `feat(training): historical data fetch and LightGBM model training`

- [ ] 19. **FastAPI project setup + Dockerfile + uvicorn config** [TDD]

  **What to do**:
  - Create `ml-service/app/main.py`:
    - FastAPI app with lifespan: load model on startup, clean up on shutdown
    - Use `ORJSONResponse` as default response class (faster serialization)
    - Add `/health` endpoint (shallow — returns status without inference)
    - Configure CORS (allow localhost origins for dev)
  - Create `ml-service/Dockerfile`:
    - Base: `python:3.12-slim`
    - Install system deps for LightGBM: `libgomp1`, `libstdc++6`
    - Copy `requirements.txt`, install pip packages
    - Copy model file: `training/models/model.txt` → `/app/models/model.txt`
    - CMD: `uvicorn app.main:app --host 0.0.0.0 --port 8000 --workers 1`
  - Create `ml-service/app/config.py`:
    - MODEL_PATH: env var with default `/app/models/model.txt`
    - FEATURE_NAMES_PATH: env var with default `/app/models/feature_names.json`
  - Write tests FIRST:
    - `test_app_starts`: FastAPI TestClient → GET /health → 200
    - `test_dockerfile_builds`: Verify Dockerfile syntax valid

  **Must NOT do**:
  - Do NOT use `async def` for CPU-bound endpoints (Task 22 uses `def`)
  - Do NOT load model more than once (lifespan startup)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: FastAPI configuration, Docker setup, lifespan pattern — moderate complexity

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 20, 21, 22)
  - **Blocks**: Task 20 (needs app for health endpoint), Task 22 (needs app for predict)
  - **Blocked By**: Task 2 (needs directory structure)

  **References**:
  - FastAPI lifespan: `@asynccontextmanager` pattern for startup/shutdown
  - `ORJSONResponse`: `from fastapi.responses import ORJSONResponse`
  - Uvicorn config: Single worker (CPU-bound ML), `--host 0.0.0.0` for Docker networking

  **Acceptance Criteria**:
  - [ ] `test_app_starts`: PASS (GET /health returns 200)
  - [ ] `test_dockerfile_builds`: PASS
  - [ ] `docker build -t ml-service ml-service/` succeeds

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: FastAPI app starts and health endpoint works
    Tool: Bash (curl)
    Preconditions: ml-service built and running
    Steps:
      1. Run: curl -s http://localhost:8000/health
    Expected Result: HTTP 200, JSON response with "status" key
    Evidence: .sisyphus/evidence/task-19-health.json

  Scenario: Docker image builds successfully
    Tool: Bash (docker build)
    Steps:
      1. Run: docker build -t ml-service-test ml-service/ 2>&1
    Expected Result: Build completes without errors, image listed in docker images
    Evidence: .sisyphus/evidence/task-19-docker-build.txt
  ```

  **Commit**: YES (groups with Tasks 19-24)
  - Message: `feat(ml): FastAPI prediction service with LightGBM`

---

- [ ] 20. **Model loading at startup + /health with model_loaded flag** [TDD]

  **What to do**:
  - Create `ml-service/app/model.py`:
    - `load_model(path)` function: loads LightGBM model from .txt file, loads feature names from JSON
    - Store model in module-level variable (loaded once at startup)
    - `get_model()` function: returns loaded model or raises if not loaded
  - Update `ml-service/app/main.py` lifespan:
    - Call `load_model()` on startup
    - Set `app.state.model_loaded = True` on success
    - Set `app.state.model_loaded = False` on failure (graceful degradation)
  - Update `/health` endpoint:
    - Return `{"status": "ok" if model_loaded else "degraded", "model_loaded": bool, "model_path": str, "feature_count": int}`
  - Write tests FIRST:
    - `test_model_loads_from_file`: Provide valid model.txt → model_loaded=True
    - `test_model_file_missing`: Model path doesn't exist → model_loaded=False, /health returns degraded
    - `test_feature_names_loaded`: Feature count matches expected (22)

  **Must NOT do**:
  - Do NOT load model on every request — load once at startup only
  - Do NOT crash the app if model file is missing — degrade gracefully

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Model lifecycle management, graceful degradation pattern

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential — depends on Task 19)
  - **Parallel Group**: Wave 4 early
  - **Blocks**: Task 22 (needs get_model for predict)
  - **Blocked By**: Task 19 (needs app structure)

  **References**:
  - LightGBM Python API: `lightgbm.Booster(model_file='model.txt')`
  - FastAPI `app.state`: Store arbitrary state (model, config) accessible across requests

  **Acceptance Criteria**:
  - [ ] `test_model_loads_from_file`: PASS
  - [ ] `test_model_file_missing`: PASS (returns degraded, not crash)
  - [ ] `test_feature_names_loaded`: PASS
  - [ ] GET /health returns `model_loaded: true` when model exists

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Health shows model loaded when model.txt present
    Tool: Bash (curl)
    Preconditions: model.txt in training/models/
    Steps:
      1. Run: curl -s http://localhost:8000/health | python3 -m json.tool
    Expected Result: {"status":"ok","model_loaded":true,"feature_count":22,...}
    Evidence: .sisyphus/evidence/task-20-health-loaded.json

  Scenario: Health shows degraded when model missing
    Tool: Bash (curl)
    Preconditions: MODEL_PATH points to non-existent file
    Steps:
      1. Run: curl -s http://localhost:8000/health
    Expected Result: {"status":"degraded","model_loaded":false,...}
    Evidence: .sisyphus/evidence/task-20-health-degraded.json
  ```

  **Commit**: YES (groups with Tasks 19-24)

---

- [ ] 21. **TA computation (indicators via pandas-ta-classic)** [TDD]

  **What to do**:
  - Create `ml-service/app/indicators.py`:
    - `compute_features(candles: list[dict]) -> dict[str, float]`:
      - Accepts list of 60 candle dicts with OHLCV
      - Converts to pandas DataFrame
      - Computes the same 22 features as `training/features.py` (Task 16)
      - Returns dict mapping feature name → value for the most recent candle
    - `compute_sma(candles: list[dict], period: int = 20) -> float`:
      - Simple moving average of closes
    - `build_predicted_candle(direction: str, confidence: float, last_candle: dict) -> dict`:
      - If UP: predicted close = last_close * (1 + confidence * 0.001)
      - If DOWN: predicted close = last_close * (1 - confidence * 0.001)
      - If UNCERTAIN: predicted close = last_close
      - Open = last_close, High = max(last_close, predicted_close) * 1.0005, Low = min(...)
  - Write tests FIRST:
    - `test_compute_features_output`: 60 candles → 22 feature values, no NaN
    - `test_compute_sma`: Known values → correct SMA
    - `test_build_predicted_candle_up`: Direction=UP → predicted_close > last_close
    - `test_build_predicted_candle_down`: Direction=DOWN → predicted_close < last_close
    - `test_build_predicted_candle_uncertain`: Direction=UNCERTAIN → predicted_close == last_close

  **Must NOT do**:
  - Do NOT import `numta` — use `pandas-ta-classic` only
  - Do NOT hardcode SMA period (make it configurable, default 20)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Financial indicator computation, pandas-ta integration

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19, 20, 22)
  - **Blocks**: Task 22 (predict needs features)
  - **Blocked By**: Task 3 (needs schema)

  **References**:
  - `pandas-ta-classic` SMA: `df.ta.sma(length=20)` returns Series
  - Feature parity requirement: Must produce same features as `training/features.py`

  **Acceptance Criteria**:
  - [ ] `test_compute_features_output`: PASS
  - [ ] `test_compute_sma`: PASS
  - [ ] `test_build_predicted_candle_up`: PASS
  - [ ] `test_build_predicted_candle_down`: PASS
  - [ ] `test_build_predicted_candle_uncertain`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Features computed from 60 candles match expected shape
    Tool: Bash (pytest)
    Steps:
      1. Run: pytest ml-service/tests/test_indicators.py -v
    Expected Result: All 5 tests pass
    Evidence: .sisyphus/evidence/task-21-indicator-tests.txt

  Scenario: UP prediction produces higher close
    Tool: Bash (python3)
    Steps:
      1. Run: python3 -c "
    from app.indicators import build_predicted_candle
    candle = build_predicted_candle('UP', 0.75, {'close': 50000})
    assert candle['close'] > 50000, f'Expected > 50000, got {candle[\"close\"]}'
    print('PASS: UP prediction close > last close')
    "
    Expected Result: "PASS: UP prediction close > last close"
    Evidence: .sisyphus/evidence/task-21-predicted-candle.txt
  ```

  **Commit**: YES (groups with Tasks 19-24)

---

- [ ] 22. **/predict endpoint (validation, inference, threshold)** [TDD]

  **What to do**:
  - Create `ml-service/app/routes.py` (or update `main.py`):
    - `POST /predict` endpoint:
      - Accepts: `PredictionInput` — list of 60 candles + optional feature override
      - Validates: exactly 60 candles, all required fields present, no NaN in values
      - Computes features via `indicators.compute_features()`
      - Runs LightGBM inference: `model.predict(features_array)` → probability
      - Applies confidence threshold:
        - probability > 0.55 → `direction = "UP"`
        - probability < 0.45 → `direction = "DOWN"`
        - 0.45 <= probability <= 0.55 → `direction = "UNCERTAIN"`
      - Builds predicted candle via `indicators.build_predicted_candle()`
      - Returns: `PredictionOutput` with direction, confidence, predicted_candle, predicted_ma
    - Use `def` (NOT `async def`) for CPU-bound inference
  - Write tests FIRST:
    - `test_predict_valid_input`: Valid 60 candles → 200 with valid PredictionOutput
    - `test_predict_insufficient_candles`: 10 candles → 422 with error detail
    - `test_predict_nan_in_input`: NaN close value → 422
    - `test_predict_up_threshold`: probability 0.60 → direction UP
    - `test_predict_down_threshold`: probability 0.30 → direction DOWN
    - `test_predict_uncertain`: probability 0.50 → direction UNCERTAIN
    - `test_predict_no_model`: Model not loaded → 503

  **Must NOT do**:
  - Do NOT use `async def` for this endpoint (blocks event loop)
  - Do NOT accept fewer than 60 candles (return 422)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: ML inference endpoint with threshold logic, error taxonomy, performance considerations

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 19, 20, 21)
  - **Blocks**: Task 23 (predicted candle), Task 25 (Go ML client)
  - **Blocked By**: Tasks 20, 21 (needs model + indicators)

  **References**:
  - FastAPI `def` vs `async def`: `def` runs in thread pool, not blocking event loop
  - LightGBM `model.predict()`: Returns probability array for binary classification
  - Pydantic validation: `PredictionInput` with `@field_validator` for 60-candle constraint

  **Acceptance Criteria**:
  - [ ] `test_predict_valid_input`: PASS
  - [ ] `test_predict_insufficient_candles`: PASS
  - [ ] `test_predict_nan_in_input`: PASS
  - [ ] `test_predict_up_threshold`: PASS
  - [ ] `test_predict_down_threshold`: PASS
  - [ ] `test_predict_uncertain`: PASS
  - [ ] `test_predict_no_model`: PASS
  - [ ] Response time < 200ms (P95 target)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Predict returns UP for bullish features
    Tool: Bash (curl)
    Preconditions: ML service running, model loaded
    Steps:
      1. Run: curl -s -X POST http://localhost:8000/predict \
         -H "Content-Type: application/json" \
         -d '{"candles": [...]}'  # 60 candles with rising trend
      2. Parse response JSON
    Expected Result: HTTP 200, response contains direction, confidence, predicted_candle, predicted_ma
    Expected Result: direction is one of: "UP", "DOWN", "UNCERTAIN"
    Evidence: .sisyphus/evidence/task-22-predict-response.json

  Scenario: Invalid input returns 422
    Tool: Bash (curl)
    Steps:
      1. Run: curl -s -w "\n%{http_code}" -X POST http://localhost:8000/predict \
         -H "Content-Type: application/json" \
         -d '{"candles": [{"open": 1}]}'  # Only 1 candle
    Expected Result: HTTP 422, JSON detail contains "Expected 60 candles"
    Evidence: .sisyphus/evidence/task-22-validation-error.txt
  ```

  **Commit**: YES (groups with Tasks 19-24)

---

- [ ] 23. **Predicted candle construction from direction and confidence** [TDD — included in Task 22 tests]

  **What to do**:
  - This is part of Task 22 — the `build_predicted_candle()` function in `indicators.py`
  - Ensure predicted candle has realistic values:
    - Predicted close moves proportionally to confidence (max ±0.1% for UP/DOWN at confidence=1.0)
    - Predicted high/low account for volatility
  - Integration: predicted_candle is included in `/predict` response

  **Must NOT do**:
  - Do NOT predict more than 1 candle (30-second prediction = 1 candle in 1s timeframe = 30 candles... wait.)
  - **CLARIFICATION**: 30-second prediction = predicting 30 x 1-second candles, OR predicting the direction of price after 30 seconds?
  - **Resolution**: The model predicts the **direction** at t+30 (UP if close[t+30] > close[t]). The predicted candle rendered is a **single synthetic candle** representing the expected state 30 seconds from now, with OHLC derived from direction + confidence. The 30 individual 1s candles in between are NOT predicted (too noisy).

  **Recommended Agent Profile**: Same as Task 22 (`deep`)

  **Parallelization**: Part of Task 22

  **Acceptance Criteria**: Same as Task 22

  **QA Scenarios**: Same as Task 22

  **Commit**: YES (groups with Tasks 19-24)

---

- [ ] 24. **Error handling + graceful degradation (422, 503, timeout)** [TDD]

  **What to do**:
  - Update `ml-service/app/main.py`:
    - Add exception handlers:
      - `ModelNotLoadedError` → 503 `{"error": "model_not_loaded", "detail": "Prediction model is not available"}`
      - `ValueError` (validation) → 422 with field-level details
      - `TimeoutError` → 504 `{"error": "inference_timeout", "detail": "Prediction took too long"}`
      - Unhandled exceptions → 500 (logged, generic response to client)
    - Add request timeout middleware: 500ms per prediction request
  - Add logging: every prediction request logs latency, direction, confidence
  - Write tests FIRST:
    - `test_predict_model_not_loaded`: GET /health shows degraded, POST /predict → 503
    - `test_predict_timeout`: Mock slow model → 504 response
    - `test_unhandled_exception`: Raise unexpected error → 500 + log entry

  **Must NOT do**:
  - Do NOT expose internal stack traces to the client
  - Do NOT hard-fail on timeout — return 504, not crash

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Error handling patterns, middleware, logging — production hardening

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Tasks 19-22)
  - **Parallel Group**: Wave 4 final
  - **Blocks**: Task 30 (frontend degradation)
  - **Blocked By**: Tasks 19, 20, 22

  **References**:
  - FastAPI exception handlers: `@app.exception_handler(ModelNotLoadedError)`
  - Starlette middleware: `BaseHTTPMiddleware` for timeout enforcement

  **Acceptance Criteria**:
  - [ ] `test_predict_model_not_loaded`: PASS
  - [ ] `test_predict_timeout`: PASS
  - [ ] `test_unhandled_exception`: PASS
  - [ ] All error responses follow consistent JSON schema: `{"error": "...", "detail": "..."}`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 503 when model not loaded
    Tool: Bash (curl)
    Preconditions: MODEL_PATH points to missing file (model not loaded)
    Steps:
      1. Run: curl -s -w "\n%{http_code}" -X POST http://localhost:8000/predict \
         -H "Content-Type: application/json" \
         -d '{"candles": [...]}'
    Expected Result: HTTP 503, JSON body contains error="model_not_loaded"
    Evidence: .sisyphus/evidence/task-24-503-response.txt

  Scenario: 422 on malformed input
    Tool: Bash (curl)
    Steps:
      1. Run: curl -s -w "\n%{http_code}" -X POST http://localhost:8000/predict \
         -H "Content-Type: application/json" \
         -d '{"wrong_field": true}'
    Expected Result: HTTP 422 with validation error details
    Evidence: .sisyphus/evidence/task-24-422-response.txt
  ```

  **Commit**: YES (groups with Tasks 19-24)
  - Message: `feat(ml): FastAPI prediction service with LightGBM`

- [ ] 25. **Go ↔ ML service HTTP client** [TDD]

  **What to do**:
  - Create `go-backend/internal/mlclient/client.go`:
    - HTTP client configured with:
      - Base URL: `http://ml-service:8000` (Docker service name, NOT localhost)
      - Timeout: 500ms (matches ML service timeout)
      - Retry: 2 retries with 100ms delay (only on 503/504, not on 422)
    - `Predict(candles []schemas.Candle) (*schemas.PredictionOutput, error)`:
      - Serializes candles to JSON matching `PredictionInput` schema
      - POST to `/predict`
      - Deserializes response to `PredictionOutput`
      - Returns typed error on HTTP error, timeout, or parse failure
    - `HealthCheck() (bool, error)`:
      - GET `/health`
      - Returns `model_loaded` status
  - Write tests FIRST (using `httptest` mock server):
    - `TestPredictSuccess`: ML returns valid response → parsed correctly
    - `TestPredictMLUnavailable`: ML returns 503 → error with "model_not_loaded"
    - `TestPredictTimeout`: ML slow → context deadline exceeded error
    - `TestHealthCheckModelLoaded`: /health returns model_loaded=true → HealthCheck returns true

  **Must NOT do**:
  - Do NOT hardcode `localhost` in client URL — use config/env
  - Do NOT block the candle pipeline on prediction (async pattern: fire and forget, or channel-based)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: HTTP client with retry logic, error taxonomy, Docker networking awareness

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 26-30)
  - **Blocks**: Task 26 (prediction broadcast needs client)
  - **Blocked By**: Task 3 (needs schemas), Task 22 (ML service must exist)

  **References**:
  - Go `net/http` client: `http.Client` with `Timeout` field
  - `httptest.NewServer`: Mock HTTP server for Go tests
  - Docker networking: Service names resolve via Docker DNS (`ml-service` → container IP)

  **Acceptance Criteria**:
  - [ ] `TestPredictSuccess`: PASS
  - [ ] `TestPredictMLUnavailable`: PASS
  - [ ] `TestPredictTimeout`: PASS
  - [ ] `TestHealthCheckModelLoaded`: PASS
  - [ ] `go test -race ./internal/mlclient/`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go client successfully calls ML service in Docker
    Tool: Bash (docker compose + curl)
    Preconditions: Both services running in Docker
    Steps:
      1. Run: docker compose exec go-backend sh -c "wget -qO- http://ml-service:8000/health"
    Expected Result: JSON response with model_loaded: true
    Evidence: .sisyphus/evidence/task-25-go-ml-health.txt

  Scenario: Predict roundtrip via Go client
    Tool: Bash (go test with integration tag)
    Steps:
      1. Run: go test -tags=integration ./internal/mlclient/ -run TestPredictRoundtrip -v
    Expected Result: Go sends 60 candles to ML, receives valid prediction
    Evidence: .sisyphus/evidence/task-25-predict-roundtrip.txt
  ```

  **Commit**: YES (groups with Tasks 25-30)
  - Message: `feat(integration): predicted candles and indicators on chart`

---

- [ ] 26. **Prediction broadcast to frontend WebSocket** [TDD]

  **What to do**:
  - Update `go-backend/internal/server/ws.go`:
    - Add new goroutine: prediction broadcaster
    - Every time buffer reaches 60 candles:
      1. Call `mlClient.Predict(buffer.Snapshot())`
      2. On success: broadcast `PredictionMessage` JSON to all WS clients
      3. On error: broadcast status message `{"type":"status","status":"ml_error"}`
    - **Serialization**: Only send 1 prediction request at a time (mutex or channel)
    - **Stale prediction**: If new candle arrives while prediction in-flight, send new request (cancel/discard old)
  - Add new message type to broadcast:
    ```json
    {
      "type": "prediction",
      "direction": "UP",
      "confidence": 0.72,
      "predicted_candle": {"open": 51200.0, "high": 51250.0, "low": 51150.0, "close": 51235.0},
      "predicted_ma": 51180.5,
      "timestamp": 1714857600
    }
    ```
  - Write tests FIRST:
    - `TestPredictionBroadcastOnBufferFull`: Buffer hits 60 → prediction sent to WS clients
    - `TestPredictionNotSentWhenBufferNotFull`: Buffer at 30 → no prediction call
    - `TestMLErrorBroadcastsStatus`: ML returns error → status message broadcast

  **Must NOT do**:
  - Do NOT send prediction to clients that joined mid-stream without the full buffer context
  - Do NOT block candle streaming while prediction is in-flight

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Concurrent prediction orchestration, fan-out broadcast — moderate complexity

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25, 27-30)
  - **Blocks**: Task 27 (frontend needs prediction messages)
  - **Blocked By**: Task 25 (needs ML client), Task 6 (needs WS server)

  **References**:
  - Channel-based request serialization: Single goroutine consuming from `chan predictRequest`
  - Stale request handling: Use context cancellation to abort in-flight HTTP calls

  **Acceptance Criteria**:
  - [ ] `TestPredictionBroadcastOnBufferFull`: PASS
  - [ ] `TestPredictionNotSentWhenBufferNotFull`: PASS
  - [ ] `TestMLErrorBroadcastsStatus`: PASS
  - [ ] `go test -race ./internal/server/`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Prediction appears in websocket after 60 candles
    Tool: Bash (websocat)
    Preconditions: All services running, Binance connected, ML service healthy
    Steps:
      1. Run: timeout 90 websocat ws://localhost:8080/ws 2>&1 | grep '"type":"prediction"' | head -1
    Expected Result: Within 80 seconds, receives prediction message with direction, confidence, predicted_candle
    Evidence: .sisyphus/evidence/task-26-prediction-ws.json

  Scenario: ML error broadcasts status correctly
    Tool: Bash (websocat)
    Steps:
      1. Stop ML service: docker compose stop ml-service
      2. Run: timeout 20 websocat ws://localhost:8080/ws 2>&1 | grep '"status":"ml_error"'
    Expected Result: Receives ml_error status message within 15 seconds
    Evidence: .sisyphus/evidence/task-26-ml-error.txt
  ```

  **Commit**: YES (groups with Tasks 25-30)

---

- [ ] 27. **Frontend dual CandlestickSeries (real + predicted overlay)** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/chart.js`:
    - Add second `CandlestickSeries` for predicted candles:
      - Use `priceScaleId: ''` to overlay on same price scale as real candles
      - Colors with RGBA transparency:
        - Up: `rgba(38, 166, 154, 0.5)` (green, 50% opacity)
        - Down: `rgba(239, 83, 80, 0.5)` (red, 50% opacity)
        - Wick up: `rgba(38, 166, 154, 0.4)`
        - Wick down: `rgba(239, 83, 80, 0.4)`
    - Insert `WhitespaceData` gap between last real candle and predicted candle:
      - Add a `{ time: gapTime }` entry with no OHLC values
    - Subscribe to WS `onPrediction` callback:
      - Clear existing predicted series data
      - Set predicted candle at `now + 30s` time position
  - Write Playwright test FIRST:
    - `test_predicted_candle_visible_after_70_seconds`: Wait 70s, assert second candlestick series has data
    - `test_predicted_candle_has_different_color`: Visual check — predicted candles look semi-transparent

  **Must NOT do**:
  - Do NOT attempt per-candle opacity (unsupported) — use dual series with RGBA
  - Do NOT render predicted candles before receiving first prediction

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Dual chart series overlay, visual styling, TradingView advanced API

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25, 26, 28-30)
  - **Blocks**: Task 29 (confidence coloring)
  - **Blocked By**: Tasks 10, 26 (needs chart instance + prediction messages)

  **References**:
  - TradingView `priceScaleId`: Use `''` (empty string) to share price scale across series
  - `WhitespaceData`: `{ time: timestamp }` with no OHLC fields creates chart gap
  - RGBA color format: `rgba(R, G, B, A)` where A=0.5 for semi-transparent

  **Acceptance Criteria**:
  - [ ] `test_predicted_candle_visible_after_70_seconds`: PASS
  - [ ] `test_predicted_candle_has_different_color`: PASS
  - [ ] Visual gap between real and predicted candles exists

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Predicted candle renders semi-transparently after real data
    Tool: Playwright
    Preconditions: Platform running for 70+ seconds, at least 1 prediction received
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 80 seconds (60s buffer + 20s for prediction to arrive and render)
      3. Take screenshot
      4. Evaluate: predictedSeries.data().length > 0
    Expected Result: At least 1 predicted candle visible on chart
    Expected Result: Predicted candle visually distinguishable from real candles (different color/opacity)
    Evidence: .sisyphus/evidence/task-27-predicted-candle.png
  ```

  **Commit**: YES (groups with Tasks 25-30)

---

- [ ] 28. **Frontend predicted SMA line (dotted LineStyle)** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/chart.js`:
    - Add second `LineSeries` for predicted SMA:
      - Color: `rgba(255, 152, 0, 0.5)` (orange, 50% opacity)
      - Line width: 1px
      - Line style: `LineStyle.Dashed` (dotted)
    - Subscribe to WS `onPrediction` callback:
      - Update predicted SMA at `now + 30s` time position with `predicted_ma` value
    - Also add the last real SMA value at `now` time for visual continuity
  - Write Playwright test FIRST:
    - `test_predicted_sma_visible`: Wait for prediction, assert dashed line series has data
    - `test_predicted_sma_different_style`: Assert predicted SMA has different line style than real SMA

  **Must NOT do**:
  - Do NOT add more than 2 indicator lines total (1 real SMA + 1 predicted SMA)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Indicator overlay, line styling, visual distinction real vs predicted

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25-27, 29-30)
  - **Blocks**: None directly
  - **Blocked By**: Tasks 14 (SMA rendering), 26 (prediction messages)

  **References**:
  - TradingView `LineStyle`: Enum with Solid, Dashed, Dotted, LargeDashed, SparseDotted
  - For dotted: `lineStyle: 2` (LineStyle.Dotted)

  **Acceptance Criteria**:
  - [ ] `test_predicted_sma_visible`: PASS
  - [ ] `test_predicted_sma_different_style`: PASS
  - [ ] Predicted SMA renders at correct time position (30s ahead)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Predicted SMA appears as dotted line after prediction
    Tool: Playwright
    Preconditions: Platform running 70+ seconds, SMA(20) visible, prediction received
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait 80 seconds
      3. Take screenshot
      4. Evaluate: chart.series().filter(s => s.options().lineStyle === 2).length >= 1
    Expected Result: At least 1 series with dotted/dashed line style
    Evidence: .sisyphus/evidence/task-28-predicted-sma.png
  ```

  **Commit**: YES (groups with Tasks 25-30)

---

- [ ] 29. **Confidence threshold rendering (UP=green, DOWN=red, UNCERTAIN=gray)** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/app.js`:
    - On prediction message:
      - Map `direction` to predicted candle colors:
        - `UP` → green RGBA `rgba(38, 166, 154, 0.6)`
        - `DOWN` → red RGBA `rgba(239, 83, 80, 0.6)`
        - `UNCERTAIN` → gray RGBA `rgba(128, 128, 128, 0.3)`
      - Update predicted CandlestickSeries options dynamically based on direction
    - Add confidence indicator: small text/icon next to predicted candle showing confidence %
  - Write Playwright test FIRST:
    - `test_up_prediction_is_green`: Mock UP prediction → assert predicted series upColor is green
    - `test_down_prediction_is_red`: Mock DOWN prediction → assert predicted series upColor is red
    - `test_uncertain_prediction_is_gray`: Mock UNCERTAIN → assert low opacity gray

  **Must NOT do**:
  - Do NOT hardcode prediction direction — use the message's `direction` field

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Dynamic visual styling based on data, confidence rendering

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25-28, 30)
  - **Blocks**: None directly
  - **Blocked By**: Tasks 27 (predicted series exists)

  **References**:
  - TradingView `series.applyOptions()`: Update series colors dynamically
  - RGBA transparency: Lower alpha for UNCERTAIN (0.3) to visually de-emphasize

  **Acceptance Criteria**:
  - [ ] `test_up_prediction_is_green`: PASS
  - [ ] `test_down_prediction_is_red`: PASS
  - [ ] `test_uncertain_prediction_is_gray`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Predicted candle color changes based on direction
    Tool: Playwright
    Preconditions: Mock WS sending different prediction directions
    Steps:
      1. Navigate to http://localhost:8080/?mock=up
      2. Wait for prediction, take screenshot
      3. Navigate to http://localhost:8080/?mock=down
      4. Wait for prediction, take screenshot
    Expected Result: UP screenshot shows green-ish predicted candle
    Expected Result: DOWN screenshot shows red-ish predicted candle
    Evidence: .sisyphus/evidence/task-29-up-prediction.png
             .sisyphus/evidence/task-29-down-prediction.png
  ```

  **Commit**: YES (groups with Tasks 25-30)

---

- [ ] 30. **ML-unavailable graceful degradation (frontend)** [Playwright TDD]

  **What to do**:
  - Update `frontend/js/app.js`:
    - Subscribe to WS `onStatus` callback for:
      - `ml_unavailable` → Show "Prediction unavailable" indicator (orange badge in status bar)
      - `ml_error` → Show "Prediction error" indicator (red badge)
      - On recovery (any prediction message received after error) → Clear badge, restore predictions
    - When ML is unavailable:
      - Keep rendering real candles (no crash, no white screen)
      - Remove/clear predicted candle series
      - Remove/clear predicted SMA series
      - Show the "Prediction unavailable" indicator
  - Write Playwright test FIRST:
    - `test_ml_unavailable_shows_indicator`: Stop ML service, assert indicator appears
    - `test_real_candles_continue_during_ml_outage`: Assert real candles still update while ML down
    - `test_prediction_restored_on_ml_recovery`: Start ML service, assert predictions return

  **Must NOT do**:
  - Do NOT crash or show white screen when ML is down
  - Do NOT stop real candle streaming

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Graceful degradation UI, state recovery, status management

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25-29)
  - **Blocks**: None
  - **Blocked By**: Tasks 13 (status rendering), 27 (predicted series)

  **References**:
  - TradingView `series.setData([])`: Clear series data
  - `chart.removeSeries(series)`: Remove series from chart entirely

  **Acceptance Criteria**:
  - [ ] `test_ml_unavailable_shows_indicator`: PASS
  - [ ] `test_real_candles_continue_during_ml_outage`: PASS
  - [ ] `test_prediction_restored_on_ml_recovery`: PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Chart survives ML service crash gracefully
    Tool: Playwright
    Preconditions: All services running, predictions visible
    Steps:
      1. Navigate to http://localhost:8080/
      2. Wait for prediction to appear (80s)
      3. Stop ML service: docker compose stop ml-service
      4. Wait 20 seconds
      5. Assert: real candles still updating (candle count increased)
      6. Assert: "Prediction unavailable" indicator visible
      7. Assert: no white screen, no JS console errors
      8. Take screenshot
    Expected Result: Clean degradation — candles stream, prediction area shows unavailable state
    Evidence: .sisyphus/evidence/task-30-ml-down.png

  Scenario: Predictions return when ML recovers
    Tool: Playwright
    Preconditions: After scenario above (ML was stopped)
    Steps:
      1. Start ML service: docker compose start ml-service
      2. Wait 30 seconds
      3. Assert: predicted candle reappears
      4. Assert: "Prediction unavailable" indicator disappears
    Expected Result: Automatic recovery, predictions restored
    Evidence: .sisyphus/evidence/task-30-ml-recovered.png
  ```

  **Commit**: YES (groups with Tasks 25-30)
  - Message: `feat(integration): predicted candles and indicators on chart`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`

  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Compare deliverables against plan.

  **Verify**: Real-time BTCUSDT streaming, 60-candle buffer, TradingView chart, LightGBM model, dual CandlestickSeries, SMA(20) real + predicted, Docker 3 services, `numta` absent, `localhost` absent in inter-service code, `train_test_split` absent, exactly 22 features, no 5m code, no asset dropdown.

  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/30] | Evidence [N files] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`

  Run `go vet ./...` + `golangci-lint` + `pytest` + `npx playwright test`. Review for: `any` types, empty catches, commented-out code, unused imports, AI slop (excessive comments, over-abstraction, generic names). Verify Docker best practices: `.dockerignore`, layer caching, non-root user.

  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Go Tests [N pass/N fail] | Python Tests [N pass/N fail] | Playwright [N pass/N fail] | VERDICT`

- [ ] F3. **Real QA Execution** — `unspecified-high` (+ `playwright` skill)

  Start clean: `docker compose down -v && docker compose up --wait`. Execute ALL QA scenarios from ALL tasks. Test cross-task: full pipeline, ML degradation + recovery, empty state, reconnection, multiple tabs. Save evidence to `.sisyphus/evidence/final-qa/`.

  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`

  For each task: read spec, read diff. Verify 1:1. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes. Verify: 22 features, SMA(20) only, BTCUSDT hardcoded, 60s only, service names not localhost.

  Output: `Tasks [N/30 compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Commit 1 (Wave 0)**: `chore: project scaffolding and prerequisites` — go.mod, docker-compose.yml, .env, requirements.txt, schemas
- **Commit 2 (Wave 1)**: `feat(backend): Binance websocket streaming and internal broadcast` — Go backend core
- **Commit 3 (Wave 2)**: `feat(frontend): candlestick chart with real-time updates` — HTML/JS chart
- **Commit 4 (Wave 3)**: `feat(training): historical data fetch and LightGBM model training` — training pipeline + model artifact
- **Commit 5 (Wave 4)**: `feat(ml): FastAPI prediction service with LightGBM` — ML microservice
- **Commit 6 (Wave 5)**: `feat(integration): predicted candles and indicators on chart` — full prediction visualization
- **Commit 7 (Wave FINAL)**: `chore: verification fixes and final polish` — review-driven adjustments

---

## Success Criteria

### Verification Commands
```bash
docker compose up --wait                              # All services healthy
curl -s http://localhost:8080/health                  # {"status":"ok","binance":"connected"}
curl -s http://localhost:8000/health                  # {"status":"ok","model_loaded":true}
go test -race ./...                                   # All pass, no races
pytest ml-service/tests/                              # All pass
docker compose down --timeout 10                      # Clean exit, code 0
```

### Final Checklist
- [ ] All 6 commit groups pushed
- [ ] Docker Compose starts all services cleanly
- [ ] Real BTCUSDT candles stream and display within 5 seconds of startup
- [ ] Predicted candles appear after 70 seconds (60s buffer + 10s first prediction)
- [ ] Predicted SMA(20) renders as dotted line alongside solid real SMA(20)
- [ ] ML service unavailable → chart continues showing real data with "Prediction unavailable" indicator
- [ ] All tests pass (go test -race, pytest, Playwright scenarios)
- [ ] Zero `localhost` hardcodes in inter-service communication
- [ ] LightGBM model uses TimeSeriesSplit validation (AUC > 0.52 on test fold)

---

## Future Work (Explicitly Out of MVP Scope)

- 5-minute timeframe support (requires separate model)
- Multi-asset support (ETH, stocks, forex, gold)
- Asset dropdown in UI
- Trend lines, support/resistance indicators
- News sentiment analysis (FinBERT + real-time news API)
- Model retraining automation (currently manual)
- Historical data persistence (database)
- Cloud deployment
- User authentication
- Mobile responsive design
