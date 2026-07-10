#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source-path=SCRIPTDIR
# shellcheck source=install.sh
source "${SCRIPT_DIR}/install.sh"

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

TEST_TMP="$(mktemp -d)"
chmod 0700 "${TEST_TMP}"
trap 'rm -rf "${TEST_TMP}"' EXIT

POSIX_MODES_SUPPORTED=true
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*)
    POSIX_MODES_SUPPORTED=false
    ;;
esac

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

if [[ "${command_name}" == "enable" ]]; then
  mkdir -p "$(dirname "${MOCK_SYSTEMD_ENABLED_LINK}")"
  rm -f -- "${MOCK_SYSTEMD_ENABLED_LINK}"
  ln -s "${MOCK_SERVICE_FILE}" "${MOCK_SYSTEMD_ENABLED_LINK}"
fi
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
    --connect-timeout | --max-time | --noproxy)
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
}

setup_case() {
  local case_dir="$1"
  mkdir -p "${case_dir}/root"
  configure_installer_test_root "${case_dir}/root"
  export MOCK_SERVICE_FILE="${SERVICE_FILE}"
  export MOCK_SYSTEMD_ENABLED_LINK="${SYSTEMD_ENABLED_LINK}"
  TRANSACTION_ACTIVE=false
  TRANSACTION_BACKUP_DIR=""
  reset_mock_state "${case_dir}"
}

seed_existing_installation() {
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
  ln -s "${SERVICE_FILE}" "${SYSTEMD_ENABLED_LINK}"
  ln -s "${NGINX_SITE}" "${NGINX_ENABLED}"
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
  render_env_file "${staging_dir}/bupt-ec.env" \
    "ming-kang/BUPT_EC" "v9.9.9" "classroom.example.com" \
    "/etc/tls/fullchain.pem" "/etc/tls/privkey.pem" \
    "test-user" "test-password" "" "127.0.0.1:8080" "release" ""
  render_systemd_service "${staging_dir}/${SERVICE_NAME}.service"
  render_nginx_site "${staging_dir}/${SERVICE_NAME}.conf" \
    "classroom.example.com" "/etc/tls/fullchain.pem" "/etc/tls/privkey.pem" "127.0.0.1:8080"
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

test_version_policy() {
  assert_eq "nightly" "$(resolve_release_version "" "")" "first install defaults to nightly"
  assert_eq "latest" "$(resolve_release_version "latest" "nightly")" "explicit version wins"
  assert_eq "v0.1.4" "$(resolve_release_version "" "v0.1.4")" "saved version is reused"

  local version
  for version in latest nightly v0.1.4; do
    validate_version "${version}"
  done
  for version in "" latest/asset v1 v1.2 v1.2.3.4 'v1.2.3;rm'; do
    assert_invalid_version "${version}"
  done

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
}

test_checksum_failures_preserve_targets() {
  local scenario case_dir before after status output work_dir
  for scenario in download-missing entry-missing mismatch; do
    case_dir="${TEST_TMP}/checksum-${scenario}"
    mkdir -p "${case_dir}"
    setup_case "${case_dir}"
    seed_existing_installation
    before="$(capture_target_state)"
    work_dir="${case_dir}/work"
    output="${case_dir}/output.log"
    mkdir -p "${work_dir}"

    case "${scenario}" in
      download-missing)
        export MOCK_CURL_CHECKSUM_MODE=missing
        ;;
      entry-missing)
        export MOCK_CHECKSUM_SOURCE="${MISSING_ENTRY_CHECKSUMS}"
        ;;
      mismatch)
        export MOCK_CHECKSUM_SOURCE="${MISMATCH_CHECKSUMS}"
        ;;
    esac

    set +e
    (download_release "ming-kang/BUPT_EC" "nightly" amd64 "${work_dir}" "https://mirror.example/nightly") > "${output}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "checksum ${scenario} unexpectedly succeeded"
    fi
    after="$(capture_target_state)"
    assert_eq "${before}" "${after}" "checksum ${scenario} preserves installed targets"
    assert_not_contains "${output}" "BUPT_EC is installed." "checksum ${scenario} success output"
  done
}

