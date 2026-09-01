# shellcheck shell=bash

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
    (download_release "ming-kang/BUPT_EC" "v0.1.4" amd64 "${work_dir}" "https://mirror.example/v0.1.4") > "${output}" 2>&1
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
  assert_service_active true "nginx failure restores previous active service"
  assert_service_enabled true "nginx failure restores previous enablement"
  assert_command_count 2 "nginx -t" "${MOCK_COMMAND_LOG}" "nginx validation plus rollback validation count"
  assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "nginx rollback stops service before restore"
  assert_command_count 1 "systemctl start ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "nginx rollback restarts previous active service"
  assert_command_count 1 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "nginx rollback reloads nginx"
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
    assert_service_active true "${failure} failure restores previous active service"
    assert_service_enabled true "${failure} failure restores previous enablement"
    assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "${failure} path attempts new service restart"
    assert_command_count 1 "systemctl start ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "${failure} path restores previous active service"
    if [[ "${failure}" == "health" ]]; then
      assert_command_count 10 "curl http://127.0.0.1:8080/healthz" "${MOCK_COMMAND_LOG}" "health failure retry count"
      assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "health failure stops new service before restore"
    fi
    assert_contains "${output}" "Rollback completed." "${failure} rollback output"
    assert_path_absent "${session_dir}" "completed ${failure} rollback session cleanup"
  done
}

test_upgrade_inactive_service_stays_inactive_on_rollback() {
  local case_dir session_dir staging_dir backup_dir output before after
  case_dir="${TEST_TMP}/inactive-upgrade-rollback"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  seed_existing_installation false false
  make_staging "${staging_dir}"
  before="$(capture_target_state)"
  export MOCK_HEALTH_FAILURES=10

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "inactive upgrade health failure unexpectedly committed"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "inactive upgrade restores installed targets"
  assert_service_active false "inactive upgrade leaves previous service inactive"
  assert_service_enabled false "inactive upgrade leaves previous service disabled"
  assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "inactive upgrade still attempts new restart"
  assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "inactive upgrade stops newly started service"
  assert_command_count 0 "systemctl start ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "inactive upgrade must not start previously inactive service"
  assert_command_count 1 "systemctl disable ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "inactive upgrade re-disables previous service"
  assert_contains "${output}" "Rollback completed." "inactive upgrade rollback output"
  assert_path_absent "${session_dir}" "completed inactive upgrade rollback session cleanup"
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
  assert_service_active false "early first install leaves service inactive"
  assert_command_count 0 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "first install early rollback restart count"
  assert_command_count 0 "systemctl start ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "first install early rollback start count"
  assert_command_count 1 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "first install early rollback still reloads nginx"
  assert_not_contains "${output}" "BUPT_EC is installed." "first install rollback success output"
  assert_path_absent "${session_dir}" "completed first install rollback session cleanup"
}

test_first_install_late_failures_stop_service_and_reload_nginx() {
  local failure case_dir session_dir staging_dir backup_dir output role target
  for failure in restart is-active reload health; do
    case_dir="${TEST_TMP}/first-install-late-${failure}"
    session_dir="${case_dir}/session"
    staging_dir="${session_dir}/staging"
    backup_dir="${session_dir}/backup"
    output="${case_dir}/output.log"
    mkdir -p "${session_dir}"
    chmod 0700 "${session_dir}"
    setup_case "${case_dir}"
    make_staging "${staging_dir}"

    case "${failure}" in
      restart)
        export MOCK_SYSTEMCTL_FAIL_COMMAND=restart
        export MOCK_SYSTEMCTL_FAIL_ON_CALL=1
        ;;
      is-active)
        # Call 1 is the pre-commit runtime snapshot probe; fail the post-restart check.
        export MOCK_SYSTEMCTL_FAIL_COMMAND=is-active
        export MOCK_SYSTEMCTL_FAIL_ON_CALL=2
        ;;
      reload)
        export MOCK_SYSTEMCTL_FAIL_COMMAND=reload
        export MOCK_SYSTEMCTL_FAIL_ON_CALL=1
        ;;
      health)
        export MOCK_HEALTH_FAILURES=10
        ;;
    esac

    if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
      fail "first install late ${failure} failure unexpectedly committed"
    fi
    while IFS=$'\t' read -r role target; do
      assert_path_absent "${target}" "first install late ${failure} ${role}"
    done < <(transaction_targets)
    assert_service_active false "first install late ${failure} leaves no active service"
    assert_service_enabled false "first install late ${failure} leaves service disabled"
    assert_not_contains "${output}" "BUPT_EC is installed." "first install late ${failure} success output"
    assert_contains "${output}" "Rollback completed." "first install late ${failure} rollback output"
    assert_path_absent "${session_dir}" "completed first install late ${failure} session cleanup"

    case "${failure}" in
      restart)
        assert_command_count 0 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "restart fail never started service"
        assert_command_count 1 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "restart fail still reloads nginx on rollback"
        ;;
      is-active)
        # restart succeeded and marked active; rollback must stop it.
        assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "is-active path restarts new service"
        assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "is-active fail stops new service"
        assert_command_count 1 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "is-active fail reloads nginx on rollback"
        ;;
      reload)
        assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "reload path restarts new service"
        assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "reload fail stops new service"
        # commit reload fails (call 1); rollback reload is call 2 and should run.
        assert_command_count 2 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "reload fail retries nginx reload on rollback"
        ;;
      health)
        assert_command_count 1 "systemctl restart ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "health path restarts new service"
        assert_command_count 1 "systemctl stop ${SERVICE_NAME}" "${MOCK_COMMAND_LOG}" "health fail stops new service"
        assert_command_count 2 "systemctl reload nginx" "${MOCK_COMMAND_LOG}" "health fail reloads nginx on commit and rollback"
        assert_command_count 10 "curl http://127.0.0.1:8080/healthz" "${MOCK_COMMAND_LOG}" "first install health failure retries"
        ;;
    esac
  done
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
  export MOCK_SYSTEMCTL_FAIL_COMMAND=start
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
  assert_mode "${backup_dir}/runtime_state" 600 "preserved runtime snapshot"
  assert_contains "${output}" "Automatic rollback was incomplete" "incomplete rollback output"
  assert_contains "${output}" "${backup_dir}" "incomplete rollback recovery path"
  rm -rf "${session_dir}"
}

