# shellcheck shell=bash
# shellcheck disable=SC2030,SC2031,SC2034,SC2153

# shellcheck disable=SC2329
test_entrypoint_stdin_pipe_reaches_root_check() {
  # curl | bash feeds the script on stdin; with set -u the old BASH_SOURCE[0]
  # guard aborted before main. Expect a clean root/EUID failure instead.
  local output status stdin_installer
  stdin_installer="${TEST_TMP}/stdin-install.sh"
  # Keep the pipe fixture byte-identical to the generated asset. On MSYS,
  # redirecting a file below /c/ can perturb its symlink emulation globally.
  cat -- "${INSTALLER_TEST_SCRIPT_DIR}/install.sh" > "${stdin_installer}"
  cmp -s "${INSTALLER_TEST_SCRIPT_DIR}/install.sh" "${stdin_installer}" || \
    fail "stdin installer fixture differs from generated install.sh"
  set +e
  # Feed install.sh on stdin (same as curl | bash); avoid pipe/cat for shellcheck.
  output="$(env -i PATH="${PATH}" HOME="${HOME:-/tmp}" bash <"${stdin_installer}" 2>&1)"
  status=$?
  set -e
  if [[ "${output}" == *"BASH_SOURCE"* ]]; then
    fail "stdin pipe still trips BASH_SOURCE: ${output}"
  fi
  if [[ "${output}" != *"must run as root"* ]]; then
    fail "stdin pipe should fail on root check, got status=${status}: ${output}"
  fi
  if [[ "${status}" -eq 0 ]]; then
    fail "stdin pipe root check should exit non-zero"
  fi
}

test_parse_mode_matrix() (
  local output_file

  reset_config_state
  parse_mode
  assert_eq install "${INSTALLER_MODE}" "default mode is install"
  parse_mode --mode=update
  assert_eq update "${INSTALLER_MODE}" "equals mode syntax"
  parse_mode --mode reconfigure
  assert_eq reconfigure "${INSTALLER_MODE}" "separate mode syntax"

  output_file="${TEST_TMP}/parse-mode-invalid.log"
  for_invalid_mode() {
    if parse_mode "$@" > "${output_file}" 2>&1; then
      fail "invalid mode arguments unexpectedly succeeded"
    fi
    assert_contains "${output_file}" "install|update|reconfigure" "invalid mode usage"
  }
  for_invalid_mode --mode=
  for_invalid_mode --mode
  for_invalid_mode --mode=unknown
  for_invalid_mode --mode install --mode=update
  for_invalid_mode --mode=install extra
  for_invalid_mode extra
)

test_interactive_modes_require_tty() (
  local case_dir mode output_file status

  case_dir="${TEST_TMP}/interactive-tty"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  TTY="${case_dir}/missing-tty"
  require_root_environment() { return 0; }

  for mode in install reconfigure; do
    reset_config_state
    output_file="${case_dir}/${mode}.log"
    set +e
    main "--mode=${mode}" > "${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "${mode} unexpectedly bypassed the TTY gate"
    fi
    assert_contains "${output_file}" "Interactive input requires a TTY." "${mode} TTY gate"
  done
)

test_prompt_return_values_do_not_capture_feedback() (
  local case_dir secret_input optional_input required_state result stderr_file

  case_dir="${TEST_TMP}/prompt-return-values"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"

  secret_input="${case_dir}/secret.input"
  stderr_file="${case_dir}/secret.stderr"
  printf 'secret-value\n' > "${secret_input}"
  TTY="${secret_input}"
  result="$(prompt_secret "Secret value" false 2> "${stderr_file}")"
  assert_eq secret-value "${result}" "prompt_secret return value"
  [[ -s "${stderr_file}" ]] || fail "prompt_secret feedback was not sent to stderr"

  optional_input="${case_dir}/optional.input"
  stderr_file="${case_dir}/optional.stderr"
  printf 'optional-secret\n' > "${optional_input}"
  TTY="${optional_input}"
  result="$(prompt_optional_secret "Optional secret" false 2> "${stderr_file}")"
  assert_eq optional-secret "${result}" "prompt_optional_secret return value"
  [[ -s "${stderr_file}" ]] || fail "prompt_optional_secret feedback was not sent to stderr"

  required_state="${case_dir}/required-state"
  stderr_file="${case_dir}/required.stderr"
  prompt() {
    if [[ ! -e "${required_state}" ]]; then
      : > "${required_state}"
      printf ''
    else
      printf 'required-value'
    fi
  }
  result="$(prompt_required "Required value" "" 2> "${stderr_file}")"
  assert_eq required-value "${result}" "retrying prompt_required return value"
  assert_contains "${stderr_file}" "This value is required." "prompt_required retry feedback"

  unset -f prompt
  TTY="${case_dir}/closed.input"
  : > "${TTY}"
  if prompt_required "Closed input" "" >/dev/null 2>&1; then
    fail "prompt_required accepted EOF instead of propagating the read failure"
  fi
)

