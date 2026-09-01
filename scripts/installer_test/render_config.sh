# shellcheck shell=bash
# shellcheck disable=SC2034,SC2153,SC2317,SC2329

unset_deployment_invocation_environment() {
  local key
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    unset "${key}"
  done
  unset REPO VERSION
}

test_deployment_config_key_contract() {
  local expected actual

  # Keep this literal contract independent from the production registry so a
  # simultaneous producer/consumer omission cannot make the test pass.
  expected=$'RELEASE_REPO\nRELEASE_VERSION\nDOMAIN\nSSL_CERT\nSSL_KEY\nJW_USERNAME\nJW_PASSWORD\nJW_TOKEN\nAPP_ADDR\nDOWNLOAD_BASE_URL\nLOG_CALLER\nREADYZ_DIAGNOSTICS'
  actual="$(printf '%s\n' "${DEPLOYMENT_CONFIG_KEYS[@]}")"
  assert_eq "${expected}" "${actual}" "deployment config key contract"
  assert_eq 12 "${#DEPLOYMENT_CONFIG_KEYS[@]}" "deployment config key count"
}

write_current_config_fixture() {
  local include_one_shot_flags="${1:-false}"
  local key

  set_valid_test_config
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "CFG_${key}" '%s' "current ${key}"
  done
  CFG_JW_PASSWORD=$'current password\nwith a newline\n'
  CFG_LOG_CALLER=$'current caller\nwith a newline\n'
  CFG_READYZ_DIAGNOSTICS="current diagnostics"
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"
  if [[ "${include_one_shot_flags}" == "true" ]]; then
    cat >> "${ENV_FILE}" <<'EOF'
ALLOW_INSECURE_DOWNLOAD_BASE_URL=true
SKIP_CHECKSUM=1
INSTALLER_MODE=update
CFG_DOMAIN=mutated-by-installed-env
EOF
  fi
}

test_config_registry_precedence_and_isolation() (
  local case_dir key value_name current_name expected_value override_set_name

  case_dir="${TEST_TMP}/config-precedence"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  unset_deployment_invocation_environment
  unset ALLOW_INSECURE_DOWNLOAD_BASE_URL SKIP_CHECKSUM
  write_current_config_fixture true
  reset_config_state
  capture_invocation_overrides
  load_current_config

  assert_eq install "${INSTALLER_MODE}" "installed env cannot change installer mode"
  assert_eq "" "${ALLOW_INSECURE_DOWNLOAD_BASE_URL+x}" "installed env cannot enable insecure download"
  assert_eq "" "${SKIP_CHECKSUM+x}" "installed env cannot skip checksums"
  assert_eq "" "${CFG_DOMAIN}" "installed env cannot mutate CFG state"
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    current_name="CURRENT_${key}"
    expected_value="current ${key}"
    case "${key}" in
      JW_PASSWORD)
        expected_value=$'current password\nwith a newline\n'
        ;;
      LOG_CALLER)
        expected_value=$'current caller\nwith a newline\n'
        ;;
      READYZ_DIAGNOSTICS)
        expected_value="current diagnostics"
        ;;
    esac
    assert_eq "${expected_value}" "${!current_name}" "${key} current config load"
    set_cfg_from_precedence "${key}" "default ${key}"
    value_name="CFG_${key}"
    assert_eq "${expected_value}" "${!value_name}" "${key} current config beats default"
  done

  reset_config_state
  unset_deployment_invocation_environment
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "${key}" '%s' "invocation ${key}"
  done
  capture_invocation_overrides
  load_current_config
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    set_cfg_from_precedence "${key}" "default ${key}"
    value_name="CFG_${key}"
    if [[ "${key}" == "RELEASE_VERSION" ]]; then
      assert_eq false "${OVERRIDE_RELEASE_VERSION_SET}" "RELEASE_VERSION is not an invocation override"
      assert_eq "current ${key}" "${!value_name}" "${key} keeps saved metadata"
    else
      assert_eq "invocation ${key}" "${!value_name}" "${key} invocation override wins"
    fi
  done

  # The original REPO and VERSION inputs remain compatible aliases, while the
  # canonical fields still have their own lifecycle variables.
  reset_config_state
  unset_deployment_invocation_environment
  REPO="legacy/repository"
  VERSION="v1.2.3"
  capture_invocation_overrides
  assert_eq true "${OVERRIDE_RELEASE_REPO_SET}" "legacy REPO override presence"
  assert_eq legacy/repository "${OVERRIDE_RELEASE_REPO}" "legacy REPO override value"
  assert_eq true "${OVERRIDE_VERSION_SET}" "VERSION override presence"
  assert_eq v1.2.3 "${OVERRIDE_VERSION}" "VERSION override value"

  # Every direct deployment field except saved RELEASE_VERSION treats an
  # explicitly empty invocation variable as an override rather than silently
  # falling back to the saved value.
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    if [[ "${key}" == "RELEASE_VERSION" ]]; then
      continue
    fi
    reset_config_state
    unset_deployment_invocation_environment
    printf -v "${key}" '%s' ""
    capture_invocation_overrides
    load_current_config
    set_cfg_from_precedence "${key}" "default ${key}"
    value_name="CFG_${key}"
    override_set_name="OVERRIDE_${key}_SET"
    assert_eq true "${!override_set_name}" "${key} explicit empty presence"
    assert_eq "" "${!value_name}" "${key} explicit empty override"
  done

  reset_config_state
  unset_deployment_invocation_environment
  RELEASE_VERSION=""
  capture_invocation_overrides
  load_current_config
  set_cfg_from_precedence RELEASE_VERSION "default RELEASE_VERSION"
  assert_eq false "${OVERRIDE_RELEASE_VERSION_SET}" "empty RELEASE_VERSION is not captured"
  assert_eq "current RELEASE_VERSION" "${CFG_RELEASE_VERSION}" "empty RELEASE_VERSION keeps saved metadata"

  # With neither source configured, each field reaches its supplied default.
  reset_config_state
  unset_deployment_invocation_environment
  rm -f "${ENV_FILE}"
  capture_invocation_overrides
  load_current_config
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    set_cfg_from_precedence "${key}" "default ${key}"
    value_name="CFG_${key}"
    assert_eq "default ${key}" "${!value_name}" "${key} default fallback"
  done
)

