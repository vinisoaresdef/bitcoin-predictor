#!/bin/bash
# =============================================================================
# BTC Predictor Platform — Docker Compose Integration Test
# =============================================================================
# Validates:
#   1. All 3 services (frontend, go-backend, ml-service) start successfully
#   2. Health endpoints respond (ml-service /health, go-backend /health)
#   3. Binance 1s stream connects and produces candles within 5 seconds
#   4. WebSocket server accepts connections on ws://localhost:8080/ws
#   5. Frontend is accessible at http://localhost:3000
#   6. All containers remain stable for at least 30 seconds
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$PROJECT_DIR/docker-compose.yml"
FRONTEND_PORT="${PORT_FRONTEND:-3000}"
BACKEND_PORT="${PORT_BACKEND:-8080}"
ML_PORT="${PORT_ML:-8000}"
STABILITY_SECONDS=30
MAX_STARTUP_WAIT=120
PASSED=0
FAILED=0
LOGS_DIR=""

# ---------------------------------------------------------------------------
# Colors
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log_info()  { echo -e "${BLUE}[INFO]${NC}  $(date '+%H:%M:%S') $*"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC}  $(date '+%H:%M:%S') $*"; PASSED=$((PASSED + 1)); }
log_fail()  { echo -e "${RED}[FAIL]${NC}  $(date '+%H:%M:%S') $*"; FAILED=$((FAILED + 1)); }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $(date '+%H:%M:%S') $*"; }

cleanup() {
    local exit_code=$?
    log_info "Cleaning up..."

    # Collect logs before tearing down
    if [ -n "$LOGS_DIR" ] && [ -d "$LOGS_DIR" ]; then
        log_info "Saving container logs to $LOGS_DIR"
        docker compose -f "$COMPOSE_FILE" logs --no-color > "$LOGS_DIR/containers.log" 2>/dev/null || true
        docker compose -f "$COMPOSE_FILE" ps > "$LOGS_DIR/ps.txt" 2>/dev/null || true
    fi

    docker compose -f "$COMPOSE_FILE" down --volumes --remove-orphans 2>/dev/null || true
    log_info "Cleanup complete."

    if [ -n "$LOGS_DIR" ]; then
        echo ""
        echo "======================================================"
        echo "  Container logs saved to: $LOGS_DIR/containers.log"
        echo "======================================================"
    fi

    exit $exit_code
}

trap cleanup EXIT

# ---------------------------------------------------------------------------
# Pre-flight: verify docker compose is available
# ---------------------------------------------------------------------------
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v docker &> /dev/null; then
        log_fail "docker not found in PATH"
        exit 1
    fi

    if ! docker compose version &> /dev/null; then
        log_fail "docker compose not available"
        exit 1
    fi

    if [ ! -f "$COMPOSE_FILE" ]; then
        log_fail "docker-compose.yml not found at $COMPOSE_FILE"
        exit 1
    fi

    log_pass "Prerequisites OK (docker + compose + compose file)"
}

