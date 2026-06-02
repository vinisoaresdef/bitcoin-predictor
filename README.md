# BTC Predictor Platform

A real-time Bitcoin price prediction platform that streams data from Binance, processes it through a machine learning pipeline, and visualizes predictions on an interactive TradingView chart.

## Overview

The BTC Predictor Platform is a full-stack application that:

- Streams real-time 1-second candlestick data from Binance WebSocket API
- Maintains a rolling buffer of 60 candles for ML feature computation
- Generates 30-second forward price predictions using LightGBM models
- Visualizes real-time and predicted data with TradingView Lightweight Charts
- Provides graceful degradation when ML services are unavailable

## Architecture

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                     BTC Predictor Platform                   │
                    └─────────────────────────────────────────────────────────────┘
                                                     │
                                                     ▼
┌──────────────┐    WebSocket    ┌─────────────────────────────────┐
│              │◄───────────────►│         Go Backend              │
│   Binance    │   1s Klines     │    (Go 1.24.2, Ring Buffer)     │
│   Exchange   │                 │                                 │
│              │                 │  • WebSocket Hub (port 8080)    │
└──────────────┘                 │  • Binance WS Client            │
                                 │  • ML HTTP Client               │
                                 │  • Candle Buffer (60 slots)     │
                                 └──────────────┬──────────────────┘
                                                │
                        ┌───────────────────────┴───────────────────────┐
                        │ HTTP API: /predict                              │
                        ▼                                                 ▼
        ┌──────────────────────────────┐                  ┌──────────────────────────────┐
        │      ML Service              │                  │     Frontend Clients         │
        │   (FastAPI + LightGBM)       │                  │  (TradingView Charts)        │
        │                              │                  │                              │
        │  • Feature Engineering       │                  │  • Real-time candles         │
        │  • Prediction API (port 8000)│                  │  • Predicted candles (30s)   │
        │  • Technical Indicators      │                  │  • SMA lines (real + pred)   │
        │  • Health endpoint           │                  │  • Status indicators         │
        └──────────────────────────────┘                  └──────────────────────────────┘

Docker Services:
┌─────────────────────────────────────────────────────────────────────────────────────┐
│  ┌──────────────────┐    ┌──────────────────┐                                      │
│  │   go-backend     │◄──►│   ml-service     │        predictor-net (bridge)        │
│  │   port: 8080     │    │   port: 8000     │                                      │
│  └────────┬─────────┘    └──────────────────┘                                      │
│           │                                                                         │
│           │ WebSocket: ws://localhost:8080/ws                                       │
│           ▼                                                                         │
│  ┌──────────────────┐                                                              │
│  │   frontend       │  (Served via http-server or nginx)                            │
│  │   port: 8081     │                                                              │
│  └──────────────────┘                                                              │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Binance WebSocket** streams 1-second klines to Go Backend
2. **Go Backend** stores candles in a ring buffer (60 slots)
3. When buffer is full, **Go Backend** sends 60 candles to **ML Service** via HTTP POST `/predict`
4. **ML Service** computes 22 features and returns prediction (UP/DOWN/UNCERTAIN + confidence)
5. **Go Backend** broadcasts to all WebSocket clients:
   - Real-time candles
   - SMA values (20-period)
   - Predictions (candle + SMA line)
   - Status messages
6. **Frontend** displays dual candlestick series (real + predicted overlay) with dotted predicted SMA line

## Prerequisites

### Required

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Backend service |
| Python | 3.11+ | ML service |
| Docker | 24.x+ | Containerization |
| Docker Compose | v2+ | Multi-container orchestration |
| Node.js | 20+ | Frontend testing (optional) |

### Installation

**Go 1.24+:**
```bash
# Download from https://go.dev/dl/
# Or using package manager:
# Ubuntu/Debian: sudo apt install golang-go
# macOS: brew install go
```

**Python 3.11+:**
```bash
# Ubuntu/Debian: sudo apt install python3 python3-pip python3-venv
# macOS: brew install python
# Verify: python3 --version
```

**Docker & Docker Compose:**
```bash
# Ubuntu: https://docs.docker.com/engine/install/ubuntu/
# macOS: Install Docker Desktop
# Verify:
docker --version
docker compose version
```

**Node.js 20+ (for E2E tests):**
```bash
# Using nvm (recommended):
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 20
nvm use 20
```

## Setup Instructions

### 1. Clone the Repository

```bash
git clone <repository-url>
cd Predictor
```

### 2. Environment Configuration

Create a `.env` file in the project root:

```bash
# Copy the example configuration
cp .env.example .env  # or create manually
```

Edit `.env` with your preferred ports:

```env
PORT_BACKEND=8080
PORT_ML=8000
BINANCE_WS_URL=wss://stream.binance.com:9443/ws
BINANCE_REST_URL=https://api.binance.com/api/v3
```

### 3. Build Docker Images

```bash
# Build all services
docker compose build

# Or build individually:
docker compose build go-backend
docker compose build ml-service
```

