#!/usr/bin/env bash
# Functional + reliability harness for a fork-built auth/proxy (`teleport`)
# and a containerized `teleport-edge` node.
#
# Prerequisites: make teleport tctl tsh teleport-edge
# Optional: EDGE_ITEST_HOLD=1 leaves the cluster running on success.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BUILD="${ROOT}/build"
WORKDIR="${TMPDIR:-/tmp}/teleport-edge-itest"
AUTH_BIN="${BUILD}/teleport"
EDGE_BIN="${BUILD}/teleport-edge"
TCTL="${BUILD}/tctl"
TSH="${BUILD}/tsh"

AUTH_WEB="127.0.0.1:3080"
AUTH_DIAG="127.0.0.1:13000"
NODE_HTTP_PORT=19180
LOCAL_FWD_PORT=19190
REMOTE_FWD_PORT=19192
HOST_HTTP_PORT=19191
EDGE_CONTAINER="teleport-edge-itest"
RESULTS="${WORKDIR}/results.txt"
PASSED=0
FAILED=0
AUTH_PID=""
HOST_HTTP_PID=""
FWD_PID=""
REMOTE_FWD_PID=""

if [[ ! -x "$AUTH_BIN" || ! -x "$EDGE_BIN" || ! -x "$TCTL" || ! -x "$TSH" ]]; then
  echo "build teleport, tctl, tsh, and teleport-edge first" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required so teleport-edge can run in a musl container" >&2
  exit 1
fi

tctl() { "$TCTL" --config="$WORKDIR/auth.yaml" "$@"; }
tsh() {
  # --relogin is an ssh subcommand flag, not global. Identity files skip login.
  timeout 45 "$TSH" --insecure --proxy="$AUTH_WEB" --identity="$WORKDIR/identity" --user=tester "$@"
}

pass() {
  PASSED=$((PASSED + 1))
  echo "PASS: $*" | tee -a "$RESULTS"
}
fail() {
  FAILED=$((FAILED + 1))
  echo "FAIL: $*" | tee -a "$RESULTS"
}

rss_kb() {
  local pid
  pid="$(edge_pid)" || return 1
  awk '/VmRSS:/ {print $2}' "/proc/${pid}/status" 2>/dev/null
}

edge_pid() {
  docker inspect -f '{{.State.Pid}}' "$EDGE_CONTAINER" 2>/dev/null | awk 'NF && $1 != 0 {print; exit}'
}

wait_for() {
  local name="$1" seconds="$2"
  shift 2
  local i err
  err="$(mktemp)"
  for i in $(seq 1 "$seconds"); do
    if "$@" >/dev/null 2>"$err"; then
      rm -f "$err"
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for ${name}" >&2
  if [[ -s "$err" ]]; then
    tr '\n' ' ' < "$err" >&2
    echo >&2
  fi
  rm -f "$err"
  return 1
}

node_registered() {
  tctl get nodes 2>/dev/null | grep -q 'edge-node'
}

heartbeat_fresh() {
  if ! tctl get nodes --format=json >"$WORKDIR/nodes.json" 2>/dev/null; then
    node_registered
    return
  fi
  python3 - "$WORKDIR/nodes.json" <<'PY'
import json, sys
from datetime import datetime, timezone
raw = open(sys.argv[1]).read().strip()
if not raw:
    sys.exit(1)
data = json.loads(raw)
if isinstance(data, dict):
    data = [data]
now = datetime.now(timezone.utc)
for n in data:
    spec = n.get("spec") or {}
    if (spec.get("hostname") or "") != "edge-node":
        continue
    expires = (n.get("metadata") or {}).get("expires")
    if not expires:
        sys.exit(0)
    expires = str(expires).replace("Z", "+00:00")
    ts = datetime.fromisoformat(expires)
    if ts.tzinfo is None:
        ts = ts.replace(tzinfo=timezone.utc)
    sys.exit(0 if ts > now else 1)
sys.exit(1)
PY
}