test_no_arg_main_default_completion_output() (
  local case_dir cert_path key_path output_file expected_file deployment_log

  case_dir="${TEST_TMP}/no-arg-main-completion"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  unset_deployment_invocation_environment
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  output_file="${case_dir}/output.log"
  expected_file="${case_dir}/expected.log"
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"

  require_root_environment() { return 0; }
  require_interactive_tty() { return 0; }
  prompt_required() {
    case "$1" in
      "GitHub repository" | "Backend listen address") printf '%s' "$2" ;;
      "Domain name") printf '%s' "classroom.example.com" ;;
      "SSL certificate path") printf '%s' "${cert_path}" ;;
      "SSL private key path") printf '%s' "${key_path}" ;;
      *) fail "unexpected required prompt in no-arg main: $1" ;;
    esac
  }
  prompt() {
    if [[ "$1" == "BUPT JW username, optional when JW token is set" ]]; then
      printf '%s' "default-user"
    else
      fail "unexpected prompt in no-arg main: $1"
    fi
  }
  prompt_optional_secret() {
    case "$1" in
      "JW token override, usually leave empty") printf '%s' "default-token" ;;
      "BUPT JW password") printf '%s' "default-password" ;;
      *) fail "unexpected optional secret prompt in no-arg main: $1" ;;
    esac
  }
  prompt_secret() { fail "unexpected required secret prompt in no-arg main"; }
  execute_deployment() { printf 'execute\n' >> "${deployment_log}"; }

  main > "${output_file}" 2>&1
  cat > "${expected_file}" <<'EOF'
BUPT_EC installer


BUPT_EC is installed.
URL: https://classroom.example.com/
Service: systemctl status bupt-ec
Upgrade later: rerun this installer.
EOF
  cmp -s "${expected_file}" "${output_file}" || fail "no-arg main completion output changed"
  assert_eq install "${INSTALLER_MODE}" "no-arg main mode"
  assert_command_count 1 execute "${deployment_log}" "no-arg main deployment count"
)

test_no_mode_install_prompt_order_baseline() (
  local case_dir prompt_log expected_log output_file

  case_dir="${TEST_TMP}/no-mode-install-prompts"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  unset_deployment_invocation_environment
  capture_invocation_overrides
  load_current_config
  parse_mode
  assert_eq install "${INSTALLER_MODE}" "no-mode install selection"

  prompt_log="${case_dir}/prompts.log"
  output_file="${case_dir}/output.log"
  expected_log="${case_dir}/expected-prompts.log"
  prompt_required() {
    printf 'required|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    case "$1" in
      "GitHub repository") printf '%s' "${2}" ;;
      "Domain name") printf '%s' "classroom.example.com" ;;
      "SSL certificate path" | "SSL private key path" | "Backend listen address") printf '%s' "${2}" ;;
      "BUPT JW username") printf '%s' "baseline-user" ;;
      *) fail "unexpected required prompt: $1" ;;
    esac
  }
  prompt() {
    printf 'prompt|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf '%s' "baseline-user"
  }
  prompt_optional_secret() {
    printf 'optional-secret|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    if [[ "$1" == "JW token override, usually leave empty" ]]; then
      printf '%s' "baseline-token"
    else
      printf '%s' "baseline-password"
    fi
  }
  prompt_secret() {
    printf 'secret|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf '%s' "baseline-password"
  }

  collect_config_interactive "${INSTALLER_MODE}" > "${output_file}"
  cat > "${expected_log}" <<'EOF'