test_current_config_load_security_and_snapshot() (
  local case_dir temp_dir cert_path key_path output_file target marker chmod_log status

  case_dir="${TEST_TMP}/current-config-security"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  assert_eq "${EUID}" "${ENV_FILE_EXPECTED_UID}" "test root config owner expectation"
  temp_dir="${case_dir}/tmp"
  mkdir -p "${temp_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3

  assert_temp_dir_empty() {
    if [[ -n "$(find "${temp_dir}" -mindepth 1 -print -quit)" ]]; then
      fail "current config temporary snapshot was not removed"
    fi
  }

  chmod_log="${case_dir}/chmod.log"
  : > "${chmod_log}"
  # Production deliberately ignores caller-controlled TMPDIR. Redirect only
  # this sourced test fixture's mktemp call so cleanup can be inspected.
  mktemp() {
    if [[ "${1:-}" == "/tmp/${SERVICE_NAME}-config.XXXXXX" ]]; then
      command mktemp "${temp_dir}/${SERVICE_NAME}-config.XXXXXX"
    else
      command mktemp "$@"
    fi
  }
  chmod() {
    if [[ "${1:-}" == "0600" && "${2:-}" == "${temp_dir}/"* ]]; then
      builtin printf '%s\n' "$*" >> "${chmod_log}"
    fi
    command chmod "$@"
  }
  load_current_config
  unset -f chmod
  assert_contains "${chmod_log}" "0600 ${temp_dir}/" "current config snapshot mode"
  assert_temp_dir_empty

  # The checked file path is trustworthy only inside a directory that another
  # user cannot modify between validation and source.
  if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
    chmod 0777 "${CONFIG_DIR}"
    output_file="${case_dir}/directory-mode.log"
    set +e
    load_current_config > "${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "configuration in a writable directory was accepted"
    fi
    assert_not_contains "${output_file}" "test-password" "directory failure redacts secrets"
    assert_temp_dir_empty

    # Even with no current env, first install must refuse an existing unsafe
    # directory before the transaction can write credentials into it.
    rm -f -- "${ENV_FILE}"
    set_valid_test_config
    INSTALLER_MODE=install
    output_file="${case_dir}/empty-unsafe-directory.log"
    set +e
    load_current_config > "${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "loader accepted an unsafe existing directory without an env"
    fi
    assert_contains "${output_file}" "Failed to load existing configuration safely." \
      "empty unsafe directory fails before interactive collection"
    assert_temp_dir_empty
    chmod 0755 "${CONFIG_DIR}"
  fi

  # A symlink is rejected before source execution, even if its target otherwise
  # has the expected owner and mode.
  target="${case_dir}/env-target"
  marker="${case_dir}/symlink-sourced"
  set_valid_test_config
  CFG_JW_TOKEN="symlink-secret"
  render_env_file "${target}"
  printf 'touch %q\n' "${marker}" >> "${target}"
  chmod 0600 "${target}"
  rm -f -- "${ENV_FILE}"
  ln -s "${target}" "${ENV_FILE}"
  if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
    output_file="${case_dir}/symlink.log"
    set +e
    load_current_config > "${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "symlinked current configuration was accepted"
    fi
    [[ ! -e "${marker}" ]] || fail "symlinked current configuration was sourced"
    assert_not_contains "${output_file}" "symlink-secret" "symlink failure redacts secret"
    assert_temp_dir_empty
  fi

  # Mode and owner checks happen before source execution. The explicit expected
  # UID seam also lets this test run without a root-owned fixture.
  rm -f -- "${ENV_FILE}"
  set_valid_test_config
  CFG_JW_TOKEN="mode-secret"
  render_env_file "${ENV_FILE}"
  chmod 0644 "${ENV_FILE}"
  if [[ "${POSIX_MODES_SUPPORTED}" == "false" ]]; then
    CURRENT_CONFIG_TEST_STAT_MODE=644
  fi
  output_file="${case_dir}/mode.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "world-readable current configuration was accepted"
  fi
  assert_not_contains "${output_file}" "mode-secret" "mode failure redacts secret"
  assert_temp_dir_empty
  unset CURRENT_CONFIG_TEST_STAT_MODE

  chmod 0600 "${ENV_FILE}"
  ENV_FILE_EXPECTED_UID=$((EUID + 1))
  output_file="${case_dir}/owner.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "wrong current configuration owner was accepted"
  fi
  assert_not_contains "${output_file}" "mode-secret" "owner failure redacts secret"
  assert_temp_dir_empty
  ENV_FILE_EXPECTED_UID="${EUID}"

  # Source errors are synchronous, source stderr stays private, and the
  # temporary snapshot is removed before the generic failure is returned.
  cat > "${ENV_FILE}" <<'EOF'
printf '%s\n' 'source-failure-secret' >&2
false
EOF
  chmod 0600 "${ENV_FILE}"
  output_file="${case_dir}/source-failure.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "failing current configuration source was accepted"
  fi
  assert_not_contains "${output_file}" "source-failure-secret" "source failure redacts stderr"
  assert_temp_dir_empty

  # Source stdout is part of the strict framing and cannot become accepted
  # configuration data or leak to the caller.
  cat > "${ENV_FILE}" <<'EOF'
printf '%s' 'malformed-output-secret'
EOF
  chmod 0600 "${ENV_FILE}"
  output_file="${case_dir}/malformed.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "malformed current configuration snapshot was accepted"
  fi
  assert_not_contains "${output_file}" "malformed-output-secret" "malformed snapshot redacts secret"
  assert_temp_dir_empty

  # An EXIT trap installed by the sourced file writes only after the loader
  # serializes all 12 frames, exercising the exact EOF/trailing-data check.
  cat > "${ENV_FILE}" <<'EOF'
trap 'printf "%s" "trailing-output-secret"' EXIT
EOF
  chmod 0600 "${ENV_FILE}"
  output_file="${case_dir}/trailing.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "trailing current configuration snapshot data was accepted"
  fi
  assert_not_contains "${output_file}" "trailing-output-secret" "trailing snapshot redacts secret"
  assert_temp_dir_empty

  # EXIT-trap stderr executes after the sourced command's own redirection has
  # ended; the loader must suppress the entire child shell, not only `source`.
  cat > "${ENV_FILE}" <<'EOF'
trap 'printf "%s\n" "exit-trap-stderr-secret" >&2' EXIT
false || true
EOF
  chmod 0600 "${ENV_FILE}"
  output_file="${case_dir}/exit-trap-stderr.log"
  set +e
  load_current_config > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status != 0 )); then
    fail "valid framed config with stderr-only EXIT trap was rejected"
  fi
  assert_not_contains "${output_file}" "exit-trap-stderr-secret" "EXIT trap stderr stays private"
  assert_temp_dir_empty
  unset -f mktemp
)

