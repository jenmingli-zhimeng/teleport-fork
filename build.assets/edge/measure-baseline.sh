#!/usr/bin/env bash
# Record stripped binary size for teleport and teleport-edge.
# RSS measurement needs a running cluster; see integration/edge/README.md.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
mkdir -p build
out="build.assets/edge/BASELINE.md"
{
  echo "# Binary size baseline"
  echo
  echo "Commit: $(git rev-parse --short HEAD) (Release 18.10.0 plus edge/v18 cuts)"
  echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo
  echo "Built with \`CGO_ENABLED=0\`, strip flags from Makefile \`BUILDFLAGS\`."
  echo
  echo "| binary | GOOS/GOARCH | bytes |"
  echo "|--------|-------------|-------|"
} > "$out"
measure() {
  local name="$1" os="$2" arch="$3" path="$4"
  if [[ -f "$path" ]]; then
    local n
    n="$(stat -c%s "$path")"
    echo "| $name | ${os}/${arch} | $n |" >> "$out"
    echo "$name ${os}/${arch}: $n"
  fi
}
measure teleport linux amd64 build/teleport
measure teleport-edge linux amd64 build/teleport-edge
measure teleport-edge linux arm64 build/teleport-edge-arm64
echo
echo "RSS: run integration/edge/run.sh with EDGE_ITEST_HOLD=1 and sample /proc/<pid>/status VmRSS."
echo "wrote $out"