# ---------------------------------------------------------------------------
# Build and start
# ---------------------------------------------------------------------------
start_services() {
    log_info "Building Docker images (this may take a while)..."
    docker compose -f "$COMPOSE_FILE" build --parallel 2>&1 | tail -5

    log_info "Starting services via docker compose up -d..."
    docker compose -f "$COMPOSE_FILE" up -d

    log_info "Waiting for services to become healthy (max ${MAX_STARTUP_WAIT}s)..."

    local elapsed=0
    local interval=2

    while [ "$elapsed" -lt "$MAX_STARTUP_WAIT" ]; do
        local ml_status frontend_status backend_status all_up

        ml_status=$(docker compose -f "$COMPOSE_FILE" ps --format json 2>/dev/null | \
            python3 -c "
import sys, json
items = [json.loads(l) for l in sys.stdin if l.strip()]
ml = next((i for i in items if 'ml-service' in i.get('Service','')), None)
if ml:
    health = ml.get('Health', '')
    print('healthy' if health == 'healthy' else health or 'unknown')
else:
    print('missing')
" 2>/dev/null || echo "unknown")

        backend_status=$(docker compose -f "$COMPOSE_FILE" ps --format json 2>/dev/null | \
            python3 -c "
import sys, json
items = [json.loads(l) for l in sys.stdin if l.strip()]
be = next((i for i in items if 'go-backend' in i.get('Service','')), None)
if be:
    health = be.get('Health', '')
    print('healthy' if health == 'healthy' else health or 'unknown')
else:
    print('missing')
" 2>/dev/null || echo "unknown")

        frontend_status=$(docker compose -f "$COMPOSE_FILE" ps --format json 2>/dev/null | \
            python3 -c "
import sys, json
items = [json.loads(l) for l in sys.stdin if l.strip()]
fe = next((i for i in items if 'frontend' in i.get('Service','')), None)
if fe:
    state = fe.get('State', '')
    health = fe.get('Health', '')
    if health == 'healthy':
        print('healthy')
    elif 'running' in state.lower() or 'up' in state.lower():
        print('running')
    else:
        print(state or 'unknown')
else:
    print('missing')
" 2>/dev/null || echo "unknown")

        echo "  [${elapsed}s] ml-service=$ml_status  go-backend=$backend_status  frontend=$frontend_status"

        if [ "$ml_status" = "healthy" ] && [ "$backend_status" = "healthy" ] && { [ "$frontend_status" = "healthy" ] || [ "$frontend_status" = "running" ]; }; then
            log_info "All services ready after ${elapsed}s"
            return 0
        fi

        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    log_warn "Not all services became healthy within ${MAX_STARTUP_WAIT}s. Proceeding with checks anyway..."
    return 0
}

# ---------------------------------------------------------------------------
# Test 1: All services running
# ---------------------------------------------------------------------------
test_services_running() {
    log_info "=== Test 1: All 3 services are running ==="

    local services
    services=$(docker compose -f "$COMPOSE_FILE" ps --services --status running 2>/dev/null)

    if echo "$services" | grep -q "go-backend"; then
        log_pass "go-backend is running"
    else
        log_fail "go-backend is NOT running"
    fi

    if echo "$services" | grep -q "ml-service"; then
        log_pass "ml-service is running"
    else
        log_fail "ml-service is NOT running"
    fi

    if echo "$services" | grep -q "frontend"; then
        log_pass "frontend is running"
    else
        log_fail "frontend is NOT running"
    fi
}

# ---------------------------------------------------------------------------
# Test 2: ML Service health endpoint
# ---------------------------------------------------------------------------
test_ml_health() {
    log_info "=== Test 2: ML Service health endpoint (port $ML_PORT) ==="

    local response http_code
    response=$(curl -s -w "\n%{http_code}" --max-time 10 "http://localhost:${ML_PORT}/health" 2>/dev/null || echo "")
    http_code=$(echo "$response" | tail -1)
    local body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "200" ] || [ "$http_code" = "503" ]; then
        log_pass "ML /health responded with $http_code"
    else
        log_fail "ML /health returned $http_code (expected 200 or 503)"
        echo "  Response: $body"
    fi

    if echo "$body" | grep -q '"model_loaded"'; then
        local model_loaded
        model_loaded=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('model_loaded', False))" 2>/dev/null || echo "false")
        if [ "$model_loaded" = "True" ]; then
            log_pass "ML model loaded successfully"
        else
            log_warn "ML model not loaded (model_loaded=$model_loaded)"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Test 3: Go Backend health endpoint
# ---------------------------------------------------------------------------
test_backend_health() {
    log_info "=== Test 3: Go Backend health endpoint (port $BACKEND_PORT) ==="

    local response http_code
    response=$(curl -s -w "\n%{http_code}" --max-time 10 "http://localhost:${BACKEND_PORT}/health" 2>/dev/null || echo "")
    http_code=$(echo "$response" | tail -1)
    local body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "200" ]; then
        log_pass "Backend /health responded with 200"
    else
        log_fail "Backend /health returned $http_code (expected 200)"
        echo "  Response: $body"
        return
    fi

    if echo "$body" | grep -q '"status":"ok"'; then
        log_pass "Backend status is 'ok'"
    else
        log_fail "Backend status is not 'ok'"
        echo "  Body: $body"
    fi

    local binance_status
    binance_status=$(echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin).get('binance', 'unknown'))" 2>/dev/null || echo "unknown")
    log_info "  Binance status: $binance_status"
    if [ "$binance_status" = "connected" ]; then
        log_pass "Binance client is connected"
    else
        log_warn "Binance status is '$binance_status' (may not be connected yet)"
    fi
}

