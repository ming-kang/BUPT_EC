# shellcheck shell=bash

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_eq() {
  local want="$1"
  local got="$2"
  local label="$3"
  if [[ "${got}" != "${want}" ]]; then
    fail "${label}: got '${got}', want '${want}'"
  fi
}

assert_invalid_version() {
  local version="$1"
  if validate_version "${version}" >/dev/null 2>&1; then
    fail "invalid VERSION was accepted: ${version}"
  fi
}

assert_path_absent() {
  local path="$1"
  local label="$2"
  if [[ -e "${path}" || -L "${path}" ]]; then
    fail "${label}: unexpected path remains at ${path}"
  fi
}

assert_mode() {
  local path="$1"
  local want="$2"
  local label="$3"
  local got
  if [[ "${POSIX_MODES_SUPPORTED}" == "false" ]]; then
    return
  fi
  got="$(stat -c '%a' "${path}")"
  assert_eq "${want}" "${got}" "${label} mode"
}

assert_contains() {
  local path="$1"
  local text="$2"
  local label="$3"
  if ! grep -Fq -- "${text}" "${path}"; then
    fail "${label}: '${text}' not found in ${path}"
  fi
}

assert_not_contains() {
  local path="$1"
  local text="$2"
  local label="$3"
  if grep -Fq -- "${text}" "${path}"; then
    fail "${label}: unexpected '${text}' found in ${path}"
  fi
}

assert_command_count() {
  local want="$1"
  local text="$2"
  local path="$3"
  local label="$4"
  local got
  got="$(grep -Fc -- "${text}" "${path}" || true)"
  if [[ "${got}" != "${want}" ]]; then
    echo "Command log for ${label}:" >&2
    sed 's/^/  /' "${path}" >&2
    fail "${label}: got '${got}', want '${want}'"
  fi
}