required|GitHub repository|ming-kang/BUPT_EC
required|Domain name|
required|SSL certificate path|/etc/letsencrypt/live/classroom.example.com/fullchain.pem
required|SSL private key path|/etc/letsencrypt/live/classroom.example.com/privkey.pem
optional-secret|JW token override, usually leave empty|false
prompt|BUPT JW username, optional when JW token is set|
optional-secret|BUPT JW password|false
required|Backend listen address|127.0.0.1:8080
EOF
  cmp -s "${expected_log}" "${prompt_log}" || fail "no-mode install prompt order/defaults changed"
  assert_contains "${output_file}" "BUPT_EC installer" "no-mode install banner"
  assert_eq latest "${CFG_RELEASE_VERSION}" "no-mode install version default"
  assert_eq baseline-token "${CFG_JW_TOKEN}" "no-mode install token prompt result"
  assert_eq baseline-password "${CFG_JW_PASSWORD}" "no-mode install password prompt result"
)

test_no_mode_install_existing_prompt_defaults() (
  local case_dir prompt_log expected_log

  case_dir="${TEST_TMP}/no-mode-install-existing-prompts"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  set_valid_test_config
  CFG_RELEASE_REPO="saved/repository"
  CFG_RELEASE_VERSION="v1.2.3"
  CFG_DOMAIN="saved.example.com"
  CFG_SSL_CERT="/saved/fullchain.pem"
  CFG_SSL_KEY="/saved/privkey.pem"
  CFG_JW_USERNAME="saved-user"
  CFG_JW_PASSWORD="saved-password"
  CFG_JW_TOKEN="saved-token"
  CFG_APP_ADDR="127.0.0.1:9090"
  CFG_DOWNLOAD_BASE_URL="https://mirror.example/releases"
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"

  reset_config_state
  unset_deployment_invocation_environment
  capture_invocation_overrides
  load_current_config
  parse_mode

  prompt_log="${case_dir}/prompts.log"
  expected_log="${case_dir}/expected-prompts.log"
  prompt_required() {
    printf 'required|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf '%s' "$2"
  }
  prompt() {
    printf 'prompt|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf '%s' "$2"
  }
  prompt_optional_secret() {
    printf 'optional-secret|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf ''
  }
  prompt_secret() {
    printf 'secret|%s|%s\n' "$1" "$2" >> "${prompt_log}"
    printf ''
  }

  collect_config_interactive "${INSTALLER_MODE}" > /dev/null
  cat > "${expected_log}" <<'EOF'
required|GitHub repository|saved/repository
required|Domain name|saved.example.com
required|SSL certificate path|/saved/fullchain.pem
required|SSL private key path|/saved/privkey.pem
optional-secret|JW token override, usually leave empty|true
prompt|BUPT JW username, optional when JW token is set|saved-user
optional-secret|BUPT JW password|true
required|Backend listen address|127.0.0.1:9090
EOF
  cmp -s "${expected_log}" "${prompt_log}" || fail "existing no-mode prompt defaults changed"
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "existing no-mode version default"
  assert_eq saved-token "${CFG_JW_TOKEN}" "existing no-mode token retention"
  assert_eq saved-password "${CFG_JW_PASSWORD}" "existing no-mode password retention"
)