### 4. Verify Prerequisites

Run the verification script (if available) or manually check:

```bash
go version          # Should show go1.24.2 or higher
python3 --version   # Should show 3.11+ or 3.12+
docker --version
docker compose version
node --version      # Should show v20+ (for tests)
```

## Running the Application

### Production Mode (Docker)

Start all services with Docker Compose:

```bash
# Start all services
docker compose up

# Start in detached mode (background)
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down
```

Access the application:
- Frontend: http://localhost:8080 (or port configured in .env)
- ML Service Health: http://localhost:8000/health
- Go Backend Health: http://localhost:8080/health

### Development Mode (Outside Docker)

#### Terminal 1: ML Service

```bash
cd ml-service

# Create virtual environment
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate

# Install dependencies
pip install -r requirements.txt

# Start the ML service
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```

The ML service will be available at http://localhost:8000

#### Terminal 2: Go Backend

```bash
cd go-backend

# Download dependencies
go mod download

# Run the backend
go run ./cmd/predictor

# Or build and run:
go build -o predictor ./cmd/predictor
./predictor
```

The backend will be available at http://localhost:8080

#### Terminal 3: Frontend (Optional)

```bash
cd frontend

# Serve with any static file server
npx http-server . -p 8081 --cors

# Or using Python
python3 -m http.server 8081
```

Access the frontend at http://localhost:8081

**Note:** When running outside Docker, ensure the backend is configured to connect to `localhost:8000` instead of `ml-service:8000`.

## Testing

### ML Service Tests

```bash
cd ml-service

# Activate virtual environment
source venv/bin/activate

# Run all tests
pytest

# Run with coverage
pytest --cov=. --cov-report=html

# Run specific test file
pytest tests/test_main.py -v
```

### Go Backend Tests

```bash
cd go-backend

# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run specific package
go test ./internal/server/ -v

# Run with coverage
go test -cover ./...
```

### Frontend Tests

```bash
cd frontend-tests

# Install dependencies
npm install

# Install Playwright browsers (if needed)
npx playwright install chromium

# Run E2E tests
npm test

# Run specific test file
npx playwright test chart.spec.js

# Run in headed mode (visible browser)
npx playwright test --headed
```

### Training Tests

```bash
cd training

# Install dependencies
pip install -r requirements.txt  # if available

# Run training tests
pytest ../tests/test_train.py -v
pytest ../tests/test_features.py -v
pytest ../tests/test_labels.py -v
```

## Project Structure

```
Predictor/
├── docker-compose.yml          # Docker orchestration
├── .env                        # Environment variables
├── .env.example                # Environment template
├── PREREQUISITES.md            # Prerequisites documentation
├── README.md                   # This file
│
├── go-backend/                 # Go Backend Service
│   ├── Dockerfile
│   ├── go.mod                  # Go 1.24.2
│   ├── go.sum
│   ├── cmd/predictor/          # Main entry point
│   │   └── main.go
│   └── internal/
│       ├── binance/            # Binance WebSocket client
│       │   ├── client.go
│       │   └── client_test.go
│       ├── buffer/             # Ring buffer for candles
│       │   ├── ring.go
│       │   └── ring_test.go
│       ├── ml/                 # ML HTTP client
│       │   ├── client.go
│       │   └── client_test.go
│       ├── schemas/            # Data structures
│       │   └── schemas.go
│       └── server/             # HTTP/WebSocket servers
│           ├── http.go
│           ├── ws.go
│           └── ws_test.go
│
├── ml-service/                 # ML Inference Service
│   ├── Dockerfile
│   ├── requirements.txt        # Python dependencies
│   ├── main.py                 # FastAPI application
│   ├── ta.py                   # Technical indicators
│   └── app/
│       └── schemas.py          # Pydantic models
│   └── tests/
│       ├── test_main.py
│       ├── test_ta.py
│       └── test_error_handling.py
│
├── frontend/                   # Static Frontend
│   ├── index.html              # Main HTML
│   ├── ws-test.html            # WebSocket test page
│   ├── css/
│   │   └── style.css
│   └── js/
│       ├── app.js              # Application logic
│       ├── chart.js            # TradingView charts
│       └── ws-client.js        # WebSocket client
│   └── tests/                  # Unit tests
│       ├── app.test.js
│       ├── chart.test.js
│       └── prediction.test.js
│
├── frontend-tests/             # E2E Tests (Playwright)
│   ├── package.json
│   ├── playwright.config.js
│   ├── *.spec.js               # Test files
│   └── playwright-report/      # Test reports
│
├── training/                   # Model Training Scripts
│   ├── train.py                # Main training script
│   ├── fetch_data.py           # Data fetching
│   ├── features.py             # Feature engineering
│   └── labels.py               # Label generation
│   └── tests/                  # Training tests
│
├── schemas/                    # Schema Documentation
│   └── README.md
│
└── .sisyphus/                  # Development documentation
    ├── plans/
    └── notepads/
```

