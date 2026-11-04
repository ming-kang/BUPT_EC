#!/usr/bin/env bash
# shellcheck shell=bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR

TEST_TMP="$(mktemp -d)"
trap 'rm -rf -- "${TEST_TMP}"' EXIT
release_dir="${TEST_TMP}/release"
mkdir -p "${release_dir}"

for arch in amd64 arm64; do
  package_dir="${release_dir}/bupt-ec-linux-${arch}"
  mkdir -p "${package_dir}"
  printf 'test binary for %s\n' "${arch}" > "${package_dir}/bupt-ec"
  chmod 0755 "${package_dir}/bupt-ec"
done

assert_layout() {
  local arch package_name expected actual packaged_installer_mode

  printf '%s\n' \
    bupt-ec-linux-amd64.tar.gz \
    bupt-ec-linux-arm64.tar.gz \
    checksums.txt \
    install.sh | sort > "${TEST_TMP}/top-assets.expected"
  find "${release_dir}" -maxdepth 1 -type f -printf '%f\n' | sort > "${TEST_TMP}/top-assets.actual"
  cmp "${TEST_TMP}/top-assets.expected" "${TEST_TMP}/top-assets.actual"
  (cd "${release_dir}" && sha256sum -c checksums.txt >/dev/null)

  for arch in amd64 arm64; do
    package_name="bupt-ec-linux-${arch}"
    expected="${TEST_TMP}/${package_name}.expected"
    actual="${TEST_TMP}/${package_name}.actual"
    printf '%s\n' \
      "${package_name}/" \
      "${package_name}/.env.example" \
      "${package_name}/README.md" \
      "${package_name}/bupt-ec" \
      "${package_name}/install.sh" | sort > "${expected}"
    tar -tzf "${release_dir}/${package_name}.tar.gz" | sort > "${actual}"
    cmp "${expected}" "${actual}"
    packaged_installer_mode="$(tar -tvzf "${release_dir}/${package_name}.tar.gz" \
      "${package_name}/install.sh" | awk '{print $1}')"
    [[ "${packaged_installer_mode}" == "-rwxr-xr-x" ]]
    tar -xOzf "${release_dir}/${package_name}.tar.gz" "${package_name}/install.sh" | \
      cmp - "${SCRIPT_DIR}/install.sh"
    grep -Fq "${package_name}.tar.gz" "${release_dir}/checksums.txt"
  done
  cmp "${SCRIPT_DIR}/install.sh" "${release_dir}/install.sh"
  [[ -x "${release_dir}/install.sh" ]]
}

bash "${SCRIPT_DIR}/compose-release-assets.sh" "${release_dir}" v0.3.0
assert_layout
bash "${SCRIPT_DIR}/compose-release-assets.sh" "${release_dir}" main-deadbeef
assert_layout

printf 'release layout tests passed (stable and main version injection).\n'
