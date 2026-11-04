#!/usr/bin/env bash
# shellcheck shell=bash
# Compose exact release assets from already-built Linux binaries.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPOSITORY_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
readonly SCRIPT_DIR REPOSITORY_ROOT

usage() {
  echo "Usage: compose-release-assets.sh <release-directory> <vX.Y.Z|main-<short-sha>>" >&2
}

if (( $# != 2 )); then
  usage
  exit 2
fi

release_dir="$1"
version="$2"
if [[ "${release_dir}" != /* ]]; then
  release_dir="$(cd "${release_dir}" && pwd)"
fi
if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ &&
      ! "${version}" =~ ^main-[0-9a-f]+$ ]]; then
  echo "Release version must be a stable tag or main-<short-sha>." >&2
  exit 1
fi

for arch in amd64 arm64; do
  package_name="bupt-ec-linux-${arch}"
  package_dir="${release_dir}/${package_name}"
  if [[ -L "${package_dir}/bupt-ec" || ! -f "${package_dir}/bupt-ec" ]]; then
    echo "Missing regular ${package_name}/bupt-ec binary." >&2
    exit 1
  fi

  cp "${REPOSITORY_ROOT}/.env.example" "${REPOSITORY_ROOT}/README.md" \
    "${REPOSITORY_ROOT}/scripts/install.sh" "${package_dir}/"
  chmod 0755 "${package_dir}/install.sh"

  (
    cd "${release_dir}"
    tar -czf "${package_name}.tar.gz" "${package_name}"
  )
  tar -tzf "${release_dir}/${package_name}.tar.gz" | sort > "${release_dir}/${package_name}.actual"
  printf '%s\n' \
    "${package_name}/" \
    "${package_name}/.env.example" \
    "${package_name}/README.md" \
    "${package_name}/bupt-ec" \
    "${package_name}/install.sh" | sort > "${release_dir}/${package_name}.expected"
  diff -u "${release_dir}/${package_name}.expected" "${release_dir}/${package_name}.actual"
  tar -xOzf "${release_dir}/${package_name}.tar.gz" "${package_name}/install.sh" | \
    cmp - "${REPOSITORY_ROOT}/scripts/install.sh"
  packaged_installer_mode="$(tar -tvzf "${release_dir}/${package_name}.tar.gz" \
    "${package_name}/install.sh" | awk '{print $1}')"
  if [[ "${packaged_installer_mode}" != "-rwxr-xr-x" ]]; then
    echo "Packaged ${package_name}/install.sh is not mode 0755." >&2
    exit 1
  fi
  rm -f "${release_dir}/${package_name}.actual" "${release_dir}/${package_name}.expected"
done

cp "${REPOSITORY_ROOT}/scripts/install.sh" "${release_dir}/install.sh"
chmod 0755 "${release_dir}/install.sh"
[[ -x "${release_dir}/install.sh" ]] || {
  echo "Top-level install.sh is not executable." >&2
  exit 1
}
(
  cd "${release_dir}"
  sha256sum bupt-ec-linux-amd64.tar.gz bupt-ec-linux-arm64.tar.gz > checksums.txt
)

for asset in bupt-ec-linux-amd64.tar.gz bupt-ec-linux-arm64.tar.gz checksums.txt install.sh; do
  [[ -f "${release_dir}/${asset}" ]] || {
    echo "Missing release asset: ${asset}" >&2
    exit 1
  }
done

expected_top_level_assets=$'bupt-ec-linux-amd64.tar.gz\nbupt-ec-linux-arm64.tar.gz\nchecksums.txt\ninstall.sh'
actual_top_level_assets="$(find "${release_dir}" -maxdepth 1 -type f -printf '%f\n' | sort)"
if [[ "${actual_top_level_assets}" != "${expected_top_level_assets}" ]]; then
  echo "Release directory contains unexpected top-level assets." >&2
  exit 1
fi