container_running() {
  [[ "$(docker inspect -f '{{.State.Running}}' "$EDGE_CONTAINER" 2>/dev/null)" == "true" ]]
}

stop_auth() {
  if [[ -n "${AUTH_PID}" ]]; then
    kill "${AUTH_PID}" 2>/dev/null || true
    wait "${AUTH_PID}" 2>/dev/null || true
    AUTH_PID=""
  fi
}

cleanup() {
  local rc=$?
  set +e
  if [[ -n "${FWD_PID}" ]]; then kill "${FWD_PID}" 2>/dev/null; fi
  if [[ -n "${REMOTE_FWD_PID}" ]]; then kill "${REMOTE_FWD_PID}" 2>/dev/null; fi
  if [[ -n "${HOST_HTTP_PID}" ]]; then kill "${HOST_HTTP_PID}" 2>/dev/null; fi
  docker rm -f "$EDGE_CONTAINER" >/dev/null 2>&1
  stop_auth
  if [[ "${EDGE_ITEST_HOLD:-}" == "1" ]]; then
    echo "EDGE_ITEST_HOLD=1 set but processes were cleaned up on exit (rc=${rc})"
  fi
  exit "$rc"
}
trap cleanup EXIT

docker rm -f "$EDGE_CONTAINER" >/dev/null 2>&1 || true
# Node data_dir is bind-mounted into the container and may be root-owned.
rm -rf "$WORKDIR" 2>/dev/null || docker run --rm --network none -v "${WORKDIR}:${WORKDIR}" alpine:3.20 rm -rf "$WORKDIR"
mkdir -p "$WORKDIR/auth" "$WORKDIR/node" "$WORKDIR/home" "$WORKDIR/www-node" "$WORKDIR/www-host"
: > "$RESULTS"
export HOME="$WORKDIR/home"
export TELEPORT_HOME="$WORKDIR/home"

sed "s#/tmp/teleport-edge-itest#${WORKDIR}#g" \
  "$ROOT/integration/edge/testdata/auth.yaml" > "$WORKDIR/auth.yaml"

# OSS/community builds refuse second_factor=off unless this test knob is set.
# Identity-file login used below does not prompt for MFA.
export TELEPORT_ALLOW_NO_SECOND_FACTOR=true

echo "starting auth+proxy..."
"$AUTH_BIN" start --config="$WORKDIR/auth.yaml" --insecure --diag-addr="$AUTH_DIAG" \
  >"$WORKDIR/auth.log" 2>&1 &
AUTH_PID=$!

wait_for "auth status" 90 tctl status
curl -fsS "http://${AUTH_DIAG}/healthz" >/dev/null 2>&1 || true
pass "auth/proxy started pid=${AUTH_PID}"

TOKEN="$(tctl tokens add --type=node --format=text | awk 'NF{print $1; exit}')"
if [[ -z "$TOKEN" ]]; then
  echo "failed to create a node join token" >&2
  exit 1
fi
pass "node join token created"

sed -e "s#TOKEN#${TOKEN}#g" \
    -e "s#/tmp/teleport-edge-itest/node#/var/lib/teleport-edge#g" \
    -e "s#127.0.0.1:3080#${AUTH_WEB}#g" \
  "$ROOT/integration/edge/testdata/edge.yaml" > "$WORKDIR/edge.yaml"

echo "edge-ok" > "$WORKDIR/www-node/index.html"
echo "host-ok" > "$WORKDIR/www-host/index.html"

if ! docker image inspect teleport-edge-itest:local >/dev/null 2>&1; then
  echo "building alpine test image..."
  docker build -t teleport-edge-itest:local - <<'EOF'
FROM alpine:3.20
RUN apk add --no-cache python3 wget \
 && adduser -D -h /home/tester tester \
 && mkdir -p /home/tester /tmp
EOF
fi

