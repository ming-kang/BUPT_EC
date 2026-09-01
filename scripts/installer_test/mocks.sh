# shellcheck shell=bash
# shellcheck disable=SC2034


TEST_TMP="$(mktemp -d)"
chmod 0700 "${TEST_TMP}"
trap 'rm -rf "${TEST_TMP}"' EXIT

POSIX_MODES_SUPPORTED=true
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*)
    POSIX_MODES_SUPPORTED=false
    ;;
esac

# Windows-backed test files cannot represent chmod 0600. Keep production's
# exact mode check intact while making only the sourced fixture's stat result
# portable; individual security tests can force an unsafe synthetic mode.
if [[ "${POSIX_MODES_SUPPORTED}" == "false" ]]; then
  stat() {
    if [[ "${1:-}" == "-c" && "${2:-}" == "%a" && "${3:-}" == "--" && "${4:-}" == "${ENV_FILE}" ]]; then
      printf '%s\n' "${CURRENT_CONFIG_TEST_STAT_MODE:-600}"
      return 0
    fi
    command stat "$@"
  }
fi

MOCK_BIN="${TEST_TMP}/mock-bin"
mkdir -p "${MOCK_BIN}"

cat > "${MOCK_BIN}/chown" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'chown %s\n' "$*" >> "${MOCK_COMMAND_LOG}"
if [[ -n "${MOCK_CHOWN_FAIL_PATTERN:-}" && "$*" == *"${MOCK_CHOWN_FAIL_PATTERN}"* ]]; then
  exit 1
fi
EOF

cat > "${MOCK_BIN}/cp" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'cp %s\n' "$*" >> "${MOCK_COMMAND_LOG}"
if [[ -n "${MOCK_CP_FAIL_PATTERN:-}" && "$*" == *"${MOCK_CP_FAIL_PATTERN}"* ]]; then
  exit 1
fi
command -p cp "$@"
EOF

cat > "${MOCK_BIN}/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

command_name="${1:-}"
printf 'systemctl %s\n' "$*" >> "${MOCK_COMMAND_LOG}"

active_file="${MOCK_STATE_DIR}/service.active"
reload_count_file="${MOCK_STATE_DIR}/nginx-reload.count"

if [[ "${MOCK_SYSTEMCTL_FAIL_COMMAND:-}" == "${command_name}" ]]; then
  counter_file="${MOCK_STATE_DIR}/systemctl-${command_name}.count"
  count=0
  if [[ -f "${counter_file}" ]]; then
    read -r count < "${counter_file}"
  fi
  count=$((count + 1))
  printf '%s\n' "${count}" > "${counter_file}"
  if (( count == ${MOCK_SYSTEMCTL_FAIL_ON_CALL:-1} )); then
    exit 1
  fi
fi

case "${command_name}" in
  is-active)
    if [[ -f "${active_file}" ]]; then
      exit 0
    fi
    exit 3
    ;;
  is-enabled)
    if [[ -L "${MOCK_SYSTEMD_ENABLED_LINK}" || -e "${MOCK_SYSTEMD_ENABLED_LINK}" ]]; then
      exit 0
    fi
    exit 1
    ;;
  enable)
    mkdir -p "$(dirname "${MOCK_SYSTEMD_ENABLED_LINK}")"
    rm -f -- "${MOCK_SYSTEMD_ENABLED_LINK}"
    ln -s "${MOCK_SERVICE_FILE}" "${MOCK_SYSTEMD_ENABLED_LINK}"
    ;;
  disable)
    rm -f -- "${MOCK_SYSTEMD_ENABLED_LINK}"
    ;;
  start | restart)
    : > "${active_file}"
    ;;
  stop)
    rm -f -- "${active_file}"
    ;;
  reload)
    if [[ "${2:-}" == "nginx" ]]; then
      count=0
      if [[ -f "${reload_count_file}" ]]; then
        read -r count < "${reload_count_file}"
      fi
      count=$((count + 1))
      printf '%s\n' "${count}" > "${reload_count_file}"
    fi
    ;;
  daemon-reload)
    ;;
esac
EOF

cat > "${MOCK_BIN}/nginx" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf 'nginx %s\n' "$*" >> "${MOCK_COMMAND_LOG}"
if [[ "${1:-}" == "-t" && -n "${MOCK_NGINX_FAIL_ON_TEST_CALL:-}" ]]; then
  counter_file="${MOCK_STATE_DIR}/nginx-test.count"
  count=0
  if [[ -f "${counter_file}" ]]; then
    read -r count < "${counter_file}"
  fi
  count=$((count + 1))
  printf '%s\n' "${count}" > "${counter_file}"
  if (( count == MOCK_NGINX_FAIL_ON_TEST_CALL )); then
    exit 1
  fi
fi
EOF

cat > "${MOCK_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