test_mode_version_semantics() (
  local case_dir cert_path key_path deployment_log expected_log output_file key cfg_name current_name

  case_dir="${TEST_TMP}/mode-version-semantics"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3

  unset_deployment_invocation_environment
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "${key}" '%s' "ignored-by-update ${key}"
  done
  VERSION=v2.3.4
  capture_invocation_overrides
  load_current_config
  adopt_current_config
  assert_eq v2.3.4 "${CFG_RELEASE_VERSION}" "update VERSION override wins"
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    if [[ "${key}" == "RELEASE_VERSION" ]]; then
      continue
    fi
    cfg_name="CFG_${key}"
    current_name="CURRENT_${key}"
    assert_eq "${!current_name}" "${!cfg_name}" "update ignores ${key} override"
  done

  reset_config_state
  unset_deployment_invocation_environment
  RELEASE_VERSION=v9.9.9
  capture_invocation_overrides
  load_current_config
  assert_eq false "${OVERRIDE_RELEASE_VERSION_SET}" "direct RELEASE_VERSION is not captured"
  adopt_current_config
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "update ignores RELEASE_VERSION override"

  # install has one version input: explicit VERSION, then saved metadata, then
  # latest. Direct RELEASE_VERSION is deliberately never an invocation alias.
  reset_config_state
  unset_deployment_invocation_environment
  RELEASE_VERSION=v9.9.9
  capture_invocation_overrides
  load_current_config
  set_install_release_version
  assert_eq false "${OVERRIDE_RELEASE_VERSION_SET}" "install ignores direct RELEASE_VERSION"
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "install keeps saved release version"

  reset_config_state
  unset_deployment_invocation_environment
  RELEASE_VERSION=""
  capture_invocation_overrides
  load_current_config
  set_install_release_version
  assert_eq false "${OVERRIDE_RELEASE_VERSION_SET}" "empty direct RELEASE_VERSION is ignored"
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "empty direct RELEASE_VERSION cannot clear saved version"

  reset_config_state
  unset_deployment_invocation_environment
  RELEASE_VERSION=v9.9.9
  capture_invocation_overrides
  clear_current_config
  set_install_release_version
  assert_eq latest "${CFG_RELEASE_VERSION}" "install defaults to latest without saved version"

  reset_config_state
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  capture_invocation_overrides
  load_current_config
  set_install_release_version
  assert_eq v2.3.4 "${CFG_RELEASE_VERSION}" "install VERSION override wins"

  # A legacy/default install changing version must not silently reuse a saved
  # version-specific mirror. Supplying DOWNLOAD_BASE_URL explicitly is the
  # operator's matching-source confirmation.
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    set_cfg_from_precedence "${key}" ""
  done
  set_install_release_version
  INSTALLER_MODE=install
  output_file="${case_dir}/install-mirror-version-mismatch.log"
  if validate_config > "${output_file}" 2>&1; then
    fail "install changed VERSION while silently reusing a saved mirror"
  fi
  assert_contains "${output_file}" "explicit matching DOWNLOAD_BASE_URL" \
    "install mirror/version mismatch guidance"

  reset_config_state
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  DOWNLOAD_BASE_URL=""
  capture_invocation_overrides
  load_current_config
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    set_cfg_from_precedence "${key}" ""
  done
  set_install_release_version
  INSTALLER_MODE=install
  validate_config
  assert_eq "" "${CFG_DOWNLOAD_BASE_URL}" \
    "explicit empty DOWNLOAD_BASE_URL selects official source for version change"

  reset_config_state
  unset_deployment_invocation_environment
  VERSION=""
  capture_invocation_overrides
  load_current_config
  set_install_release_version
  assert_eq true "${OVERRIDE_VERSION_SET}" "empty VERSION remains explicit"
  assert_eq "" "${CFG_RELEASE_VERSION}" "empty VERSION does not fall back to saved version"

  reset_config_state
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  capture_invocation_overrides
  load_current_config
  prompt_required() { printf '%s' "$2"; }
  prompt() { printf '%s' "$2"; }
  prompt_optional_secret() { printf ''; }
  prompt_secret() { printf ''; }
  collect_config_interactive reconfigure > /dev/null
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "reconfigure ignores VERSION"

  deployment_log="${case_dir}/reconfigure-deployment.log"
  expected_log="${case_dir}/expected-reconfigure-deployment.log"
  : > "${deployment_log}"
  INSTALLER_MODE=reconfigure
  install_packages() { printf 'install_packages\n' >> "${deployment_log}"; }
  create_user() { printf 'create_user\n' >> "${deployment_log}"; }
  download_release() { printf 'download|%s\n' "$2" >> "${deployment_log}"; }
  prepare_staging() { printf 'prepare_staging\n' >> "${deployment_log}"; }
  perform_install_transaction() { printf 'transaction\n' >> "${deployment_log}"; }
  execute_deployment
  cat > "${expected_log}" <<'EOF'
install_packages
create_user
download|v1.2.3
prepare_staging
transaction
EOF
  cmp -s "${expected_log}" "${deployment_log}" || fail "reconfigure deployment call order changed"
)

