#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=install.sh
source "${SCRIPT_DIR}/install.sh"

assert_eq() {
  local want="$1"
  local got="$2"
  local label="$3"
  if [[ "${got}" != "${want}" ]]; then
    echo "FAIL: ${label}: got '${got}', want '${want}'" >&2
    exit 1
  fi
}

assert_invalid_version() {
  local version="$1"
  if validate_version "${version}" >/dev/null 2>&1; then
    echo "FAIL: invalid VERSION was accepted: ${version}" >&2
    exit 1
  fi
}

assert_eq "nightly" "$(resolve_release_version "" "")" "first install defaults to nightly"
assert_eq "latest" "$(resolve_release_version "latest" "nightly")" "explicit version wins"
assert_eq "v0.1.4" "$(resolve_release_version "" "v0.1.4")" "saved version is reused"

for version in latest nightly v0.1.4; do
  validate_version "${version}"
done
for version in "" latest/asset v1 v1.2 v1.2.3.4 'v1.2.3;rm'; do
  assert_invalid_version "${version}"
done

# Keep URL tests deterministic and offline.
host_reachable() { return 0; }
assert_eq \
  "https://github.com/ming-kang/BUPT_EC/releases/latest/download" \
  "$(resolve_download_base_url "ming-kang/BUPT_EC" "latest" "")" \
  "latest release URL"
assert_eq \
  "https://github.com/ming-kang/BUPT_EC/releases/download/nightly" \
  "$(resolve_download_base_url "ming-kang/BUPT_EC" "nightly" "")" \
  "nightly release URL"
assert_eq \
  "https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.4" \
  "$(resolve_download_base_url "ming-kang/BUPT_EC" "v0.1.4" "")" \
  "stable tag release URL"
assert_eq \
  "https://mirror.example/releases/v0.1.4" \
  "$(resolve_download_base_url "ignored/repo" "nightly" "https://mirror.example/releases/v0.1.4/")" \
  "custom download URL"

echo "install version tests passed"
