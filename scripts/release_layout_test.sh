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
  local version="$1"
  local arch package_name expected actual marker_count packaged_cli_mode packaged_installer_mode

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
      "${package_name}/bupt-ec-cli" \
      "${package_name}/install.sh" | sort > "${expected}"
    tar -tzf "${release_dir}/${package_name}.tar.gz" | sort > "${actual}"
    cmp "${expected}" "${actual}"
    tar -xOzf "${release_dir}/${package_name}.tar.gz" "${package_name}/bupt-ec-cli" | \
      grep -Fqx "CLI_BUILD_VERSION=\"${version}\" # BUPT_EC_CLI_BUILD_VERSION"
    marker_count="$(tar -xOzf "${release_dir}/${package_name}.tar.gz" "${package_name}/bupt-ec-cli" | \
      grep -Fc 'BUPT_EC_CLI_BUILD_VERSION' || true)"
    [[ "${marker_count}" == 1 ]]
    packaged_cli_mode="$(tar -tvzf "${release_dir}/${package_name}.tar.gz" \
      "${package_name}/bupt-ec-cli" | awk '{print $1}')"
    [[ "${packaged_cli_mode}" == "-rwxr-xr-x" ]]
    packaged_installer_mode="$(tar -tvzf "${release_dir}/${package_name}.tar.gz" \
      "${package_name}/install.sh" | awk '{print $1}')"
    [[ "${packaged_installer_mode}" == "-rwxr-xr-x" ]]
    tar -xOzf "${release_dir}/${package_name}.tar.gz" "${package_name}/install.sh" | \
      cmp - "${SCRIPT_DIR}/install.sh"
    [[ -x "${release_dir}/${package_name}/bupt-ec-cli" ]]
    grep -Fq "${package_name}.tar.gz" "${release_dir}/checksums.txt"
  done
  cmp "${release_dir}/bupt-ec-linux-amd64/bupt-ec-cli" \
    "${release_dir}/bupt-ec-linux-arm64/bupt-ec-cli"
  cmp "${SCRIPT_DIR}/install.sh" "${release_dir}/install.sh"
  [[ -x "${release_dir}/install.sh" ]]
  [[ ! -e "${release_dir}/bupt-ec-cli" && ! -L "${release_dir}/bupt-ec-cli" ]]
}

bash "${SCRIPT_DIR}/generate-install.sh" --check
bash "${SCRIPT_DIR}/compose-release-assets.sh" "${release_dir}" v0.3.0
assert_layout v0.3.0
bash "${SCRIPT_DIR}/compose-release-assets.sh" "${release_dir}" main-deadbeef
assert_layout main-deadbeef
grep -Fqx 'CLI_BUILD_VERSION="dev" # BUPT_EC_CLI_BUILD_VERSION' "${SCRIPT_DIR}/bupt-ec-cli.sh"

printf 'release layout tests passed (stable and main version injection).\n'