test_reconfigure_main_uses_saved_version() (
  local case_dir cert_path key_path output_file deployment_log

  case_dir="${TEST_TMP}/reconfigure-main"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
  unset_deployment_invocation_environment
  VERSION=v9.9.9
  output_file="${case_dir}/output.log"
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"

  require_root_environment() { return 0; }
  require_interactive_tty() { return 0; }
  prompt_required() { printf '%s' "$2"; }
  prompt() { printf '%s' "$2"; }
  prompt_optional_secret() { printf ''; }
  prompt_secret() { printf ''; }
  execute_deployment() {
    printf '%s|%s\n' "${INSTALLER_MODE}" "${CFG_RELEASE_VERSION}" >> "${deployment_log}"
  }

  main --mode=reconfigure > "${output_file}" 2>&1
  assert_eq reconfigure "${INSTALLER_MODE}" "reconfigure main mode"
  assert_eq v1.2.3 "${CFG_RELEASE_VERSION}" "reconfigure main saved version"
  assert_eq 'reconfigure|v1.2.3' "$(< "${deployment_log}")" \
    "reconfigure main deployment selection"
  assert_contains "${output_file}" "BUPT_EC reconfiguration completed." \
    "reconfigure main completion"
  assert_contains "${output_file}" "Version: v1.2.3" \
    "reconfigure main completion version"
)

test_runtime_flags_across_modes() (
  local case_dir cert_path key_path env_file prompt_log

  case_dir="${TEST_TMP}/runtime-flags-modes"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
  set_valid_test_config
  CFG_SSL_CERT="${cert_path}"
  CFG_SSL_KEY="${key_path}"
  CFG_RELEASE_VERSION="v1.2.3"
  CFG_DOWNLOAD_BASE_URL="https://mirror.example/releases"
  CFG_LOG_CALLER="current-log-caller"
  CFG_READYZ_DIAGNOSTICS="current-readyz"
  render_env_file "${ENV_FILE}"

  prompt_log="${case_dir}/prompts.log"
  prompt_required() {
    printf '%s\n' "$1" >> "${prompt_log}"
    printf '%s' "$2"
  }
  prompt() {
    printf '%s\n' "$1" >> "${prompt_log}"
    printf '%s' "$2"
  }
  prompt_optional_secret() {
    printf '%s\n' "$1" >> "${prompt_log}"
    printf ''
  }
  prompt_secret() {
    printf '%s\n' "$1" >> "${prompt_log}"
    printf ''
  }

  unset_deployment_invocation_environment
  LOG_CALLER="install-log-caller"
  READYZ_DIAGNOSTICS="install-readyz"
  capture_invocation_overrides
  load_current_config
  : > "${prompt_log}"
  collect_config_interactive install > /dev/null
  assert_eq install-log-caller "${CFG_LOG_CALLER}" "install LOG_CALLER override"
  assert_eq install-readyz "${CFG_READYZ_DIAGNOSTICS}" "install READYZ override"
  assert_not_contains "${prompt_log}" "LOG_CALLER" "install has no LOG_CALLER prompt"
  assert_not_contains "${prompt_log}" "READYZ_DIAGNOSTICS" "install has no READYZ prompt"
  env_file="${case_dir}/install.env"
  render_env_file "${env_file}"
  assert_contains "${env_file}" "LOG_CALLER='install-log-caller'" "install persists LOG_CALLER"
  assert_contains "${env_file}" "READYZ_DIAGNOSTICS='install-readyz'" "install persists READYZ"

  reset_config_state
  unset_deployment_invocation_environment
  LOG_CALLER="reconfigure-log-caller"
  READYZ_DIAGNOSTICS="reconfigure-readyz"
  capture_invocation_overrides
  load_current_config
  : > "${prompt_log}"
  collect_config_interactive reconfigure > /dev/null
  assert_eq reconfigure-log-caller "${CFG_LOG_CALLER}" "reconfigure LOG_CALLER override"
  assert_eq reconfigure-readyz "${CFG_READYZ_DIAGNOSTICS}" "reconfigure READYZ override"
  assert_not_contains "${prompt_log}" "LOG_CALLER" "reconfigure has no LOG_CALLER prompt"
  assert_not_contains "${prompt_log}" "READYZ_DIAGNOSTICS" "reconfigure has no READYZ prompt"
  env_file="${case_dir}/reconfigure.env"
  render_env_file "${env_file}"
  assert_contains "${env_file}" "LOG_CALLER='reconfigure-log-caller'" "reconfigure persists LOG_CALLER"
  assert_contains "${env_file}" "READYZ_DIAGNOSTICS='reconfigure-readyz'" "reconfigure persists READYZ"

  reset_config_state
  unset_deployment_invocation_environment
  LOG_CALLER="ignored-update-log-caller"
  READYZ_DIAGNOSTICS="ignored-update-readyz"
  capture_invocation_overrides
  load_current_config
  adopt_current_config
  assert_eq current-log-caller "${CFG_LOG_CALLER}" "update keeps saved LOG_CALLER"
  assert_eq current-readyz "${CFG_READYZ_DIAGNOSTICS}" "update keeps saved READYZ"
  env_file="${case_dir}/update.env"
  render_env_file "${env_file}"
  assert_contains "${env_file}" "LOG_CALLER='current-log-caller'" "update persists saved LOG_CALLER"
  assert_contains "${env_file}" "READYZ_DIAGNOSTICS='current-readyz'" "update persists saved READYZ"
)

