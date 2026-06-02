#!/bin/bash
set -e

echo "=== Running Go Backend Tests ==="
cd go-backend
go test -race ./...
cd ..

echo "=== Running ML Service Tests ==="
cd ml-service
pip install -q -r requirements.txt
pytest
cd ..

echo "=== Running Frontend Tests ==="
cd frontend-tests
npm ci --silent
npx playwright install --with-deps chromium
npx playwright test
cd ..

echo "=== Building Docker Compose ==="
docker compose build

echo "=== All CI checks passed ==="