test_staging_failures_preserve_targets() {
  local case_dir before after status output work_dir staging_dir

  case_dir="${TEST_TMP}/archive-missing-binary"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  before="$(capture_target_state)"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  output="${case_dir}/output.log"
  mkdir -p "${work_dir}"
  set +e
  prepare_staging "${MISSING_BINARY_ARCHIVE}" "${work_dir}" "${staging_dir}" \
    "ming-kang/BUPT_EC" nightly classroom.example.com /cert /key user password "" \
    "127.0.0.1:8080" release "" > "${output}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "archive without binary unexpectedly staged"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "archive failure preserves installed targets"

  case_dir="${TEST_TMP}/render-failure"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  before="$(capture_target_state)"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  output="${case_dir}/output.log"
  mkdir -p "${work_dir}"
  export MOCK_CHOWN_FAIL_PATTERN="${staging_dir}/bupt-ec.env"
  set +e
  prepare_staging "${VALID_ARCHIVE}" "${work_dir}" "${staging_dir}" \
    "ming-kang/BUPT_EC" nightly classroom.example.com /cert /key user password "" \
    "127.0.0.1:8080" release "" > "${output}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "render failure unexpectedly staged all candidates"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "render failure preserves installed targets"
}

test_snapshot_failure_preserves_targets() {
  local case_dir staging_dir backup_dir output before after status
  case_dir="${TEST_TMP}/snapshot-failure"
  staging_dir="${case_dir}/staging"
  backup_dir="${case_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}"
  before="$(capture_target_state)"
  export MOCK_CP_FAIL_PATTERN="${ENV_FILE}"

  set +e
  perform_install_transaction "${staging_dir}" "${backup_dir}" "127.0.0.1:8080" > "${output}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "snapshot copy failure unexpectedly entered commit"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "snapshot failure preserves installed targets"
  assert_eq false "${TRANSACTION_ACTIVE}" "snapshot failure transaction active flag"
  assert_eq "" "${TRANSACTION_BACKUP_DIR}" "snapshot failure backup pointer"
  assert_command_count 0 "systemctl " "${MOCK_COMMAND_LOG}" "snapshot failure system command count"
}

test_nginx_failure_rolls_back_upgrade() {
  local case_dir session_dir staging_dir backup_dir output before after
  case_dir="${TEST_TMP}/nginx-rollback"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}"
  before="$(capture_target_state)"
  export MOCK_NGINX_FAIL_ON_TEST_CALL=1

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "nginx validation failure unexpectedly committed"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "nginx failure restores installed targets"
  assert_command_count 2 "nginx -t" "${MOCK_COMMAND_LOG}" "nginx validation plus rollback validation count"
  assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "old service restart after nginx failure"
  assert_contains "${output}" "Rollback completed." "nginx rollback output"
  assert_path_absent "${session_dir}" "completed nginx rollback session cleanup"
}

test_restart_and_health_failures_roll_back_upgrade() {
  local failure case_dir session_dir staging_dir backup_dir output before after
  for failure in restart health; do
    case_dir="${TEST_TMP}/${failure}-rollback"
    session_dir="${case_dir}/session"
    staging_dir="${session_dir}/staging"
    backup_dir="${session_dir}/backup"
    output="${case_dir}/output.log"
    mkdir -p "${session_dir}"
    chmod 0700 "${session_dir}"
    setup_case "${case_dir}"
    seed_existing_installation
    make_staging "${staging_dir}"
    before="$(capture_target_state)"

    if [[ "${failure}" == "restart" ]]; then
      export MOCK_SYSTEMCTL_FAIL_COMMAND=restart
      export MOCK_SYSTEMCTL_FAIL_ON_CALL=1
    else
      export MOCK_HEALTH_FAILURES=10
    fi

    if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
      fail "${failure} failure unexpectedly committed"
    fi
    after="$(capture_target_state)"
    assert_eq "${before}" "${after}" "${failure} failure restores installed targets"
    assert_command_count 2 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "${failure} path restarts new then old service"
    if [[ "${failure}" == "health" ]]; then
      assert_command_count 10 "curl http://127.0.0.1:8080/healthz" "${MOCK_COMMAND_LOG}" "health failure retry count"
    fi
    assert_contains "${output}" "Rollback completed." "${failure} rollback output"
    assert_path_absent "${session_dir}" "completed ${failure} rollback session cleanup"
  done
}

test_first_install_failure_removes_new_targets() {
  local case_dir session_dir staging_dir backup_dir output role target
  case_dir="${TEST_TMP}/first-install-rollback"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  make_staging "${staging_dir}"
  export MOCK_NGINX_FAIL_ON_TEST_CALL=1

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "first install nginx failure unexpectedly committed"
  fi
  while IFS=$'\t' read -r role target; do
    assert_path_absent "${target}" "first install rollback ${role}"
  done < <(transaction_targets)
  assert_command_count 0 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "first install rollback old service restart count"
  assert_not_contains "${output}" "BUPT_EC is installed." "first install rollback success output"
  assert_path_absent "${session_dir}" "completed first install rollback session cleanup"
}

