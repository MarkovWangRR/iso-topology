#!/usr/bin/env bash
# Release regression gate. Run this BEFORE cutting any version — it must come
# back all-green. Covers: build, vet, the full test suite (unit + golden +
# edit-engine matrix + cumulative-session invariants), the WASM build, and a
# bounded coverage-guided fuzz of the edit engine.
#
#   scripts/release-regression.sh            # ~60s fuzz
#   FUZZTIME=5m scripts/release-regression.sh  # deeper pre-release fuzz
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

step "build";        go build ./...
step "vet";          go vet ./...
step "test (unit + golden + edit matrix + cumulative-session invariants)"
                     go test ./...
step "wasm build";   GOOS=js GOARCH=wasm go build ./...
step "fuzz: edit-engine floor invariants (${FUZZTIME:-60s})"
                     go test . -run '^$' -fuzz '^FuzzEditSession$' -fuzztime "${FUZZTIME:-60s}"

printf '\n\033[1;32mALL GREEN — ready to release\033[0m\n'