test_update_noninteractive_skips_packages() (
  local case_dir cert_path key_path output_file prompt_log tty_log status

  case_dir="${TEST_TMP}/update-noninteractive"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
  unset_deployment_invocation_environment

  output_file="${case_dir}/output.log"
  prompt_log="${case_dir}/prompts.log"
  tty_log="${case_dir}/tty.log"
  : > "${prompt_log}"
  : > "${tty_log}"
  require_root_environment() { return 0; }
  require_interactive_tty() {
    printf 'tty\n' >> "${tty_log}"
    return 1
  }
  install_packages() { printf 'install_packages\n' >> "${MOCK_COMMAND_LOG}"; }
  create_user() { printf 'create_user\n' >> "${MOCK_COMMAND_LOG}"; }
  prompt() { printf 'prompt\n' >> "${prompt_log}"; return 1; }
  prompt_required() { printf 'prompt_required\n' >> "${prompt_log}"; return 1; }
  prompt_secret() { printf 'prompt_secret\n' >> "${prompt_log}"; return 1; }
  prompt_optional_secret() { printf 'prompt_optional_secret\n' >> "${prompt_log}"; return 1; }

  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status != 0 )); then
    cat "${output_file}" >&2
    fail "noninteractive update failed"
  fi
  [[ ! -s "${prompt_log}" ]] || fail "update unexpectedly prompted"
  [[ ! -s "${tty_log}" ]] || fail "update unexpectedly checked the TTY"
  assert_command_count 0 install_packages "${MOCK_COMMAND_LOG}" "update install_packages count"
  assert_command_count 0 "apt-get " "${MOCK_COMMAND_LOG}" "update apt command count"
  assert_command_count 1 create_user "${MOCK_COMMAND_LOG}" "update create_user count"
  assert_contains "${ENV_FILE}" "RELEASE_VERSION='v1.2.3'" "update retains saved version"
  assert_contains "${output_file}" "BUPT_EC update completed." "update completion summary"
)

test_update_explicit_version_reaches_download() (
  local case_dir cert_path key_path output_file deployment_log

  case_dir="${TEST_TMP}/update-explicit-version"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  set_valid_test_config
  CFG_RELEASE_VERSION=v1.2.3
  CFG_SSL_CERT="${cert_path}"
  CFG_SSL_KEY="${key_path}"
  CFG_DOWNLOAD_BASE_URL=""
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  output_file="${case_dir}/output.log"
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"

  require_root_environment() { return 0; }
  create_user() { return 0; }
  download_release() { printf 'download|%s\n' "$2" >> "${deployment_log}"; }
  prepare_staging() { printf 'staging\n' >> "${deployment_log}"; }
  perform_install_transaction() { printf 'transaction\n' >> "${deployment_log}"; }

  main --mode=update < /dev/null > "${output_file}" 2>&1
  assert_eq v2.3.4 "${CFG_RELEASE_VERSION}" "explicit update selected version"
  assert_contains "${deployment_log}" "download|v2.3.4" \
    "explicit update passes selected version to download"
  assert_contains "${output_file}" "Version: v2.3.4" \
    "explicit update completion version"
)

