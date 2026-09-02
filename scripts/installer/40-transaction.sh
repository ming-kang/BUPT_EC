# shellcheck shell=bash
# shellcheck disable=SC2153

transaction_targets() {
  printf '%s\t%s\n' \
    binary "${INSTALL_DIR}/bupt-ec" \
    env "${ENV_FILE}" \
    cli "${CLI_FILE}" \
    metadata "${DEPLOYMENT_METADATA_FILE}" \
    service "${SERVICE_FILE}" \
    systemd_enabled "${SYSTEMD_ENABLED_LINK}" \
    nginx_site "${NGINX_SITE}" \
    nginx_enabled "${NGINX_ENABLED}"
}

write_runtime_snapshot() {
  local destination="$1"
  local service_present=0
  local service_enabled=0
  local service_active=0
  local nginx_site_present=0
  local nginx_enabled=0

  if [[ -e "${SERVICE_FILE}" || -L "${SERVICE_FILE}" ]]; then
    service_present=1
  fi
  if systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
    service_enabled=1
  fi
  if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
    service_active=1
  fi
  if [[ -e "${NGINX_SITE}" || -L "${NGINX_SITE}" ]]; then
    nginx_site_present=1
  fi
  if [[ -e "${NGINX_ENABLED}" || -L "${NGINX_ENABLED}" ]]; then
    nginx_enabled=1
  fi

  (umask 077; cat > "${destination}" <<EOF
service_present=${service_present}
service_enabled=${service_enabled}
service_active=${service_active}
nginx_site_present=${nginx_site_present}
nginx_enabled=${nginx_enabled}
EOF
  ) || return
  chmod 0600 "${destination}" || return
}

read_runtime_snapshot_value() {
  local snapshot="$1"
  local key="$2"
  local line value

  line="$(grep -E "^${key}=" "${snapshot}" 2>/dev/null || true)"
  value="${line#*=}"
  if [[ "${value}" == "1" ]]; then
    printf '1'
  else
    printf '0'
  fi
}

snapshot_installation() {
  local backup_dir="$1"
  local manifest="${backup_dir}/manifest"
  local runtime_state="${backup_dir}/runtime_state"
  local role target

  rm -rf "${backup_dir}" || return
  mkdir -p "${backup_dir}" || return
  chmod 0700 "${backup_dir}" || return
  (umask 077; : > "${manifest}") || return

  while IFS=$'\t' read -r role target; do
    if [[ -e "${target}" || -L "${target}" ]]; then
      if ! cp -a -- "${target}" "${backup_dir}/${role}"; then
        echo "Failed to snapshot ${role}." >&2
        return 1
      fi
      printf '%s\t1\t%s\n' "${role}" "${target}" >> "${manifest}" || return
    else
      printf '%s\t0\t%s\n' "${role}" "${target}" >> "${manifest}" || return
    fi
  done < <(transaction_targets)
  chmod 0600 "${manifest}" || return
  if [[ -f "${backup_dir}/env" ]]; then
    chmod 0600 "${backup_dir}/env" || return
  fi
  write_runtime_snapshot "${runtime_state}" || return
}

atomic_install_file() {
  local source="$1"
  local target="$2"
  local mode="$3"
  local owner="$4"
  local target_dir tmp

  target_dir="$(dirname "${target}")"
  tmp="${target}.new.$$"
  mkdir -p "${target_dir}" || return
  rm -f -- "${tmp}" || return
  install -m "${mode}" "${source}" "${tmp}" || { rm -f -- "${tmp}"; return 1; }
  chown "${owner}" "${tmp}" || { rm -f -- "${tmp}"; return 1; }
  mv -Tf -- "${tmp}" "${target}" || { rm -f -- "${tmp}"; return 1; }
}

atomic_install_symlink() {
  local link_target="$1"
  local target="$2"
  local target_dir tmp

  target_dir="$(dirname "${target}")"
  tmp="${target}.new.$$"
  mkdir -p "${target_dir}" || return
  rm -f -- "${tmp}" || return
  ln -s "${link_target}" "${tmp}" || { rm -f -- "${tmp}"; return 1; }
  mv -Tf -- "${tmp}" "${target}" || { rm -f -- "${tmp}"; return 1; }
}

