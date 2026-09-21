#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

: "${REGISTRY:?e.g. asia-southeast2-docker.pkg.dev/abak-academy-platform/app}"
: "${TAG:?commit SHA — never 'latest'}"

# VERSION_TAG is the optional human-facing name for the same image, e.g. v1.4.0.
# The commit SHA stays the image's identity: it is immutable and it is what
# `docker ps` on vm-app is diffed against when a production report comes in.
# A version tag can be moved; a SHA cannot, so both are pushed and never one.
VERSION_TAG="${VERSION_TAG:-}"

# api/worker are byte-identical across environments; web is not. Splitting them lets CI
# build the pair once and push it to both registries, and build the two webs in parallel.
COMPONENTS="${COMPONENTS:-api worker web}"

# build-artifact.sh emits GOARCH=amd64, so the images must be amd64 or the binary will not execute.
PLATFORM=linux/amd64

built=()

for component in $COMPONENTS; do
  case "$component" in
    api|worker)
      docker build --platform "$PLATFORM" \
        -f "deploy/images/Dockerfile.${component}" \
        -t "${REGISTRY}/${component}:${TAG}" backend
      built+=("${REGISTRY}/${component}:${TAG}")
      if [ -n "$VERSION_TAG" ]; then
        docker tag "${REGISTRY}/${component}:${TAG}" "${REGISTRY}/${component}:${VERSION_TAG}"
        built+=("${REGISTRY}/${component}:${VERSION_TAG}")
      fi
      ;;
    web)
      : "${TARGET_ENV:?staging|prod}"
      : "${NEXT_PUBLIC_API_BASE_URL:?inlined into the web bundle at build time}"
      # web is not promotable between environments: NEXT_PUBLIC_* is inlined into the client bundle here.
      docker build --platform "$PLATFORM" -f web/Dockerfile \
        --build-arg "NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}" \
        --build-arg "NEXT_PUBLIC_MIDTRANS_SNAP_URL=${NEXT_PUBLIC_MIDTRANS_SNAP_URL:-https://app.sandbox.midtrans.com/snap/snap.js}" \
        --build-arg "NEXT_PUBLIC_MIDTRANS_CLIENT_KEY=${NEXT_PUBLIC_MIDTRANS_CLIENT_KEY:-}" \
        --build-arg "NEXT_PUBLIC_GOOGLE_CLIENT_ID=${NEXT_PUBLIC_GOOGLE_CLIENT_ID:-}" \
        -t "${REGISTRY}/web:${TARGET_ENV}-${TAG}" web
      built+=("${REGISTRY}/web:${TARGET_ENV}-${TAG}")
      if [ -n "$VERSION_TAG" ]; then
        docker tag "${REGISTRY}/web:${TARGET_ENV}-${TAG}" "${REGISTRY}/web:${TARGET_ENV}-${VERSION_TAG}"
        built+=("${REGISTRY}/web:${TARGET_ENV}-${VERSION_TAG}")
      fi
      ;;
    *)
      echo "unknown component: ${component}" >&2
      exit 1
      ;;
  esac
done

if [[ "${PUSH:-0}" == "1" ]]; then
  for image in "${built[@]}"; do
    docker push "$image"
  done
fi
