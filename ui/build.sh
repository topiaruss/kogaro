#!/bin/bash
set -e

# Build the Svelte frontend
cd frontend
npm install
npm run build
cd ..

# Build the Go binary with Wails desktop tags
# Note: wails CLI segfaults on macOS with Go 1.24 — build directly with go build
CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers" \
  go build -tags desktop,production -o build/bin/kogaro-ui .

echo "Built: build/bin/kogaro-ui"
