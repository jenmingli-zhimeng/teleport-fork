#!/usr/bin/env bash
# Diff copied upstream sources between two git refs (default: previous merge
# parent vs HEAD). Non-zero exit if any watched file changed.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
FROM="${1:-}"
TO="${2:-HEAD}"
if [[ -z "$FROM" ]]; then
  FROM="$(git merge-base HEAD upstream/v18 2>/dev/null || git merge-base HEAD origin/master 2>/dev/null || git rev-parse HEAD^)"
fi
changed=0
while IFS= read -r path; do
  [[ -z "$path" || "$path" =~ ^# ]] && continue
  if ! git diff --quiet "$FROM" "$TO" -- "$path"; then
    echo "CHANGED $path"
    git log --oneline "$FROM..$TO" -- "$path" || true
    changed=1
  else
    echo "ok      $path"
  fi
done < build.assets/edge/WATCH_SOURCES
if [[ "$changed" -ne 0 ]]; then
  echo
  echo "Watched upstream files changed. Review lib/edgeagent and port security/reliability fixes."
  exit 1
fi
echo "No watched-file changes between $FROM and $TO."