test_update_preflight_failures() (
  local case_dir cert_path key_path output_file status deployment_log

  case_dir="${TEST_TMP}/update-preflight-missing-current"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  unset_deployment_invocation_environment
  require_root_environment() { return 0; }
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  download_release() { printf 'download\n' >> "${deployment_log}"; }
  snapshot_installation() { printf 'snapshot\n' >> "${deployment_log}"; }
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "update without current config unexpectedly succeeded"
  fi
  assert_contains "${output_file}" "--mode=install" "missing current config recovery guidance"
  [[ ! -s "${deployment_log}" ]] || fail "missing current config reached deployment"

  case_dir="${TEST_TMP}/update-preflight-missing-release-metadata"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  set_valid_test_config
  CFG_SSL_CERT="${cert_path}"
  CFG_SSL_KEY="${key_path}"
  CFG_RELEASE_VERSION=""
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "VERSION bypassed missing saved release metadata"
  fi
  assert_contains "${output_file}" "RELEASE_REPO and RELEASE_VERSION" "missing release metadata message"
  assert_contains "${output_file}" "--mode=install" "missing release metadata install guidance"
  assert_not_contains "${output_file}" "--mode=reconfigure" "missing release metadata does not suggest reconfigure"
  [[ ! -s "${deployment_log}" ]] || fail "missing release metadata reached deployment"

  case_dir="${TEST_TMP}/update-preflight-untrusted-env"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
  chmod 0644 "${ENV_FILE}"
  if [[ "${POSIX_MODES_SUPPORTED}" == "false" ]]; then
    CURRENT_CONFIG_TEST_STAT_MODE=644
  fi
  unset_deployment_invocation_environment
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "update accepted an untrusted saved env"
  fi
  assert_contains "${output_file}" "Repair the installed configuration" \
    "untrusted env gives repair guidance"
  assert_contains "${output_file}" "--mode=install" \
    "untrusted env gives rebuild guidance"
  assert_not_contains "${output_file}" "--mode=reconfigure" \
    "untrusted env does not suggest an unusable reconfigure path"
  [[ ! -s "${deployment_log}" ]] || fail "untrusted env reached deployment"
  unset CURRENT_CONFIG_TEST_STAT_MODE

  case_dir="${TEST_TMP}/update-preflight-mirror-version-mismatch"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
  unset_deployment_invocation_environment
  VERSION=v2.3.4
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "explicit update reused a saved mirror for a different version"
  fi
  assert_contains "${output_file}" "custom DOWNLOAD_BASE_URL" \
    "mirror/version mismatch names trust boundary"
  assert_contains "${output_file}" "--mode=install" \
    "mirror/version mismatch gives matching-mirror recovery"
  [[ ! -s "${deployment_log}" ]] || fail "mirror/version mismatch reached deployment"

  case_dir="${TEST_TMP}/update-preflight-invalid-version"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  cert_path="${case_dir}/cert.pem"
  key_path="${case_dir}/key.pem"
  : > "${cert_path}"
  : > "${key_path}"
  write_valid_current_config "${cert_path}" "${key_path}" nightly
  unset_deployment_invocation_environment
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "update accepted an invalid saved version"
  fi
  assert_contains "${output_file}" "VERSION=latest" \
    "invalid saved version gives migration selector"
  assert_not_contains "${output_file}" "--mode=reconfigure" \
    "invalid saved version does not suggest reconfigure"
  [[ ! -s "${deployment_log}" ]] || fail "invalid saved version reached deployment"

  case_dir="${TEST_TMP}/update-preflight-invalid-current"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  unset_deployment_invocation_environment
  set_valid_test_config
  CFG_DOMAIN=""
  CFG_JW_TOKEN="token-that-must-not-leak"
  CFG_JW_PASSWORD="password-that-must-not-leak"
  mkdir -p "${CONFIG_DIR}"
  render_env_file "${ENV_FILE}"
  deployment_log="${case_dir}/deployment.log"
  : > "${deployment_log}"
  output_file="${case_dir}/output.log"
  set +e
  main --mode=update < /dev/null > "${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "update with invalid saved config unexpectedly succeeded"
  fi
  assert_contains "${output_file}" "--mode=reconfigure" "invalid current config recovery guidance"
  assert_not_contains "${output_file}" "token-that-must-not-leak" "invalid config output hides token"
  assert_not_contains "${output_file}" "password-that-must-not-leak" "invalid config output hides password"
  [[ ! -s "${deployment_log}" ]] || fail "invalid current config reached deployment"

  local missing_tool
  for missing_tool in curl tar sha256sum install systemctl nginx; do
    case_dir="${TEST_TMP}/update-preflight-missing-${missing_tool}"
    mkdir -p "${case_dir}"
    setup_case "${case_dir}"
    cert_path="${case_dir}/cert.pem"
    key_path="${case_dir}/key.pem"
    : > "${cert_path}"
    : > "${key_path}"
    write_valid_current_config "${cert_path}" "${key_path}" v1.2.3
    unset_deployment_invocation_environment
    deployment_log="${case_dir}/deployment.log"
    : > "${deployment_log}"
    output_file="${case_dir}/output.log"
    command() {
      if [[ "${1:-}" == "-v" && "${2:-}" == "${missing_tool}" ]]; then
        return 1
      fi
      builtin command "$@"
    }
    set +e
    main --mode=update < /dev/null > "${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "update with missing ${missing_tool} unexpectedly succeeded"
    fi
    assert_contains "${output_file}" "requires ${missing_tool}" "missing ${missing_tool} names requirement"
    assert_contains "${output_file}" "--mode=install" "missing ${missing_tool} recovery guidance"
    [[ ! -s "${deployment_log}" ]] || fail "missing ${missing_tool} reached deployment"
  done
)

