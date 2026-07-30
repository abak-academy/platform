#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

: "${REGISTRY:?e.g. ghcr.io/panca1093/bimbel-abak-academy}"
: "${TAG:?commit SHA — never 'latest'}"
: "${TARGET_ENV:?staging|prod}"
: "${NEXT_PUBLIC_API_BASE_URL:?inlined into the web bundle at build time}"

NEXT_PUBLIC_MIDTRANS_SNAP_URL="${NEXT_PUBLIC_MIDTRANS_SNAP_URL:-https://app.sandbox.midtrans.com/snap/snap.js}"
NEXT_PUBLIC_MIDTRANS_CLIENT_KEY="${NEXT_PUBLIC_MIDTRANS_CLIENT_KEY:-}"
NEXT_PUBLIC_GOOGLE_CLIENT_ID="${NEXT_PUBLIC_GOOGLE_CLIENT_ID:-}"

# build-artifact.sh emits GOARCH=amd64, so the images must be amd64 or the binary will not execute.
PLATFORM=linux/amd64

docker build --platform "$PLATFORM" -f deploy/images/Dockerfile.api    -t "${REGISTRY}/api:${TAG}"    backend
docker build --platform "$PLATFORM" -f deploy/images/Dockerfile.worker -t "${REGISTRY}/worker:${TAG}" backend

# web is not promotable between environments: NEXT_PUBLIC_* is inlined into the client bundle here.
docker build --platform "$PLATFORM" -f web/Dockerfile \
  --build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}" \
  --build-arg "NEXT_PUBLIC_MIDTRANS_SNAP_URL=${NEXT_PUBLIC_MIDTRANS_SNAP_URL}" \
  --build-arg "NEXT_PUBLIC_MIDTRANS_CLIENT_KEY=${NEXT_PUBLIC_MIDTRANS_CLIENT_KEY}" \
  --build-arg "NEXT_PUBLIC_GOOGLE_CLIENT_ID=${NEXT_PUBLIC_GOOGLE_CLIENT_ID}" \
  -t "${REGISTRY}/web:${TARGET_ENV}-${TAG}" web

if [[ "${PUSH:-0}" == "1" ]]; then
  docker push "${REGISTRY}/api:${TAG}"
  docker push "${REGISTRY}/worker:${TAG}"
  docker push "${REGISTRY}/web:${TARGET_ENV}-${TAG}"
fi