test_invalid_cli_action_rolls_back() {
  local case_dir session_dir staging_dir backup_dir output before after

  case_dir="${TEST_TMP}/invalid-cli-action"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}"
  printf 'install\nextra\n' > "${staging_dir}/cli.action"
  before="$(capture_target_state)"

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "invalid CLI action unexpectedly committed"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "invalid CLI action restores all targets"
  assert_contains "${output}" "Staged CLI action is missing or invalid." "invalid CLI action error"
  assert_path_absent "${session_dir}" "invalid CLI action session cleanup"
}

test_cli_staging_validation() {
  local scenario case_dir session_dir staging_dir backup_dir output before after

  for scenario in action-nul action-mode candidate-mode metadata-content action-version remove-candidate staging-mode; do
    case_dir="${TEST_TMP}/cli-staging-${scenario}"
    session_dir="${case_dir}/session"
    staging_dir="${session_dir}/staging"
    backup_dir="${session_dir}/backup"
    output="${case_dir}/output.log"
    mkdir -p "${session_dir}"
    chmod 0700 "${session_dir}"
    setup_case "${case_dir}"
    seed_existing_installation

    case "${scenario}" in
      remove-candidate)
        make_staging "${staging_dir}" remove
        printf 'unexpected cli\n' > "${staging_dir}/bupt-ec-cli"
        chmod 0755 "${staging_dir}/bupt-ec-cli"
        ;;
      *) make_staging "${staging_dir}" ;;
    esac

    case "${scenario}" in
      action-nul)
        printf 'install\0\n' > "${staging_dir}/cli.action"
        ;;
      action-mode)
        if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
          chmod 0644 "${staging_dir}/cli.action"
        else
          export CLI_ACTION_TEST_STAT_MODE=644
        fi
        ;;
      candidate-mode)
        if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
          chmod 0644 "${staging_dir}/bupt-ec-cli"
        else
          export CLI_CANDIDATE_TEST_STAT_MODE=644
        fi
        ;;
      metadata-content)
        printf 'RELEASE_VERSION=v9.9.9\nAPP_ADDR=127.0.0.1:9999\n' > "${staging_dir}/deployment.meta"
        ;;
      action-version)
        render_cli_action "${staging_dir}/cli.action" remove
        ;;
      remove-candidate)
        ;;
      staging-mode)
        if [[ "${POSIX_MODES_SUPPORTED}" == "true" ]]; then
          chmod 0755 "${staging_dir}"
        else
          export CLI_STAGING_DIR_TEST_MODE=755
        fi
        ;;
    esac

    before="$(capture_target_state)"
    if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
      fail "CLI staging ${scenario} unexpectedly committed"
    fi
    after="$(capture_target_state)"
    assert_eq "${before}" "${after}" "CLI staging ${scenario} restores all targets"
    assert_path_absent "${session_dir}" "CLI staging ${scenario} session cleanup"
  done
}