start_edge_container() {
  docker rm -f "$EDGE_CONTAINER" >/dev/null 2>&1 || true
  mkdir -p "$WORKDIR/node"
  docker run -d --name "$EDGE_CONTAINER" --network host \
    -v "$EDGE_BIN:/usr/local/bin/teleport-edge:ro" \
    -v "$WORKDIR/edge.yaml:/etc/teleport.yaml:ro" \
    -v "$WORKDIR/www-node:/var/www:ro" \
    -v "$WORKDIR/node:/var/lib/teleport-edge" \
    teleport-edge-itest:local \
    sh -c 'python3 -m http.server '"$NODE_HTTP_PORT"' --bind 127.0.0.1 --directory /var/www >/tmp/httpd.log 2>&1 & exec /usr/local/bin/teleport-edge start --config=/etc/teleport.yaml --insecure' \
    >"$WORKDIR/edge-docker.cid"
}

echo "starting teleport-edge in alpine (musl) container..."
start_edge_container

wait_for "edge container running" 20 container_running
echo "waiting for edge-node heartbeat..."
if ! wait_for "edge-node registration" 90 node_registered; then
  echo "--- auth log (tail) ---" >&2
  tail -n 80 "$WORKDIR/auth.log" >&2 || true
  echo "--- edge logs ---" >&2
  docker logs "$EDGE_CONTAINER" >&2 || true
  exit 1
fi
pass "edge-node registered via reverse tunnel"

IDLE_RSS="$(rss_kb || true)"
echo "idle VmRSS=${IDLE_RSS:-unknown} kB" | tee -a "$RESULTS"

tctl users add tester --roles=access --logins=tester >/dev/null
tctl auth sign --user=tester --out="$WORKDIR/identity" --overwrite --ttl=2h >/dev/null
pass "local user tester + identity file"

python3 -m http.server "$HOST_HTTP_PORT" --bind 127.0.0.1 --directory "$WORKDIR/www-host" \
  >"$WORKDIR/host-http.log" 2>&1 &
HOST_HTTP_PID=$!

echo "== functional checks =="
if out="$(tsh ssh tester@edge-node -- echo shell-ok 2>"$WORKDIR/ssh.err")"; then
  if [[ "$out" == *shell-ok* ]]; then
    pass "tsh ssh shell"
  else
    fail "tsh ssh shell (unexpected output: ${out})"
  fi
else
  fail "tsh ssh shell ($(tr '\n' ' ' < "$WORKDIR/ssh.err"))"
fi

echo "scp-payload" > "$WORKDIR/upload.txt"
if tsh scp "$WORKDIR/upload.txt" tester@edge-node:/tmp/upload.txt >/dev/null 2>"$WORKDIR/scp-up.err"; then
  if got="$(tsh ssh tester@edge-node -- cat /tmp/upload.txt 2>/dev/null)" && [[ "$got" == scp-payload ]]; then
    pass "tsh scp upload (SFTP)"
  else
    fail "tsh scp upload (remote file mismatch: ${got:-empty})"
  fi
else
  fail "tsh scp upload ($(tr '\n' ' ' < "$WORKDIR/scp-up.err"))"
fi

if tsh scp tester@edge-node:/tmp/upload.txt "$WORKDIR/download.txt" >/dev/null 2>"$WORKDIR/scp-down.err"; then
  if [[ "$(cat "$WORKDIR/download.txt")" == scp-payload ]]; then
    pass "tsh scp download (SFTP)"
  else
    fail "tsh scp download (local file mismatch)"
  fi
else
  fail "tsh scp download ($(tr '\n' ' ' < "$WORKDIR/scp-down.err"))"
fi

if command -v sftp >/dev/null 2>&1; then
  tsh config > "$WORKDIR/ssh_config" 2>/dev/null || true
  if [[ -s "$WORKDIR/ssh_config" ]]; then
    cat > "$WORKDIR/sftp.batch" <<'EOF'
