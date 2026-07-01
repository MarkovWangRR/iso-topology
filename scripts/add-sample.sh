#!/usr/bin/env bash
# add-sample.sh — turn a reported scene into a permanent golden regression sample.
#
# Issue #12: a submitted "bad case" (see .github/ISSUE_TEMPLATE/bad-case.yml)
# becomes a frozen sample under samples/<category>/<name>/ so its exact rendering
# is locked in by TestGolden and can never silently regress again.
#
# Usage:
#   scripts/add-sample.sh <category> <name> <input.yaml|input.d2>
#     <category>  node | topology
#     <name>      slug for the sample directory
#     <input>     path to the scene DSL to freeze
#
# Example:
#   scripts/add-sample.sh topology cramped-labels ./bad.yaml
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <node|topology> <name> <input.yaml|input.d2>" >&2
  exit 2
fi

category="$1"; name="$2"; input="$3"

case "$category" in
  node|topology) ;;
  *) echo "error: category must be 'node' or 'topology'" >&2; exit 2 ;;
esac
if [[ ! -f "$input" ]]; then
  echo "error: input file not found: $input" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="$repo_root/samples/$category/$name"
ext="${input##*.}"
[[ "$ext" == "d2" || "$ext" == "json" ]] || ext="yaml"

mkdir -p "$dir"
cp "$input" "$dir/input.$ext"

# Render the frozen golden through the SAME pipeline the test uses.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
( cd "$repo_root" && go run ./cmd/isotopo render "$dir/input.$ext" "$tmp" )
cp "$tmp/topology.svg" "$dir/expected.svg"

echo "added sample: samples/$category/$name/{input.$ext,expected.svg}"
echo "verify: go test -run 'TestGolden/$category/$name' ."