test_legacy_cli_removal_and_rollback() {
  local case_dir session_dir staging_dir backup_dir output before after

  case_dir="${TEST_TMP}/legacy-cli-removal"
  staging_dir="${case_dir}/staging"
  backup_dir="${case_dir}/backup"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}" remove
  perform_install_transaction "${staging_dir}" "${backup_dir}" "127.0.0.1:8080"
  assert_path_absent "${CLI_FILE}" "legacy successful transaction CLI removal"
  assert_path_absent "${DEPLOYMENT_METADATA_FILE}" "legacy successful transaction metadata removal"
  assert_path_absent "${backup_dir}" "legacy successful transaction backup cleanup"

  case_dir="${TEST_TMP}/legacy-cli-rollback"
  session_dir="${case_dir}/session"
  staging_dir="${session_dir}/staging"
  backup_dir="${session_dir}/backup"
  output="${case_dir}/output.log"
  mkdir -p "${session_dir}"
  chmod 0700 "${session_dir}"
  setup_case "${case_dir}"
  seed_existing_installation
  make_staging "${staging_dir}" remove
  before="$(capture_target_state)"
  export MOCK_HEALTH_FAILURES=10

  if run_transaction_with_cleanup "${session_dir}" "${staging_dir}" "${backup_dir}" > "${output}" 2>&1; then
    fail "legacy CLI removal health failure unexpectedly committed"
  fi
  after="$(capture_target_state)"
  assert_eq "${before}" "${after}" "legacy removal health failure restores CLI and metadata"
  assert_path_absent "${session_dir}" "legacy rollback session cleanup"
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
  assert_mode "${staging_dir}/bupt-ec-cli" 755 "candidate CLI"
  assert_mode "${staging_dir}/deployment.meta" 644 "candidate metadata"
  assert_mode "${staging_dir}/cli.action" 600 "candidate CLI action"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${staging_dir}/bupt-ec.env" "candidate env ownership"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${staging_dir}/bupt-ec-cli" "candidate CLI ownership"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${staging_dir}/deployment.meta" "candidate metadata ownership"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${staging_dir}/cli.action" "candidate action ownership"

  snapshot_installation "${preview_backup}"
  assert_mode "${preview_backup}" 700 "backup directory"
  assert_mode "${preview_backup}/manifest" 600 "backup manifest"
  assert_mode "${preview_backup}/env" 600 "backup env"
  rm -rf "${preview_backup}"

  perform_install_transaction "${staging_dir}" "${backup_dir}" "127.0.0.1:8080"

  cmp -s "${staging_dir}/bupt-ec" "${INSTALL_DIR}/bupt-ec" || fail "successful upgrade binary mismatch"
  cmp -s "${staging_dir}/bupt-ec.env" "${ENV_FILE}" || fail "successful upgrade env mismatch"
  cmp -s "${staging_dir}/bupt-ec-cli" "${CLI_FILE}" || fail "successful upgrade CLI mismatch"
  cmp -s "${staging_dir}/deployment.meta" "${DEPLOYMENT_METADATA_FILE}" || fail "successful upgrade metadata mismatch"
  cmp -s "${staging_dir}/${SERVICE_NAME}.service" "${SERVICE_FILE}" || fail "successful upgrade service mismatch"
  cmp -s "${staging_dir}/${SERVICE_NAME}.conf" "${NGINX_SITE}" || fail "successful upgrade nginx mismatch"
  assert_enabled_target "${SYSTEMD_ENABLED_LINK}" "${SERVICE_FILE}" "successful upgrade systemd enablement"
  assert_enabled_target "${NGINX_ENABLED}" "${NGINX_SITE}" "successful upgrade nginx enablement"
  assert_mode "${INSTALL_DIR}/bupt-ec" 755 "installed binary"
  assert_mode "${ENV_FILE}" 600 "installed env"
  assert_mode "${CLI_FILE}" 755 "installed CLI"
  assert_mode "${DEPLOYMENT_METADATA_FILE}" 644 "installed metadata"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${ENV_FILE}.new." "installed env ownership"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${CLI_FILE}.new." "installed CLI ownership"
  assert_contains "${MOCK_COMMAND_LOG}" "chown root:root ${DEPLOYMENT_METADATA_FILE}.new." "installed metadata ownership"
  assert_mode "${SERVICE_FILE}" 644 "installed service"
  assert_mode "${NGINX_SITE}" 644 "installed nginx"
  assert_path_absent "${backup_dir}" "successful upgrade backup cleanup"
  assert_eq false "${TRANSACTION_ACTIVE}" "successful transaction active flag"
  assert_eq "" "${TRANSACTION_BACKUP_DIR}" "successful transaction backup pointer"
  assert_command_count 1 "curl http://127.0.0.1:8080/healthz" "${MOCK_COMMAND_LOG}" "successful health check count"
}