test_render_failure_propagation() (
  local case_dir output_file first_failure render_failure

  case_dir="${TEST_TMP}/render-failure-propagation"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  set_valid_test_config

  first_failure="${case_dir}/shell-quote-first-printf"
  output_file="${case_dir}/shell-quote.out"
  printf() {
    if [[ "${1:-}" == "'" && ! -e "${first_failure}" ]]; then
      : > "${first_failure}"
      return 1
    fi
    # shellcheck disable=SC2059
    builtin printf "$@"
  }
  if shell_quote "quoted value" > "${output_file}"; then
    unset -f printf
    fail "shell_quote ignored an intermediate printf failure"
  fi
  unset -f printf

  sed() { return 1; }
  if shell_quote "quoted value" > "${output_file}"; then
    unset -f sed
    fail "shell_quote ignored a sed failure in conditional context"
  fi
  unset -f sed

  first_failure="${case_dir}/shell-quote-pipeline-printf"
  printf() {
    if [[ "${1:-}" == "%s" && ! -e "${first_failure}" ]]; then
      : > "${first_failure}"
      return 1
    fi
    # shellcheck disable=SC2059
    builtin printf "$@"
  }
  if shell_quote "quoted value" > "${output_file}"; then
    unset -f printf
    fail "shell_quote ignored a pipeline printf failure in conditional context"
  fi
  unset -f printf

  render_failure="${case_dir}/render-first-printf"
  printf() {
    if [[ "${1:-}" == "%s=" && ! -e "${render_failure}" ]]; then
      : > "${render_failure}"
      return 1
    fi
    # shellcheck disable=SC2059
    builtin printf "$@"
  }
  if render_env_file "${case_dir}/failed.env"; then
    unset -f printf
    fail "render_env_file ignored a failed write in conditional context"
  fi
  unset -f printf
  # The destination redirection must happen after umask 077, so even a failed
  # render never leaves a transient world-readable secret file.
  if [[ -e "${case_dir}/failed.env" ]]; then
    assert_mode "${case_dir}/failed.env" 600 "failed rendered env"
  fi
)

