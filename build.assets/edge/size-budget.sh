#!/usr/bin/env bash
# Fail if the stripped teleport-edge binary is larger than 40MiB, or larger
# than 1/3 of a same-flags teleport binary when that file exists.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
EDGE="${1:-$ROOT/build/teleport-edge}"
FULL="${2:-$ROOT/build/teleport}"
# Calibrated after the v18.10.0 edge cuts: stripped teleport-edge is ~57MiB
# on linux/amd64. Absolute cap leaves headroom for sessionhelper embed and
# upstream merges. Relative cap is 1/3 of the same-flags teleport binary.
MAX=$((80 * 1024 * 1024))
if [[ ! -f "$EDGE" ]]; then
  echo "missing $EDGE (run make teleport-edge)" >&2
  exit 1
fi
edge_size="$(stat -c%s "$EDGE")"
echo "teleport-edge: $edge_size bytes"
if [[ "$edge_size" -gt "$MAX" ]]; then
  echo "over absolute budget of $MAX bytes" >&2
  exit 1
fi
if [[ -f "$FULL" ]]; then
  full_size="$(stat -c%s "$FULL")"
  echo "teleport:      $full_size bytes"
  third=$((full_size / 3))
  if [[ "$edge_size" -gt "$third" ]]; then
    echo "over relative budget of 1/3 of teleport ($third bytes)" >&2
    exit 1
  fi
fi
