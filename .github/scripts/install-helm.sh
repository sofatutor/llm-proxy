#!/usr/bin/env bash
set -euo pipefail

HELM_VERSION="${1:-v3.15.4}"

if [[ -z "${HELM_VERSION}" ]]; then
  echo "usage: $0 <helm_version>" >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required" >&2
  exit 1
fi

if ! command -v tar >/dev/null 2>&1; then
  echo "ERROR: tar is required" >&2
  exit 1
fi

if ! command -v sha256sum >/dev/null 2>&1; then
  echo "ERROR: sha256sum is required" >&2
  exit 1
fi

HELM_TARBALL="helm-${HELM_VERSION}-linux-amd64.tar.gz"
HELM_URL="https://get.helm.sh/${HELM_TARBALL}"
HELM_SHA_URL="${HELM_URL}.sha256"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

echo "Downloading ${HELM_URL}..."
curl -fsSLo "${tmpdir}/${HELM_TARBALL}" "${HELM_URL}"
curl -fsSLo "${tmpdir}/${HELM_TARBALL}.sha256" "${HELM_SHA_URL}"

(
  cd "${tmpdir}"
  expected_sha256="$(tr -d '\n' < "${HELM_TARBALL}.sha256" | awk '{print $1}')"
  if [[ -z "${expected_sha256}" ]]; then
    echo "ERROR: empty checksum file for ${HELM_TARBALL}" >&2
    exit 1
  fi
  echo "${expected_sha256}  ${HELM_TARBALL}" | sha256sum -c -
)

tar -xzf "${tmpdir}/${HELM_TARBALL}" -C "${tmpdir}"

install_dir="/usr/local/bin"
helm_bin_src="${tmpdir}/linux-amd64/helm"

if [[ ! -f "${helm_bin_src}" ]]; then
  echo "ERROR: expected helm binary at ${helm_bin_src}" >&2
  exit 1
fi

if command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "${helm_bin_src}" "${install_dir}/helm"
else
  if [[ "$(id -u)" -eq 0 ]]; then
    install -m 0755 "${helm_bin_src}" "${install_dir}/helm"
  else
    install_dir="${HOME}/.local/bin"
    mkdir -p "${install_dir}"
    install -m 0755 "${helm_bin_src}" "${install_dir}/helm"
    echo "NOTE: installed helm to ${install_dir}; ensure it is on PATH" >&2
  fi
fi

helm version --short