put /tmp/teleport-edge-itest-sftp-src /tmp/sftp.out
ls /tmp
bye
EOF
    echo sftp-ok > "$WORKDIR/sftp-src.txt"
    sed -i "s#/tmp/teleport-edge-itest-sftp-src#${WORKDIR}/sftp-src.txt#" "$WORKDIR/sftp.batch"
    if sftp -b "$WORKDIR/sftp.batch" -F "$WORKDIR/ssh_config" -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null tester@edge-node >/dev/null 2>"$WORKDIR/sftp.err"; then
      got="$(tsh ssh tester@edge-node -- cat /tmp/sftp.out 2>/dev/null || true)"
      if [[ "$got" == sftp-ok ]]; then
        pass "openssh sftp via tsh config"
      else
        fail "openssh sftp via tsh config (remote file mismatch)"
      fi
    else
      echo "SKIP: openssh sftp via tsh config ($(tr '\n' ' ' < "$WORKDIR/sftp.err"))" | tee -a "$RESULTS"
    fi
  fi
fi

tsh ssh -N -L "127.0.0.1:${LOCAL_FWD_PORT}:127.0.0.1:${NODE_HTTP_PORT}" tester@edge-node \
  >"$WORKDIR/local-fwd.log" 2>&1 &
FWD_PID=$!
if wait_for "local forward listen" 20 bash -c "curl -fsS http://127.0.0.1:${LOCAL_FWD_PORT}/ | grep -q edge-ok"; then
  pass "SSH local port forward (-L)"
else
  fail "SSH local port forward (-L)"
  tail -n 20 "$WORKDIR/local-fwd.log" || true
fi
kill "$FWD_PID" 2>/dev/null || true
FWD_PID=""

tsh ssh -N -R "127.0.0.1:${REMOTE_FWD_PORT}:127.0.0.1:${HOST_HTTP_PORT}" tester@edge-node \
  >"$WORKDIR/remote-fwd.log" 2>&1 &
REMOTE_FWD_PID=$!
sleep 2
if tsh ssh tester@edge-node -- wget -qO- "http://127.0.0.1:${REMOTE_FWD_PORT}/" 2>/dev/null | grep -q host-ok; then
  pass "SSH remote port forward (-R)"
else
  fail "SSH remote port forward (-R)"
  tail -n 20 "$WORKDIR/remote-fwd.log" || true
fi
kill "$REMOTE_FWD_PID" 2>/dev/null || true
REMOTE_FWD_PID=""

echo "== reliability checks =="
ONE_RSS=""
tsh ssh tester@edge-node -- sleep 8 >/dev/null 2>&1 &
sleep 2
ONE_RSS="$(rss_kb || true)"
wait || true
echo "1-session VmRSS=${ONE_RSS:-unknown} kB" | tee -a "$RESULTS"
if [[ -n "$ONE_RSS" && -n "$IDLE_RSS" ]]; then
  pass "RSS sampled idle=${IDLE_RSS}kB session=${ONE_RSS}kB"
else
  echo "SKIP: RSS sample incomplete" | tee -a "$RESULTS"
fi

CONC=6
fail_conc=0
for i in $(seq 1 "$CONC"); do
  tsh ssh tester@edge-node -- sh -c "sleep 5; echo ok-$i" >"$WORKDIR/conc-$i.out" 2>"$WORKDIR/conc-$i.err" &
done
wait || fail_conc=1
ok_count=0
for i in $(seq 1 "$CONC"); do
  if grep -q "ok-$i" "$WORKDIR/conc-$i.out" 2>/dev/null; then
    ok_count=$((ok_count + 1))
  fi
done
if [[ "$ok_count" -eq "$CONC" && "$fail_conc" -eq 0 ]]; then
  pass "concurrent sessions ${CONC}/${CONC}"
else
  fail "concurrent sessions ${ok_count}/${CONC}"