restore_snapshot_target() {
  local backup="$1"
  local target="$2"
  local target_dir tmp

  target_dir="$(dirname "${target}")"
  tmp="${target}.rollback.$$"
  mkdir -p "${target_dir}" || return
  rm -rf -- "${tmp}" || return
  cp -a -- "${backup}" "${tmp}" || { rm -rf -- "${tmp}"; return 1; }
  mv -Tf -- "${tmp}" "${target}" || { rm -rf -- "${tmp}"; return 1; }
}

rollback_installation() {
  local backup_dir="$1"
  local role existed target
  local failed=0
  local runtime_state="${backup_dir}/runtime_state"
  local service_present=0
  local service_enabled=0
  local service_active=0

  if [[ ! -r "${backup_dir}/manifest" ]]; then
    echo "Rollback manifest is missing or unreadable." >&2
    return 1
  fi

  if [[ -r "${runtime_state}" ]]; then
    service_present="$(read_runtime_snapshot_value "${runtime_state}" service_present)"
    service_enabled="$(read_runtime_snapshot_value "${runtime_state}" service_enabled)"
    service_active="$(read_runtime_snapshot_value "${runtime_state}" service_active)"
  fi

  echo "Installation failed; rolling back previous files..." >&2

  # Stop any running unit before replacing/removing files so first-install late
  # failures cannot leave a process without a unit file.
  if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
    if ! systemctl stop "${SERVICE_NAME}" >/dev/null 2>&1; then
      echo "Rollback failed while stopping ${SERVICE_NAME}." >&2
      failed=1
    fi
  fi

  while IFS=$'\t' read -r role existed target; do
    if ! rm -f -- "${target}.new.$$" "${target}.rollback.$$"; then
      echo "Rollback failed while cleaning temporary ${role} files." >&2
      failed=1
    fi
    if [[ "${existed}" == "1" ]]; then
      if ! restore_snapshot_target "${backup_dir}/${role}" "${target}"; then
        echo "Rollback failed while restoring ${role}." >&2
        failed=1
      fi
    elif ! rm -rf -- "${target}"; then
      echo "Rollback failed while removing new ${role}." >&2
      failed=1
    fi
  done < "${backup_dir}/manifest"

  if ! systemctl daemon-reload >/dev/null 2>&1; then
    echo "Rollback failed during systemctl daemon-reload." >&2
    failed=1
  fi

  if [[ "${service_present}" == "1" ]]; then
    if [[ "${service_enabled}" == "1" ]]; then
      if ! systemctl enable "${SERVICE_NAME}" >/dev/null 2>&1; then
        echo "Rollback failed while re-enabling ${SERVICE_NAME}." >&2
        failed=1
      fi
    else
      if ! systemctl disable "${SERVICE_NAME}" >/dev/null 2>&1; then
        echo "Rollback failed while disabling ${SERVICE_NAME}." >&2
        failed=1
      fi
    fi
    if [[ "${service_active}" == "1" ]]; then
      if ! systemctl start "${SERVICE_NAME}" >/dev/null 2>&1; then
        echo "Rollback failed while starting previous ${SERVICE_NAME}." >&2
        failed=1
      fi
    fi
  else
    # First install: ensure enablement is gone and the unit is not left active.
    rm -f -- "${SYSTEMD_ENABLED_LINK}" >/dev/null 2>&1 || failed=1
    if systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
      if ! systemctl stop "${SERVICE_NAME}" >/dev/null 2>&1; then
        echo "Rollback failed while stopping new ${SERVICE_NAME}." >&2
        failed=1
      fi
    fi
  fi

  # Always revalidate and reload Nginx so a newly loaded site is dropped even
  # when no previous site existed (first-install late failure after reload).
  if ! nginx -t >/dev/null 2>&1; then
    echo "Rollback failed during nginx configuration test." >&2
    failed=1
  fi
  if ! systemctl reload nginx >/dev/null 2>&1; then
    echo "Rollback failed while reloading nginx." >&2
    failed=1
  fi

  if (( failed == 0 )); then
    echo "Rollback completed." >&2
  else
    echo "Rollback completed with errors; inspect systemd and nginx state." >&2
  fi
  return "${failed}"
}