test_incomplete_rollback_preserves_recovery_backup() {
  local case_dir session_dir staging_dir backup_dir output before after
  case_dir="${TEST_TMP}/incomplete-rollback"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}"
  before="$(capture_target_state)"
  export MOCK_NGINX_FAIL_ON_TEST_CALL=1
  export MOCK_SYSTEMCTL_FAIL_COMMAND=restart
  export MOCK_SYSTEMCTL_FAIL_ON_CALL=1

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "incomplete rollback scenario unexpectedly succeeded"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "incomplete rollback still restores target files"
  [[ -d "${backup_dir}" ]] || fail "incomplete rollback did not preserve its backup"
  assert_mode "${session_dir}" 700 "preserved recovery directory"
  assert_mode "${backup_dir}" 700 "preserved recovery backup"
  assert_mode "${backup_dir}/env" 600 "preserved recovery env"
  assert_contains "${output}" "Automatic rollback was incomplete" "incomplete rollback output"
  assert_contains "${output}" "${backup_dir}" "incomplete rollback recovery path"
  rm -rf "${session_dir}"
}

test_successful_upgrade_commits_and_cleans_backup() {
  local case_dir staging_dir backup_dir preview_backup
  case_dir="${TEST_TMP}/successful-upgrade"
  staging_dir="${case_dir}/staging"
  backup_dir="${case_dir}/backup"
  preview_backup="${case_dir}/preview-backup"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}"

  assert_mode "${staging_dir}" 700 "candidate directory"
  assert_mode "${staging_dir}/bupt-ec.env" 600 "candidate env"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${staging_dir}/bupt-ec.env" "candidate env ownership"

  snapshot_installation "${preview_backup}"
  assert_mode "${preview_backup}" 700 "backup directory"
  assert_mode "${preview_backup}/manifest" 600 "backup manifest"
  assert_mode "${preview_backup}/env" 600 "backup env"
  rm -rf "${preview_backup}"

  perform_install_transaction "${staging_dir}" "${backup_dir}" "127.0.0.1:8080"

  cmp -s "${staging_dir}/bupt-ec" "${INSTALL_DIR}/bupt-ec" || fail "successful upgrade binary mismatch"
  cmp -s "${staging_dir}/bupt-ec.env" "${ENV_FILE}" || fail "successful upgrade env mismatch"
  cmp -s "${staging_dir}/${SERVICE_NAME}.service" "${SERVICE_FILE}" || fail "successful upgrade service mismatch"
  cmp -s "${staging_dir}/${SERVICE_NAME}.conf" "${NGINX_SITE}" || fail "successful upgrade nginx mismatch"
  assert_enabled_target "${SYSTEMD_ENABLED_LINK}" "${SERVICE_FILE}" "successful upgrade systemd enablement"
  assert_enabled_target "${NGINX_ENABLED}" "${NGINX_SITE}" "successful upgrade nginx enablement"
  assert_mode "${INSTALL_DIR}/bupt-ec" 755 "installed binary"
  assert_mode "${ENV_FILE}" 600 "installed env"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${ENV_FILE}.new." "installed env ownership"
  assert_mode "${SERVICE_FILE}" 644 "installed service"
  assert_mode "${NGINX_SITE}" 644 "installed nginx"
  assert_path_absent "${backup_dir}" "successful upgrade backup cleanup"
  assert_eq false "${TRANSACTION_ACTIVE}" "successful transaction active flag"
  assert_eq "" "${TRANSACTION_BACKUP_DIR}" "successful transaction backup pointer"
  assert_command_count 1 "curl http://127.0.0.1:8080/healthz" "${MOCK_COMMAND_LOG}" "successful health check count"
}

test_version_policy
test_checksum_failures_preserve_targets
test_staging_failures_preserve_targets
test_snapshot_failure_preserves_targets
test_nginx_failure_rolls_back_upgrade
test_restart_and_health_failures_roll_back_upgrade
test_first_install_failure_removes_new_targets
test_incomplete_rollback_preserves_recovery_backup
test_successful_upgrade_commits_and_cleans_backup

echo "installer behavior tests passed"