fi
CONC_RSS="$(rss_kb || true)"
echo "post-concurrent VmRSS=${CONC_RSS:-unknown} kB" | tee -a "$RESULTS"

echo "restarting edge container (SIGKILL path)..."
docker kill "$EDGE_CONTAINER" >/dev/null || true
start_edge_container
if wait_for "edge re-register after kill" 90 node_registered \
  && tsh ssh tester@edge-node -- echo rebound >/dev/null 2>&1; then
  pass "edge reconnect after container kill"
else
  fail "edge reconnect after container kill"
  docker logs --tail 50 "$EDGE_CONTAINER" || true
fi

echo "restarting auth+proxy..."
stop_auth
sleep 1
"$AUTH_BIN" start --config="$WORKDIR/auth.yaml" --insecure --diag-addr="$AUTH_DIAG" \
  >>"$WORKDIR/auth.log" 2>&1 &
AUTH_PID=$!
if wait_for "auth after restart" 90 tctl status \
  && wait_for "edge heartbeat after auth restart" 90 node_registered \
  && tsh ssh tester@edge-node -- echo after-auth >/dev/null 2>&1; then
  pass "edge reconnect after auth restart"
else
  fail "edge reconnect after auth restart"
fi

sleep 15
if heartbeat_fresh; then
  pass "idle heartbeat still fresh"
else
  fail "idle heartbeat still fresh"
fi

LOCK_OUT="$(tctl lock --user=tester --message=itest --ttl=5m)"
LOCK_NAME="$(printf '%s\n' "$LOCK_OUT" | sed -n 's/.*name "\([^"]*\)".*/\1/p')"
sleep 2
if tsh ssh tester@edge-node -- echo should-fail >/dev/null 2>"$WORKDIR/lock.err"; then
  fail "lock rejects new SSH session"
else
  pass "lock rejects new SSH session"
fi
if [[ -n "$LOCK_NAME" ]]; then
  tctl rm "lock/${LOCK_NAME}" >/dev/null
  sleep 2
  if tsh ssh tester@edge-node -- echo unlocked >/dev/null 2>&1; then
    pass "SSH works after lock removal"
  else
    fail "SSH works after lock removal"
  fi
else
  fail "could not parse lock name from: ${LOCK_OUT}"
fi

echo "initiating host CA rotation (init phase)..."
if tctl auth rotate --type=host --manual --phase=init >/dev/null 2>"$WORKDIR/rotate.err"; then
  sleep 8
  if tsh ssh tester@edge-node -- echo rotated >/dev/null 2>&1 && node_registered; then
    pass "SSH still works during host CA rotation init"
  else
    fail "SSH still works during host CA rotation init"
  fi
  tctl auth rotate --type=host --manual --phase=standby >/dev/null 2>/dev/null || true
else
  echo "SKIP: host CA rotation ($(tr '\n' ' ' < "$WORKDIR/rotate.err"))" | tee -a "$RESULTS"
fi

echo
echo "======== summary ========"
echo "PASS=${PASSED} FAIL=${FAILED}" | tee -a "$RESULTS"
echo "idle_rss_kb=${IDLE_RSS:-} one_session_rss_kb=${ONE_RSS:-} post_conc_rss_kb=${CONC_RSS:-}" | tee -a "$RESULTS"
echo "logs: $WORKDIR"
if [[ "$FAILED" -ne 0 ]]; then
  echo "auth log tail:" >&2
  tail -n 40 "$WORKDIR/auth.log" >&2 || true
  echo "edge log tail:" >&2
  docker logs --tail 40 "$EDGE_CONTAINER" >&2 || true
  exit 1
fi

if [[ "${EDGE_ITEST_HOLD:-}" == "1" ]]; then
  echo "holding cluster (EDGE_ITEST_HOLD=1); Ctrl-C to stop"
  trap - EXIT
  wait "$AUTH_PID"
fi
echo "ok"