local_health_url() {
  local app_addr="$1"
  if [[ "${app_addr}" =~ ^(127\.0\.0\.1|localhost):[0-9]{1,5}$ || "${app_addr}" =~ ^\[::1\]:[0-9]{1,5}$ ]]; then
    printf 'http://%s/healthz' "${app_addr}"
    return
  fi
  return 1
}

wait_for_health() {
  local health_url="$1"
  local attempt

  for attempt in {1..10}; do
    if curl -fsS --noproxy '*' --connect-timeout 2 --max-time 2 "${health_url}" >/dev/null 2>&1; then
      return
    fi
    if (( attempt < 10 )); then
      sleep 1
    fi
  done

  echo "Service health check failed: ${health_url}" >&2
  return 1
}

validate_cli_staging_directory() {
  local staging_dir="$1"
  local owner mode

  if [[ -L "${staging_dir}" || ! -d "${staging_dir}" ]]; then
    return 1
  fi
  owner="$(stat -c '%u' -- "${staging_dir}" 2>/dev/null)" || return 1
  mode="$(stat -c '%a' -- "${staging_dir}" 2>/dev/null)" || return 1
  [[ "${owner}" == "${ENV_FILE_EXPECTED_UID}" && "${mode}" == "700" ]]
}

validate_cli_staging_regular_file() {
  local candidate="$1"
  local expected_mode="$2"
  local owner mode

  if [[ -L "${candidate}" || ! -f "${candidate}" ]]; then
    return 1
  fi
  owner="$(stat -c '%u' -- "${candidate}" 2>/dev/null)" || return 1
  mode="$(stat -c '%a' -- "${candidate}" 2>/dev/null)" || return 1
  [[ "${owner}" == "${ENV_FILE_EXPECTED_UID}" && "${mode}" == "${expected_mode}" ]]
}

read_cli_staging_action() {
  local action_file="$1"
  local action="" extra=""

  validate_cli_staging_regular_file "${action_file}" 600 || return 1
  {
    IFS= read -r action || return 1
    if IFS= read -r extra; then
      return 1
    fi
    [[ -z "${extra}" ]] || return 1
  } < "${action_file}" || return 1
  case "${action}" in
    install | remove) ;;
    *) return 1 ;;
  esac
  # `read` drops NUL bytes. The byte comparison also requires the terminating
  # newline and rejects an action whose valid-looking first line has trailers.
  cmp -s -- "${action_file}" <(printf '%s\n' "${action}") || return 1
  printf '%s' "${action}"
}

staged_metadata_matches_config() {
  local metadata_file="$1"

  cmp -s -- "${metadata_file}" <(
    printf 'RELEASE_VERSION=%s\nAPP_ADDR=%s\n' "${CFG_RELEASE_VERSION}" "${CFG_APP_ADDR}"
  )
}

