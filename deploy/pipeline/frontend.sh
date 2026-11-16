#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../web"

# Pinned: a newer/older npm validates package-lock.json's optional,
# platform-mismatched, bundleDependencies-carrying entries (e.g.
# @tailwindcss/oxide-wasm32-wasi) inconsistently, so npm ci can fail
# non-deterministically depending on which npm actions/setup-node resolves
# for the given node-version. Match the npm this lockfile was generated with.
npm install -g npm@11.6.2
npm ci
npx tsc --noEmit
npx vitest run