## Key Configuration Options

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT_BACKEND` | 8080 | Go backend HTTP/WebSocket port |
| `PORT_ML` | 8000 | ML service HTTP port |
| `BINANCE_WS_URL` | wss://stream.binance.com:9443/ws | Binance WebSocket endpoint |
| `BINANCE_REST_URL` | https://api.binance.com/api/v3 | Binance REST API endpoint |
| `ML_SERVICE_URL` | http://ml-service:8000 | ML service URL (Docker) |

### Docker Configuration

Edit `docker-compose.yml` to customize:

```yaml
services:
  go-backend:
    ports:
      - "${PORT_BACKEND}:8080"  # Host:Container
    environment:
      - PORT=${PORT_BACKEND}
    restart: unless-stopped
    
  ml-service:
    ports:
      - "${PORT_ML}:8000"
    restart: unless-stopped
```

### ML Service Configuration

In `ml-service/main.py`:

```python
MODEL_PATH = Path("models/model.joblib")  # Path to trained model
MODEL_VERSION = "1.0.0"                   # Model version
REQUEST_TIMEOUT_SECONDS = 30              # Prediction timeout
```

### Frontend Configuration

In `frontend/js/ws-client.js`:

```javascript
const DEFAULT_WS_URL = 'ws://localhost:8080/ws';
const DEFAULT_HTTP_URL = 'http://localhost:8080';
```

## API Endpoints

### Go Backend

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ws` | WebSocket | Real-time data stream |

### ML Service

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health + model status |
| `/predict` | POST | Get price prediction |
| `/ta` | POST | Compute technical indicators |

### Prediction Request Format

```json
POST /predict
{
  "candles": [
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "open": 50000.0,
      "high": 50100.0,
      "low": 49900.0,
      "close": 50050.0,
      "volume": 100.5
    }
    // ... exactly 60 candles
  ]
}
```

### Prediction Response Format

```json
{
  "direction": "UP",
  "confidence": 0.85,
  "probabilities": [0.85, 0.10, 0.05]
}
```

## Troubleshooting

### Common Issues

#### 1. Port Already in Use

**Error:** `bind: address already in use`

**Solution:**
```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port in .env
PORT_BACKEND=8081
```

#### 2. ML Service Model Not Found

**Error:** `Model file not found at models/model.joblib`

**Solution:**
```bash
# Train a model first
cd training
python3 train.py

# Or copy an existing model to ml-service/models/
mkdir -p ml-service/models
cp path/to/model.joblib ml-service/models/
```

#### 3. WebSocket Connection Failed

**Error:** `WebSocket connection failed`

**Solution:**
- Verify backend is running: `curl http://localhost:8080/health`
- Check firewall settings
- Verify correct WebSocket URL in frontend

#### 4. Docker Permission Denied

**Error:** `permission denied while trying to connect to Docker daemon`

**Solution:**
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Log out and log back in
# Or:
newgrp docker
```

#### 5. ML Service Timeout

**Error:** `Request timed out after 30 seconds`

**Solution:**
- Check ML service is healthy: `curl http://localhost:8000/health`
- Verify model is loaded
- Increase timeout in `ml-service/main.py` if needed

#### 6. Go Module Issues

**Error:** `module not found` or `checksum mismatch`

**Solution:**
```bash
cd go-backend

# Clean and re-download
go clean -modcache
go mod download
go mod tidy
```

#### 7. Python Dependencies Conflicts

**Error:** `ModuleNotFoundError` or version conflicts

**Solution:**
```bash
cd ml-service

# Recreate virtual environment
rm -rf venv
python3 -m venv venv
source venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt
```

#### 8. Playwright Tests Fail

**Error:** `Browser not found` or `page.goto: net::ERR_CONNECTION_REFUSED`

**Solution:**
```bash
cd frontend-tests

# Install browsers
npx playwright install chromium

# Start frontend server first
npx http-server ../frontend -p 8080 --cors &

# Then run tests
npm test
```

### Getting Help

1. Check service logs: `docker compose logs -f <service-name>`
2. Verify environment: `cat .env`
3. Check health endpoints:
   ```bash
   curl http://localhost:8080/health
   curl http://localhost:8000/health
   ```
4. Review PREREQUISITES.md for version requirements
5. Check individual service READMEs in subdirectories

## Development Notes

### Adding New Features

1. **Backend:** Add handlers in `go-backend/internal/server/`
2. **ML Service:** Add endpoints in `ml-service/main.py`
3. **Frontend:** Update `frontend/js/` files
4. **Tests:** Add corresponding test files

### Running in Debug Mode

**Go Backend:**
```bash
cd go-backend
go run ./cmd/predictor  # Auto-reload not built-in
```

**ML Service:**
```bash
cd ml-service
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```

### Code Quality

```bash
# Go formatting
cd go-backend
go fmt ./...

# Python formatting
cd ml-service
black .

# Run linters
golangci-lint run  # Go
pylint ml-service/ # Python
```

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here]

## Changelog

See commit history for detailed changes.