test_deployment_metadata_render_contract() {
  local case_dir metadata_file expected_file target_roles expected_roles

  case_dir="${TEST_TMP}/deployment-metadata"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  CFG_RELEASE_VERSION="v0.3.0"
  CFG_APP_ADDR="[::1]:8080"
  metadata_file="${case_dir}/deployment.meta"
  expected_file="${case_dir}/expected.meta"
  render_deployment_metadata "${metadata_file}"

  printf 'RELEASE_VERSION=v0.3.0\nAPP_ADDR=[::1]:8080\n' > "${expected_file}"
  cmp -s "${expected_file}" "${metadata_file}" || fail "metadata exact records"
  assert_mode "${metadata_file}" 644 "rendered metadata"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${metadata_file}" "metadata ownership"
  assert_not_contains "${metadata_file}" "RELEASE_REPO=" "metadata excludes repository"
  assert_not_contains "${metadata_file}" "JW_" "metadata excludes JW fields"
  assert_not_contains "${metadata_file}" "DOWNLOAD_BASE_URL" "metadata excludes mirror"
  assert_not_contains "${metadata_file}" "LOG_CALLER" "metadata excludes logging flag"
  assert_not_contains "${metadata_file}" "READYZ_DIAGNOSTICS" "metadata excludes readiness flag"
  expected_roles=$'binary\nenv\ncli\nmetadata\nservice\nsystemd_enabled\nnginx_site\nnginx_enabled'
  target_roles="$(transaction_targets | cut -f1)"
  assert_eq "${expected_roles}" "${target_roles}" "eight transaction target roles"
}

test_cli_archive_staging_matrix() {
  local case_dir work_dir staging_dir before after output status

  case_dir="${TEST_TMP}/missing-cli-archive"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  output="${case_dir}/output.log"
  mkdir -p "${case_dir}" "${work_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  CFG_RELEASE_VERSION="v0.3.0"
  seed_existing_installation
  before="$(capture_target_state)"
  set +e
  prepare_staging "${MISSING_CLI_ARCHIVE}" "${work_dir}" "${staging_dir}" > "${output}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "CLI-bearing archive without CLI unexpectedly staged"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "missing CLI archive preserves installed targets"
  assert_contains "${output}" "does not contain bupt-ec-cli" "missing CLI archive error"

  case_dir="${TEST_TMP}/legacy-cli-archive"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  mkdir -p "${case_dir}" "${work_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  CFG_RELEASE_VERSION="v0.2.0"
  prepare_staging "${LEGACY_ARCHIVE}" "${work_dir}" "${staging_dir}"
  assert_eq remove "$(< "${staging_dir}/cli.action")" "legacy staging action"
  assert_path_absent "${staging_dir}/bupt-ec-cli" "legacy staging CLI candidate"
  assert_path_absent "${staging_dir}/deployment.meta" "legacy staging metadata candidate"
}

