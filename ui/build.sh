#!/bin/bash
set -e

# Build the Svelte frontend
cd frontend
npm install
npm run build
cd ..

# Inject build metadata
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S UTC')

# Build the Go binary with Wails desktop tags
# Note: wails CLI segfaults on macOS with Go 1.24 — build directly with go build
CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
  go build -tags desktop,production \
  -ldflags "-X 'main.buildCommit=${COMMIT}' -X 'main.buildTime=${BUILD_TIME}'" \
  -o build/bin/kogaro-ui .

echo "Built: build/bin/kogaro-ui (${COMMIT} @ ${BUILD_TIME})"
