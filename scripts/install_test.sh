#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC1091
set -euo pipefail

INSTALLER_TEST_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly INSTALLER_TEST_SCRIPT_DIR

bash "${INSTALLER_TEST_SCRIPT_DIR}/generate-install.sh" --check
# shellcheck source-path=SCRIPTDIR
# shellcheck source=install.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/install.sh"
# shellcheck source-path=SCRIPTDIR/installer_test
# shellcheck source=installer_test/assertions.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/assertions.sh"
# shellcheck source=installer_test/mocks.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/mocks.sh"
# shellcheck source=installer_test/policy.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/policy.sh"
# shellcheck source=installer_test/render_config.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/render_config.sh"
# shellcheck source=installer_test/transaction.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/transaction.sh"
# shellcheck source=installer_test/modes.sh
source "${INSTALLER_TEST_SCRIPT_DIR}/installer_test/modes.sh"

run_suite() {
  local suite="$1"
  shift
  local test_name

  printf 'Running %s suite (%d entry scenarios)...\n' "${suite}" "$#"
  for test_name in "$@"; do
    "${test_name}"
  done
}

expected_test_function_count=39
actual_test_function_count="$(declare -F | awk '$3 ~ /^test_/ { count++ } END { print count + 0 }')"
assert_eq "${expected_test_function_count}" "${actual_test_function_count}" \
  "installer test function inventory"

run_suite modes \
  test_entrypoint_stdin_pipe_reaches_root_check \
  test_parse_mode_matrix \
  test_interactive_modes_require_tty \
  test_prompt_return_values_do_not_capture_feedback \
  test_no_arg_main_default_completion_output \
  test_no_mode_install_prompt_order_baseline \
  test_no_mode_install_existing_prompt_defaults \
  test_mode_version_semantics \
  test_reconfigure_main_uses_saved_version \
  test_runtime_flags_across_modes \
  test_update_noninteractive_skips_packages \
  test_update_explicit_version_reaches_download \
  test_update_preflight_failures \
  test_execute_deployment_cleans_failed_session_setup \
  test_transaction_directory_umask_is_safe_and_restored \
  test_generator_executable_mode_contract
run_suite policy \
  test_version_policy \
  test_app_addr_validation \
  test_download_release_stops_after_base_resolution_failure \
  test_checksum_bypass_requires_exact_one
# Keep the former transactional scenario order; isolated fixtures still
# establish a fresh root per case.
run_suite transaction \
  test_checksum_failures_preserve_targets
run_suite render-config \
  test_deployment_config_key_contract \
  test_config_registry_precedence_and_isolation \
  test_current_config_load_security_and_snapshot \
  test_render_failure_propagation \
  test_config_render_round_trip \
  test_staging_failures_preserve_targets
run_suite transaction \
  test_snapshot_failure_preserves_targets \
  test_nginx_failure_rolls_back_upgrade \
  test_restart_and_health_failures_roll_back_upgrade \
  test_upgrade_inactive_service_stays_inactive_on_rollback \
  test_first_install_failure_removes_new_targets \
  test_first_install_late_failures_stop_service_and_reload_nginx \
  test_incomplete_rollback_preserves_recovery_backup
run_suite render-config \
  test_api_proxy_read_timeout_budget
run_suite transaction \
  test_successful_upgrade_commits_and_cleans_backup

printf 'installer behavior tests passed (36 entry scenarios; %s test functions)\n' \
  "${actual_test_function_count}"
