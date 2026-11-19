#!/bin/bash
# Generate /etc/teleport/teleport.yaml from environment, then start the cluster.
set -euo pipefail

CONFIG_PATH="${TELEPORT_CONFIG_PATH:-/etc/teleport/teleport.yaml}"
DATA_DIR="${TELEPORT_DATA_DIR:-/var/lib/teleport}"
CERT_FILE="${TELEPORT_CERT_FILE:-/etc/teleport/certs/fullchain.pem}"
KEY_FILE="${TELEPORT_KEY_FILE:-/etc/teleport/certs/privkey.pem}"
WEB_PORT="${TELEPORT_WEB_PORT:-27443}"
WEB_LISTEN_ADDR="${TELEPORT_WEB_LISTEN_ADDR:-0.0.0.0:${WEB_PORT}}"
AUTH_LISTEN_ADDR="${TELEPORT_AUTH_LISTEN_ADDR:-127.0.0.1:3025}"
DIAG_LISTEN_ADDR="${TELEPORT_DIAG_LISTEN_ADDR:-127.0.0.1:3000}"
CLUSTER_NAME="${TELEPORT_CLUSTER_NAME:-teleport.example.com}"
PUBLIC_HOST="${TELEPORT_PUBLIC_HOST:-${CLUSTER_NAME}}"
PUBLIC_ADDR="${TELEPORT_PUBLIC_ADDR:-${PUBLIC_HOST}:${WEB_PORT}}"
NODENAME="${TELEPORT_NODENAME:-teleport}"
LOG_LEVEL="${TELEPORT_LOG_LEVEL:-info}"
SESSION_RECORDING="${TELEPORT_SESSION_RECORDING:-node-sync}"
SECOND_FACTOR="${TELEPORT_SECOND_FACTOR:-otp}"
HTTPS_RELOAD_INTERVAL="${TELEPORT_HTTPS_RELOAD_INTERVAL:-1h}"
SSH_ENABLED="${TELEPORT_SSH_ENABLED:-no}"

mkdir -p "${DATA_DIR}" /etc/teleport /etc/teleport/web-overlay/app /etc/teleport/certs

# Allow `docker run ... tctl ...` / `teleport version` without starting the cluster.
if [[ $# -gt 0 ]]; then
  case "$1" in
    tctl|tsh|teleport)
      exec "$@"
      ;;
    version|status|help|configure)
      exec teleport "$@"
      ;;
  esac
fi

if [[ "${TELEPORT_USE_MOUNTED_CONFIG:-}" == "1" && -s "${CONFIG_PATH}" ]]; then
  echo "Using mounted config ${CONFIG_PATH}"
else
  HTTPS_BLOCK=""
  if [[ -s "${CERT_FILE}" && -s "${KEY_FILE}" ]]; then
    HTTPS_BLOCK=$(cat <<EOF
  https_keypairs:
    - cert_file: ${CERT_FILE}
      key_file: ${KEY_FILE}
  https_keypairs_reload_interval: ${HTTPS_RELOAD_INTERVAL}
EOF
)
  else
    echo "No TLS keypair at ${CERT_FILE} / ${KEY_FILE}; Teleport will use a self-signed certificate."
  fi

  if [[ "${SECOND_FACTOR}" == "off" || "${SECOND_FACTOR}" == "false" || "${SECOND_FACTOR}" == "no" ]]; then
    export TELEPORT_ALLOW_NO_SECOND_FACTOR=true
  fi

  cat > "${CONFIG_PATH}" <<EOF
version: v3
teleport:
  nodename: ${NODENAME}
  data_dir: ${DATA_DIR}
  log:
    output: stderr
    severity: ${LOG_LEVEL}
    format:
      output: text
  diag_addr: ${DIAG_LISTEN_ADDR}
auth_service:
  enabled: yes
  listen_addr: ${AUTH_LISTEN_ADDR}
  cluster_name: ${CLUSTER_NAME}
  session_recording: ${SESSION_RECORDING}
  proxy_listener_mode: multiplex
  authentication:
    type: local
    second_factor: ${SECOND_FACTOR}
proxy_service:
  enabled: yes
  web_listen_addr: ${WEB_LISTEN_ADDR}
  public_addr: ${PUBLIC_ADDR}
${HTTPS_BLOCK}
ssh_service:
  enabled: ${SSH_ENABLED}
EOF
  echo "Wrote ${CONFIG_PATH} (public_addr=${PUBLIC_ADDR} web_listen_addr=${WEB_LISTEN_ADDR})"
fi

exec teleport start -c "${CONFIG_PATH}" "$@"