output=""
url=""
while (( $# > 0 )); do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    --connect-timeout | --max-time | --noproxy | --proto | --proto-redir)
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

printf 'curl %s\n' "${url}" >> "${MOCK_COMMAND_LOG}"

if [[ "${url}" == http://*/healthz ]]; then
  counter_file="${MOCK_STATE_DIR}/health.count"
  count=0
  if [[ -f "${counter_file}" ]]; then
    read -r count < "${counter_file}"
  fi
  count=$((count + 1))
  printf '%s\n' "${count}" > "${counter_file}"
  if (( count <= ${MOCK_HEALTH_FAILURES:-0} )); then
    exit 22
  fi
  exit 0
fi

case "${url}" in
  *.tar.gz)
    cp "${MOCK_ARCHIVE_SOURCE:?}" "${output:?}"
    ;;
  */checksums.txt)
    if [[ "${MOCK_CURL_CHECKSUM_MODE:-copy}" == "missing" ]]; then
      exit 22
    fi
    cp "${MOCK_CHECKSUM_SOURCE:?}" "${output:?}"
    ;;
  *)
    ;;
esac
EOF

cat > "${MOCK_BIN}/sleep" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'sleep %s\n' "$*" >> "${MOCK_COMMAND_LOG}"
EOF

chmod 0755 "${MOCK_BIN}"/*
export PATH="${MOCK_BIN}:${PATH}"

VALID_ARCHIVE="${TEST_TMP}/bupt-ec-linux-amd64.tar.gz"
MISSING_BINARY_ARCHIVE="${TEST_TMP}/missing-binary.tar.gz"
VALID_CHECKSUMS="${TEST_TMP}/checksums-valid.txt"
MISSING_ENTRY_CHECKSUMS="${TEST_TMP}/checksums-missing-entry.txt"
MISMATCH_CHECKSUMS="${TEST_TMP}/checksums-mismatch.txt"

create_release_archive() {
  local destination="$1"
  local include_binary="$2"
  local source_dir
  source_dir="$(mktemp -d "${TEST_TMP}/archive.XXXXXX")"
  mkdir -p "${source_dir}/bupt-ec-linux-amd64"
  if [[ "${include_binary}" == "true" ]]; then
    printf 'candidate binary\n' > "${source_dir}/bupt-ec-linux-amd64/bupt-ec"
    chmod 0755 "${source_dir}/bupt-ec-linux-amd64/bupt-ec"
  else
    printf 'archive without binary\n' > "${source_dir}/bupt-ec-linux-amd64/README.md"
  fi
  tar -czf "${destination}" -C "${source_dir}" bupt-ec-linux-amd64
  rm -rf "${source_dir}"
}

create_release_archive "${VALID_ARCHIVE}" true
create_release_archive "${MISSING_BINARY_ARCHIVE}" false
printf '%s  bupt-ec-linux-amd64.tar.gz\n' "$(sha256sum "${VALID_ARCHIVE}" | awk '{print $1}')" > "${VALID_CHECKSUMS}"
printf '%s  another-package.tar.gz\n' "$(sha256sum "${VALID_ARCHIVE}" | awk '{print $1}')" > "${MISSING_ENTRY_CHECKSUMS}"
printf '%064d  bupt-ec-linux-amd64.tar.gz\n' 0 > "${MISMATCH_CHECKSUMS}"

reset_mock_state() {
  local case_dir="$1"
  MOCK_STATE_DIR="${case_dir}/mock-state"
  MOCK_COMMAND_LOG="${case_dir}/commands.log"
  mkdir -p "${MOCK_STATE_DIR}"
  : > "${MOCK_COMMAND_LOG}"
  export MOCK_STATE_DIR MOCK_COMMAND_LOG
  export MOCK_ARCHIVE_SOURCE="${VALID_ARCHIVE}"
  export MOCK_CHECKSUM_SOURCE="${VALID_CHECKSUMS}"
  export MOCK_CURL_CHECKSUM_MODE=copy
  export MOCK_HEALTH_FAILURES=0
  unset MOCK_NGINX_FAIL_ON_TEST_CALL
  unset MOCK_SYSTEMCTL_FAIL_COMMAND
  unset MOCK_SYSTEMCTL_FAIL_ON_CALL
  unset MOCK_CHOWN_FAIL_PATTERN
  unset MOCK_CP_FAIL_PATTERN
  unset CURRENT_CONFIG_TEST_STAT_MODE
}

setup_case() {
  local case_dir="$1"
  mkdir -p "${case_dir}/root"
  configure_installer_test_root "${case_dir}/root"
  export MOCK_SERVICE_FILE="${SERVICE_FILE}"
  export MOCK_SYSTEMD_ENABLED_LINK="${SYSTEMD_ENABLED_LINK}"
  reset_config_state
  TRANSACTION_ACTIVE=false
  TRANSACTION_BACKUP_DIR=""
  reset_mock_state "${case_dir}"
}

set_valid_test_config() {
  CFG_RELEASE_REPO="ming-kang/BUPT_EC"
  CFG_RELEASE_VERSION="v9.9.9"
  CFG_DOMAIN="classroom.example.com"
  CFG_SSL_CERT="/etc/tls/fullchain.pem"
  CFG_SSL_KEY="/etc/tls/privkey.pem"
  CFG_JW_USERNAME="test-user"
  CFG_JW_PASSWORD="test-password"
  CFG_JW_TOKEN=""
  CFG_APP_ADDR="127.0.0.1:8080"
  CFG_DOWNLOAD_BASE_URL=""
  CFG_LOG_CALLER=""
  CFG_READYZ_DIAGNOSTICS=""
}

write_valid_current_config() {
  local cert_path="$1"
  local key_path="$2"
  local version="${3:-v9.9.9}"

  set_valid_test_config
  CFG_RELEASE_VERSION="${version}"
  CFG_SSL_CERT="${cert_path}"
  CFG_SSL_KEY="${key_path}"
  CFG_DOWNLOAD_BASE_URL="https://mirror.example/releases"
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"
}

seed_existing_installation() {
  local service_active="${1:-true}"
  local service_enabled="${2:-true}"

  mkdir -p "${INSTALL_DIR}/run_log" "${CONFIG_DIR}" \
    "$(dirname "${SERVICE_FILE}")" "$(dirname "${SYSTEMD_ENABLED_LINK}")" \
    "$(dirname "${NGINX_SITE}")" "$(dirname "${NGINX_ENABLED}")"
  printf 'old binary\n' > "${INSTALL_DIR}/bupt-ec"
  printf 'old env\n' > "${ENV_FILE}"
  printf 'old service\n' > "${SERVICE_FILE}"
  printf 'old nginx\n' > "${NGINX_SITE}"
  chmod 0755 "${INSTALL_DIR}/bupt-ec"
  chmod 0600 "${ENV_FILE}"
  chmod 0644 "${SERVICE_FILE}" "${NGINX_SITE}"
  rm -f -- "${SYSTEMD_ENABLED_LINK}"
  if [[ "${service_enabled}" == "true" ]]; then
    ln -s "${SERVICE_FILE}" "${SYSTEMD_ENABLED_LINK}"
  fi
  ln -s "${NGINX_SITE}" "${NGINX_ENABLED}"
  if [[ "${service_active}" == "true" ]]; then
    : > "${MOCK_STATE_DIR}/service.active"
  else
    rm -f -- "${MOCK_STATE_DIR}/service.active"
  fi
}

assert_service_active() {
  local want="$1"
  local label="$2"
  if [[ "${want}" == "true" ]]; then
    if [[ ! -f "${MOCK_STATE_DIR}/service.active" ]]; then
      fail "${label}: service is inactive, want active"
    fi
  elif [[ -f "${MOCK_STATE_DIR}/service.active" ]]; then
    fail "${label}: service is active, want inactive"
  fi
}

assert_service_enabled() {
  local want="$1"
  local label="$2"
  if [[ "${want}" == "true" ]]; then
    if [[ ! -L "${SYSTEMD_ENABLED_LINK}" && ! -e "${SYSTEMD_ENABLED_LINK}" ]]; then
      fail "${label}: service is disabled, want enabled"
    fi
  elif [[ -L "${SYSTEMD_ENABLED_LINK}" || -e "${SYSTEMD_ENABLED_LINK}" ]]; then
    fail "${label}: service is enabled, want disabled"
  fi
}

capture_target_state() {
  local role target checksum mode
  while IFS=$'\t' read -r role target; do
    if [[ -L "${target}" ]]; then
      printf '%s\tlink\t%s\n' "${role}" "$(readlink "${target}")"
    elif [[ -f "${target}" ]]; then
      checksum="$(sha256sum "${target}" | awk '{print $1}')"
      mode="$(stat -c '%a' "${target}")"
      printf '%s\tfile\t%s\t%s\n' "${role}" "${checksum}" "${mode}"
    elif [[ -e "${target}" ]]; then
      printf '%s\tother\n' "${role}"
    else
      printf '%s\tabsent\n' "${role}"
    fi
  done < <(transaction_targets)
}

make_staging() {
  local staging_dir="$1"
  rm -rf "${staging_dir}"
  mkdir -p "${staging_dir}"
  chmod 0700 "${staging_dir}"
  printf 'new binary\n' > "${staging_dir}/bupt-ec"
  chmod 0755 "${staging_dir}/bupt-ec"
  chown root:root "${staging_dir}/bupt-ec"
  set_valid_test_config
  render_env_file "${staging_dir}/bupt-ec.env"
  render_systemd_service "${staging_dir}/${SERVICE_NAME}.service"
  render_nginx_site "${staging_dir}/${SERVICE_NAME}.conf"
}

run_transaction_with_cleanup() {
  local session_dir="$1"
  local staging_dir="$2"
  local backup_dir="$3"
  local status

  (
    set +e
    initialize_installer_session "${session_dir}"
    perform_install_transaction "${staging_dir}" "${backup_dir}" "127.0.0.1:8080"
    status=$?
    exit "${status}"
  )
}

assert_enabled_target() {
  local path="$1"
  local target="$2"
  local label="$3"
  if [[ -L "${path}" ]]; then
    assert_eq "${target}" "$(readlink "${path}")" "${label} link target"
  elif ! cmp -s "${path}" "${target}"; then
    fail "${label}: ${path} is neither a symlink nor an equivalent MSYS copy"
  fi
}
