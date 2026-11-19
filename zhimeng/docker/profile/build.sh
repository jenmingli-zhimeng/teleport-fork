#!/bin/bash
# Build the cluster image on a machine that can reach docker.io / Go / pnpm,
# then save a complete tarball for offline load in China.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd -P)"
PROFILE="${ROOT}/.local/docker/profile"
TMP="${ROOT}/.local/docker/tmp"
IMAGES="${ROOT}/.local/docker/images"
CTX="${TMP}/cluster-ctx"
VERSION="${TELEPORT_VERSION:-18.10.0}"
IMAGE_NAME="${TELEPORT_IMAGE:-teleport-cluster:${VERSION}}"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
esac
TAR="${IMAGES}/teleport-cluster-${VERSION}-linux-${ARCH}.tar.gz"

cd "${ROOT}"
mkdir -p "${TMP}" "${IMAGES}" "${CTX}"

# Integration-test leftovers can point HOME at a scratch dir without write
# access to /usr/bin (corepack) or the real cargo/go caches.
if [[ "${HOME}" == *teleport-edge-itest* ]] || [[ ! -w "${HOME:-/}" ]]; then
  export HOME
  HOME="$(getent passwd "$(id -un)" | cut -d: -f6)"
fi
export HOME="${HOME:-/home/${USER}}"

# pnpm 11 (package.json packageManager) needs Node >= 22. Stage a local
# Node under .local/docker/tmp so we do not need root or a system upgrade.
NODE_VERSION="${TELEPORT_NODE_VERSION:-22.21.0}"
NODE_DIR="${TMP}/node-v${NODE_VERSION}-linux-x64"
if [[ ! -x "${NODE_DIR}/bin/node" ]]; then
  echo "==> Fetching Node ${NODE_VERSION} into ${NODE_DIR}"
  curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" \
    | tar -xJ -C "${TMP}"
fi
export PATH="${NODE_DIR}/bin:${HOME}/.local/bin:${HOME}/.cargo/bin:${PATH}"
hash -r
corepack enable
corepack prepare pnpm@11.3.0 --activate
echo "node $(node -v)  pnpm $(pnpm -v)"

echo "==> Building webassets and cluster binaries"
# This host can reach the network; the target server cannot.
unset WEBASSETS_SKIP_BUILD || true
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.25.11}"
make -C "${ROOT}" build/teleport build/tctl build/tsh

echo "==> Staging Docker build context in ${CTX}"
rm -rf "${CTX}"
mkdir -p "${CTX}"
cp -a "${ROOT}/build/teleport" "${ROOT}/build/tctl" "${ROOT}/build/tsh" "${CTX}/"
cp -a "${PROFILE}/entrypoint.sh" "${CTX}/entrypoint.sh"
chmod +x "${CTX}/teleport" "${CTX}/tctl" "${CTX}/tsh" "${CTX}/entrypoint.sh"

echo "==> Building ${IMAGE_NAME}"
# --pull=false: debian:bookworm-slim is expected to already exist locally (or
# have been pulled on this machine). The saved image includes every layer.
docker build --pull=false -f "${PROFILE}/Dockerfile" -t "${IMAGE_NAME}" "${CTX}"

echo "==> Saving ${TAR}"
docker save "${IMAGE_NAME}" | gzip -1 > "${TAR}"
ls -lh "${TAR}"
echo "Load on the target host with:"
echo "  gunzip -c ${TAR##*/} | docker load"
echo "  # or: docker load -i ${TAR##*/}"