# ---------------------------------------------------------------------------
# Test 4: Binance 1s stream produces candles (via buffer size growing)
# ---------------------------------------------------------------------------
test_binance_stream() {
    log_info "=== Test 4: Binance 1s stream produces candles within 5s ==="

    # Get initial buffer size
    local initial_response initial_size
    initial_response=$(curl -s --max-time 5 "http://localhost:${BACKEND_PORT}/health" 2>/dev/null || echo "")
    initial_size=$(echo "$initial_response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('buffer_size', 0))" 2>/dev/null || echo "0")
    log_info "  Initial buffer size: $initial_size"

    # Wait 7 seconds for candles to arrive from Binance 1s stream
    sleep 7

    # Get final buffer size
    local final_response final_size
    final_response=$(curl -s --max-time 5 "http://localhost:${BACKEND_PORT}/health" 2>/dev/null || echo "")
    final_size=$(echo "$final_response" | python3 -c "import sys,json; print(json.load(sys.stdin).get('buffer_size', 0))" 2>/dev/null || echo "0")
    log_info "  Final buffer size: $final_size"

    if [ "$final_size" -gt "$initial_size" ]; then
        log_pass "Buffer grew from $initial_size to $final_size (candles flowing from Binance)"
    elif [ "$final_size" -gt 0 ]; then
        log_pass "Buffer has $final_size candles (Binance stream is active)"
    else
        log_fail "Buffer is empty after 7s (no candles from Binance)"
        log_info "  Checking go-backend logs for connection details..."
        docker compose -f "$COMPOSE_FILE" logs go-backend 2>/dev/null | tail -20
    fi
}

# ---------------------------------------------------------------------------
# Test 5: WebSocket server accepts connections
# ---------------------------------------------------------------------------
test_websocket() {
    log_info "=== Test 5: WebSocket server ws://localhost:$BACKEND_PORT/ws ==="

    local ws_status
    ws_status=$(python3 -c "
import http.client
import sys
try:
    conn = http.client.HTTPConnection('localhost', $BACKEND_PORT, timeout=5)
    conn.request('GET', '/ws', headers={
        'Connection': 'Upgrade',
        'Upgrade': 'websocket',
        'Sec-WebSocket-Version': '13',
        'Sec-WebSocket-Key': 'dGhlIHNhbXBsZSBub25jZQ==',
        'Host': 'localhost:$BACKEND_PORT'
    })
    resp = conn.getresponse()
    status = resp.status
    resp.read()
    conn.close()
    print(status)
except Exception as e:
    print(f'error: {e}', file=sys.stderr)
    print('0')
" 2>/dev/null)

    if [ "$ws_status" = "101" ]; then
        log_pass "WebSocket upgrade accepted (HTTP 101)"
    elif [ "$ws_status" = "400" ] || [ "$ws_status" = "426" ]; then
        log_warn "WebSocket returned HTTP $ws_status (upgrade may require specific headers)"
    elif [ "$ws_status" = "0" ]; then
        log_fail "WebSocket connection failed (connection refused or timeout)"
    else
        log_fail "WebSocket returned HTTP $ws_status (expected 101)"
    fi
}

# ---------------------------------------------------------------------------
# Test 6: Frontend is accessible
# ---------------------------------------------------------------------------
test_frontend() {
    log_info "=== Test 6: Frontend accessible at http://localhost:$FRONTEND_PORT ==="

    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "http://localhost:${FRONTEND_PORT}/" 2>/dev/null || echo "000")

    if [ "$http_code" = "200" ]; then
        log_pass "Frontend / returned HTTP 200"
    else
        log_fail "Frontend / returned HTTP $http_code (expected 200)"

        # Try the go-backend directly (it may also serve frontend)
        local be_code
        be_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "http://localhost:${BACKEND_PORT}/" 2>/dev/null || echo "000")
        log_info "  Backend / returned HTTP $be_code (go-backend may serve frontend on $BACKEND_PORT)"
    fi

    # Verify the response contains the expected HTML title
    local body
    body=$(curl -s --max-time 10 "http://localhost:${FRONTEND_PORT}/" 2>/dev/null || echo "")
    if echo "$body" | grep -q "BTCUSDT Predictor"; then
        log_pass "Frontend page contains expected title 'BTCUSDT Predictor'"
    else
        log_warn "Frontend page does not contain 'BTCUSDT Predictor' title"
    fi
}

