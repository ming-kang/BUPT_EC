# shellcheck shell=bash

require_update_tools() {
  local tool
  local -a required_tools=(curl tar sha256sum install systemctl nginx)

  for tool in "${required_tools[@]}"; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      echo "--mode=update requires ${tool}; install the required system tools or rerun with --mode=install on a supported apt-based system." >&2
      return 1
    fi
  done
}

execute_deployment() {
  local arch tmp_dir archive staging_dir backup_dir previous_umask transaction_status

  # update never runs apt. Check every non-core deployment command before
  # creating a session, downloading assets, or taking a transaction snapshot.
  if [[ "${INSTALLER_MODE}" == "update" ]]; then
    require_update_tools || return
  fi

  arch="$(detect_arch)" || return
  tmp_dir="$(mktemp -d)" || return
  if ! chmod 0700 "${tmp_dir}"; then
    rm -rf -- "${tmp_dir}" || true
    return 1
  fi
  initialize_installer_session "${tmp_dir}"
  staging_dir="${tmp_dir}/staging"
  backup_dir="${tmp_dir}/backup"

  if [[ "${INSTALLER_MODE}" != "update" ]]; then
    install_packages || return
  fi
  create_user || return
  download_release "${CFG_RELEASE_REPO}" "${CFG_RELEASE_VERSION}" "${arch}" "${tmp_dir}" "${CFG_DOWNLOAD_BASE_URL}" || return
  archive="${tmp_dir}/bupt-ec-linux-${arch}.tar.gz"
  prepare_staging "${archive}" "${tmp_dir}" "${staging_dir}" || return

  # commit_installation creates fixed installation/config directories. Force a
  # safe creation mode even when root invoked the installer with a permissive
  # umask, then restore the caller's value on every transaction result.
  previous_umask="$(umask)" || return
  umask 022 || return
  transaction_status=0
  perform_install_transaction "${staging_dir}" "${backup_dir}" "${CFG_APP_ADDR}" || transaction_status=$?
  if ! umask "${previous_umask}"; then
    return 1
  fi
  return "${transaction_status}"
}

print_completion_summary() {
  case "${INSTALLER_MODE}" in
    install)
      # Keep the compatible default-install completion text byte-for-byte.
      echo
      echo "BUPT_EC is installed."
      echo "URL: https://${CFG_DOMAIN}/"
      echo "Service: systemctl status ${SERVICE_NAME}"
      echo "Upgrade later: rerun this installer."
      ;;
    update)
      echo
      echo "BUPT_EC update completed."
      echo "Version: ${CFG_RELEASE_VERSION}"
      echo "URL: https://${CFG_DOMAIN}/"
      echo "Service: systemctl status ${SERVICE_NAME}"
      ;;
    reconfigure)
      echo
      echo "BUPT_EC reconfiguration completed."
      echo "Version: ${CFG_RELEASE_VERSION}"
      echo "URL: https://${CFG_DOMAIN}/"
      echo "Service: systemctl status ${SERVICE_NAME}"
      ;;
  esac
}

main() {
  parse_mode "$@" || return
  require_root_environment || return
  capture_invocation_overrides || return
  if ! load_current_config; then
    current_config_load_guidance
    return 1
  fi

  case "${INSTALLER_MODE}" in
    install)
      require_interactive_tty || return
      collect_config_interactive install || return
      ;;
    update)
      require_existing_installation update || return
      adopt_current_config || return
      ;;
    reconfigure)
      require_interactive_tty || return
      require_existing_installation reconfigure || return
      collect_config_interactive reconfigure || return
      ;;
  esac

  validate_config || return
  execute_deployment || return
  print_completion_summary
}

if [[ "${INSTALLER_SOURCED}" != "true" ]]; then
  main "$@"
fi