test_execute_deployment_cleans_failed_session_setup() (
  local case_dir temp_dir status

  case_dir="${TEST_TMP}/failed-session-setup"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  temp_dir="${case_dir}/installer-session"
  INSTALLER_MODE=install
  detect_arch() { printf 'amd64'; }
  mktemp() {
    if [[ "${1:-}" == "-d" ]]; then
      mkdir -p "${temp_dir}"
      printf '%s\n' "${temp_dir}"
      return 0
    fi
    command mktemp "$@"
  }
  chmod() {
    if [[ "${1:-}" == "0700" && "${2:-}" == "${temp_dir}" ]]; then
      return 1
    fi
    command chmod "$@"
  }

  set +e
  execute_deployment >/dev/null 2>&1
  status=$?
  set -e
  unset -f chmod mktemp
  if (( status == 0 )); then
    fail "deployment continued after session chmod failed"
  fi
  assert_path_absent "${temp_dir}" "failed uninitialized installer session cleanup"
)

test_transaction_directory_umask_is_safe_and_restored() (
  local case_dir temp_dir caller_umask

  case_dir="${TEST_TMP}/transaction-umask"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  rm -rf -- "${CONFIG_DIR}"
  temp_dir="${case_dir}/installer-session"
  INSTALLER_MODE=install
  set_valid_test_config
  detect_arch() { printf 'amd64'; }
  mktemp() {
    if [[ "${1:-}" == "-d" ]]; then
      mkdir -p "${temp_dir}"
      printf '%s\n' "${temp_dir}"
      return 0
    fi
    command mktemp "$@"
  }
  install_packages() { return 0; }
  create_user() { return 0; }
  download_release() { return 0; }
  prepare_staging() { return 0; }
  perform_install_transaction() {
    mkdir -p "${CONFIG_DIR}"
  }

  umask 000
  execute_deployment
  caller_umask="$(umask)"
  assert_eq 0000 "${caller_umask}" "execute_deployment restores caller umask"
  if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
    assert_mode "${CONFIG_DIR}" 755 "new configuration directory"
  fi
)

test_generator_executable_mode_contract() (
  local case_dir fixture fragment

  if [[ "${POSIX_MODES_SUPPORTED}" == "false" ]]; then
    return
  fi
  case_dir="${TEST_TMP}/generator-mode-contract"
  fixture="${case_dir}/fixture"
  mkdir -p "${fixture}/scripts/installer"
  setup_case "${case_dir}"
  cp "${INSTALLER_TEST_SCRIPT_DIR}/generate-install.sh" "${fixture}/scripts/generate-install.sh"
  for fragment in \
    00-preamble.sh 10-config.sh 20-release.sh 30-render.sh 40-transaction.sh 50-main.sh
  do
    cp "${INSTALLER_TEST_SCRIPT_DIR}/installer/${fragment}" "${fixture}/scripts/installer/${fragment}"
  done

  bash "${fixture}/scripts/generate-install.sh"
  assert_mode "${fixture}/scripts/install.sh" 755 "generated installer"

  # Executable mode is part of the tracked release artifact, not only its bytes.
  chmod 0644 "${fixture}/scripts/install.sh"
  if bash "${fixture}/scripts/generate-install.sh" --check >/dev/null 2>&1; then
    fail "generator --check accepted a non-executable installer"
  fi
  bash "${fixture}/scripts/generate-install.sh"
  assert_mode "${fixture}/scripts/install.sh" 755 "regenerated installer"

  printf '\n# intentional drift probe\n' >> \
    "${fixture}/scripts/installer/50-main.sh"
  if bash "${fixture}/scripts/generate-install.sh" --check >/dev/null 2>&1; then
    fail "generator --check accepted a stale generated installer"
  fi
)