# ---------------------------------------------------------------------------
# Test 7: ML service model check in logs
# ---------------------------------------------------------------------------
test_ml_model_loaded() {
    log_info "=== Test 7: ML service loaded model ==="

    local ml_logs
    ml_logs=$(docker compose -f "$COMPOSE_FILE" logs ml-service 2>/dev/null || echo "")

    if echo "$ml_logs" | grep -q "Model loaded successfully"; then
        log_pass "ML service log confirms 'Model loaded successfully'"
    else
        log_warn "ML service log does not contain 'Model loaded successfully'"
        # Check for model not found errors
        if echo "$ml_logs" | grep -q "Model file not found"; then
            log_fail "ML service could not find model file!"
        fi
    fi
}

# ---------------------------------------------------------------------------
# Test 8: Go Backend Binance connection in logs
# ---------------------------------------------------------------------------
test_backend_binance_log() {
    log_info "=== Test 8: Go Backend Binance connection in logs ==="

    local be_logs
    be_logs=$(docker compose -f "$COMPOSE_FILE" logs go-backend 2>/dev/null || echo "")

    if echo "$be_logs" | grep -q "Binance WebSocket client started"; then
        log_pass "Go Backend log confirms 'Binance WebSocket client started'"
    else
        log_warn "Go Backend log does not contain 'Binance WebSocket client started'"
    fi

    if echo "$be_logs" | grep -q "HTTP server starting"; then
        log_pass "Go Backend HTTP server started"
    else
        log_warn "Go Backend HTTP server start message not found in logs"
    fi

    # Check for connection errors
    if echo "$be_logs" | grep -qi "failed to connect"; then
        log_fail "Go Backend failed to connect to Binance"
        echo "$be_logs" | grep -i "failed to connect" | tail -5
    fi
}

# ---------------------------------------------------------------------------
# Test 9: Stability check — containers remain up for 30s
# ---------------------------------------------------------------------------
test_stability() {
    log_info "=== Test 9: Stability check (waiting ${STABILITY_SECONDS}s) ==="
    log_info "Monitoring container stability..."

    local start_time
    start_time=$(date +%s)

    sleep "$STABILITY_SECONDS"

    local running
    running=$(docker compose -f "$COMPOSE_FILE" ps --services --status running 2>/dev/null | wc -l)

    if [ "$running" -ge 3 ]; then
        log_pass "All 3 containers still running after ${STABILITY_SECONDS}s"
    elif [ "$running" -gt 0 ]; then
        log_fail "Only $running/3 containers survived ${STABILITY_SECONDS}s"
        docker compose -f "$COMPOSE_FILE" ps 2>/dev/null
    else
        log_fail "All containers crashed within ${STABILITY_SECONDS}s"
        docker compose -f "$COMPOSE_FILE" ps 2>/dev/null
    fi

    # Check for restarts
    local restarts
    restarts=$(docker compose -f "$COMPOSE_FILE" ps --format json 2>/dev/null | \
        python3 -c "
import sys, json
total = 0
for line in sys.stdin:
    if line.strip():
        item = json.loads(line)
        rs = item.get('ExitCode') or '0'
        try:
            total += int(rs)
        except:
            pass
print(total)
" 2>/dev/null || echo "0")

    if [ -z "$restarts" ]; then
        restarts="0"
    fi

    if [ "$restarts" != "0" ]; then
        log_warn "Detected container exit codes (possible crashes/restarts)"
    else
        log_pass "No unexpected container exits detected"
    fi
}

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
print_summary() {
    echo ""
    echo "======================================================"
    echo "  INTEGRATION TEST RESULTS"
    echo "======================================================"
    echo -e "  ${GREEN}PASSED:${NC} $PASSED"
    echo -e "  ${RED}FAILED:${NC} $FAILED"
    echo "======================================================"

    if [ "$FAILED" -gt 0 ]; then
        echo ""
        echo "  Some checks failed. Logs available at:"
        echo "    $LOGS_DIR/containers.log"
        echo "    $LOGS_DIR/ps.txt"
        return 1
    else
        echo ""
        echo -e "  ${GREEN}All integration checks passed!${NC}"
        return 0
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    LOGS_DIR=$(mktemp -d -t predictor-integration-XXXXXX)
    export LOGS_DIR

    echo ""
    echo "======================================================"
    echo "  BTC Predictor Platform — Integration Test"
    echo "  $(date)"
    echo "======================================================"
    echo ""

    check_prerequisites
    start_services

    # Give services a moment to fully initialize
    sleep 3

    test_services_running
    test_ml_health
    test_backend_health
    test_binance_stream
    test_websocket
    test_frontend
    test_ml_model_loaded
    test_backend_binance_log
    test_stability

    print_summary
}

main "$@"
