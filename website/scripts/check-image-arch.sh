#!/bin/bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <image-ref> [required-platform...]"
    echo "Example: $0 registry.ogaro.com/kogaro-website:abc123 linux/amd64 linux/arm64"
    exit 1
fi

IMAGE_REF="$1"
shift

if [[ $# -gt 0 ]]; then
    REQUIRED_PLATFORMS=("$@")
else
    REQUIRED_PLATFORMS=("linux/amd64" "linux/arm64")
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "❌ docker is required"
    exit 1
fi

if ! docker buildx version >/dev/null 2>&1; then
    echo "❌ docker buildx is required"
    exit 1
fi

echo "🔎 Checking image platforms for: $IMAGE_REF"
INSPECT_OUTPUT="$(docker buildx imagetools inspect "$IMAGE_REF" 2>/dev/null || true)"

if [[ -z "$INSPECT_OUTPUT" ]]; then
    echo "❌ Unable to inspect image: $IMAGE_REF"
    exit 1
fi

for platform in "${REQUIRED_PLATFORMS[@]}"; do
    if ! printf '%s\n' "$INSPECT_OUTPUT" | rg -q "$platform"; then
        echo "❌ Missing required platform: $platform"
        exit 1
    fi
done

echo "✅ Required platforms present: ${REQUIRED_PLATFORMS[*]}"