test_config_render_round_trip() (
  local case_dir env_file expected_file actual_file key value_name

  case_dir="${TEST_TMP}/config-render-round-trip"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "CFG_${key}" '%s' "value for ${key}: spaces 'quotes' \$dollar"
  done
  CFG_JW_PASSWORD=$'password with spaces\nsecond line\n'
  CFG_LOG_CALLER=$'caller=true\nsecond line\n'
  CFG_READYZ_DIAGNOSTICS=$'diagnostics with a newline\n'

  env_file="${case_dir}/round-trip.env"
  expected_file="${case_dir}/expected.nul"
  actual_file="${case_dir}/actual.nul"
  render_env_file "${env_file}"

  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    assert_contains "${env_file}" "${key}=" "rendered ${key} field"
  done
  assert_not_contains "${env_file}" "ALLOW_INSECURE_DOWNLOAD_BASE_URL=" "env excludes insecure download flag"
  assert_not_contains "${env_file}" "SKIP_CHECKSUM=" "env excludes checksum bypass"
  assert_mode "${env_file}" 600 "rendered env"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${env_file}" "rendered env ownership"

  (
    for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
      value_name="CFG_${key}"
      printf '%s\0' "${!value_name}"
    done
  ) > "${expected_file}"
  (
    # shellcheck disable=SC1090
    . "${env_file}"
    for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
      printf '%s\0' "${!key}"
    done
  ) > "${actual_file}"
  cmp -s "${expected_file}" "${actual_file}" || fail "shell-quoted env did not round-trip all registry values"
)

test_staging_failures_preserve_targets() {
  local case_dir before after status output work_dir staging_dir

  case_dir="${TEST_TMP}/archive-missing-binary"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  seed_existing_installation
  before="$(capture_target_state)"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  output="${case_dir}/output.log"
  mkdir -p "${work_dir}"
  set +e
  prepare_staging "${MISSING_BINARY_ARCHIVE}" "${work_dir}" "${staging_dir}" > "${output}" 2>&1
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
  set_valid_test_config
  seed_existing_installation
  before="$(capture_target_state)"
  work_dir="${case_dir}/work"
  staging_dir="${case_dir}/staging"
  output="${case_dir}/output.log"
  mkdir -p "${work_dir}"
  export MOCK_CHOWN_FAIL_PATTERN="${staging_dir}/bupt-ec.env"
  set +e
  prepare_staging "${VALID_ARCHIVE}" "${work_dir}" "${staging_dir}" > "${output}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "render failure unexpectedly staged all candidates"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "render failure preserves installed targets"
}

test_api_proxy_read_timeout_budget() {
  local conf_file api_timeout spa_timeout
  conf_file="${TEST_TMP}/nginx-timeout.conf"
  reset_config_state
  set_valid_test_config
  render_nginx_site "${conf_file}"
  api_timeout="$(awk '
    /location \/api\// { in_api=1; next }
    in_api && /location \// { in_api=0 }
    in_api && /proxy_read_timeout/ { print; exit }
  ' "${conf_file}")"
  spa_timeout="$(awk '
    /location \/ \{/ { in_spa=1; next }
    in_spa && /^[[:space:]]*\}/ { in_spa=0 }
    in_spa && /proxy_read_timeout/ { print; exit }
  ' "${conf_file}")"
  if [[ "${api_timeout}" != *"proxy_read_timeout 60s;"* ]]; then
    fail "api location uses 60s read timeout: got '${api_timeout}'"
  fi
  if [[ "${spa_timeout}" != *"proxy_read_timeout 30s;"* ]]; then
    fail "spa location keeps 30s read timeout: got '${spa_timeout}'"
  fi
  assert_contains "${conf_file}" "location = /metrics" "nginx denies public metrics path"
  assert_contains "${conf_file}" "return 404;" "metrics path returns 404"
}