commit_installation() {
  local staging_dir="$1"
  local app_addr="$2"
  local health_url cli_action expected_cli_action

  if ! validate_cli_staging_directory "${staging_dir}"; then
    echo "Staging directory is missing or unsafe." >&2
    return 1
  fi
  if ! cli_action="$(read_cli_staging_action "${staging_dir}/cli.action")"; then
    echo "Staged CLI action is missing or invalid." >&2
    return 1
  fi
  if is_cli_bearing_release "${CFG_RELEASE_VERSION}"; then
    expected_cli_action=install
  else
    expected_cli_action=remove
  fi
  if [[ "${cli_action}" != "${expected_cli_action}" ]]; then
    echo "Staged CLI action does not match the selected release." >&2
    return 1
  fi
  if [[ "${cli_action}" == "install" ]]; then
    if ! validate_cli_staging_regular_file "${staging_dir}/bupt-ec-cli" 755 ||
       ! validate_cli_staging_regular_file "${staging_dir}/deployment.meta" 644 ||
       ! staged_metadata_matches_config "${staging_dir}/deployment.meta"; then
      echo "Staged CLI candidates are missing or invalid." >&2
      return 1
    fi
  elif [[ -e "${staging_dir}/bupt-ec-cli" || -L "${staging_dir}/bupt-ec-cli" ||
          -e "${staging_dir}/deployment.meta" || -L "${staging_dir}/deployment.meta" ]]; then
    echo "Staged CLI remove action must not include CLI candidates." >&2
    return 1
  fi

  mkdir -p "${INSTALL_DIR}/run_log" "${CONFIG_DIR}" || return
  chmod 0755 "${INSTALL_DIR}" || return
  chown root:root "${INSTALL_DIR}" || return
  chown -R "${APP_USER}:${APP_GROUP}" "${INSTALL_DIR}/run_log" || return
  chmod 0750 "${INSTALL_DIR}/run_log" || return

  atomic_install_file "${staging_dir}/bupt-ec" "${INSTALL_DIR}/bupt-ec" 0755 root:root || return
  atomic_install_file "${staging_dir}/bupt-ec.env" "${ENV_FILE}" 0600 root:root || return
  if [[ "${cli_action}" == "install" ]]; then
    atomic_install_file "${staging_dir}/bupt-ec-cli" "${CLI_FILE}" 0755 root:root || return
    atomic_install_file "${staging_dir}/deployment.meta" "${DEPLOYMENT_METADATA_FILE}" 0644 root:root || return
  else
    rm -f -- "${CLI_FILE}" || return
    rm -f -- "${DEPLOYMENT_METADATA_FILE}" || return
  fi
  atomic_install_file "${staging_dir}/${SERVICE_NAME}.service" "${SERVICE_FILE}" 0644 root:root || return
  atomic_install_file "${staging_dir}/${SERVICE_NAME}.conf" "${NGINX_SITE}" 0644 root:root || return
  atomic_install_symlink "${NGINX_SITE}" "${NGINX_ENABLED}" || return

  systemctl daemon-reload || return
  systemctl enable "${SERVICE_NAME}" || return
  nginx -t || return
  systemctl restart "${SERVICE_NAME}" || return
  systemctl is-active --quiet "${SERVICE_NAME}" || return
  systemctl reload nginx || return

  if health_url="$(local_health_url "${app_addr}")"; then
    wait_for_health "${health_url}" || return
  else
    echo "Skipping local health check for non-loopback APP_ADDR=${app_addr}." >&2
  fi
}

perform_install_transaction() {
  local staging_dir="$1"
  local backup_dir="$2"
  local app_addr="$3"
  local status

  snapshot_installation "${backup_dir}" || return
  TRANSACTION_BACKUP_DIR="${backup_dir}"
  TRANSACTION_ACTIVE=true
  commit_installation "${staging_dir}" "${app_addr}" || {
    status=$?
    return "${status}"
  }
  TRANSACTION_ACTIVE=false
  if ! rm -rf "${backup_dir}"; then
    echo "Installation validated, but the transaction backup could not be removed." >&2
    return 1
  fi
  TRANSACTION_BACKUP_DIR=""
}

installer_cleanup() {
  local status="$1"
  local rollback_status=0
  local preserve_tmp=false

  trap - ERR EXIT
  set +e
  if [[ "${TRANSACTION_ACTIVE}" == "true" && -n "${TRANSACTION_BACKUP_DIR}" ]]; then
    if (( status == 0 )); then
      status=1
    fi
    rollback_installation "${TRANSACTION_BACKUP_DIR}"
    rollback_status=$?
    if (( rollback_status != 0 )); then
      preserve_tmp=true
    fi
  fi
  if [[ -n "${INSTALLER_TMP_DIR}" && "${preserve_tmp}" == "false" ]]; then
    rm -rf "${INSTALLER_TMP_DIR}"
  fi
  if (( rollback_status != 0 )); then
    echo "Automatic rollback was incomplete; root-only recovery files were preserved at ${TRANSACTION_BACKUP_DIR}." >&2
  fi
  exit "${status}"
}

initialize_installer_session() {
  local tmp_dir="$1"
  INSTALLER_TMP_DIR="${tmp_dir}"
  trap 'installer_cleanup "$?"' EXIT
}
