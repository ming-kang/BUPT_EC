# shellcheck shell=bash
# shellcheck disable=SC2034

installer_usage() {
  echo "Usage: install.sh [--mode=install|update|reconfigure]" >&2
}

parse_mode() {
  local mode=""
  local seen_mode=false

  INSTALLER_MODE="install"
  while (( $# > 0 )); do
    case "$1" in
      --mode=*)
        if [[ "${seen_mode}" == "true" ]]; then
          echo "--mode may only be specified once." >&2
          installer_usage
          return 2
        fi
        mode="${1#--mode=}"
        if [[ -z "${mode}" ]]; then
          echo "--mode requires a value." >&2
          installer_usage
          return 2
        fi
        seen_mode=true
        ;;
      --mode)
        if [[ "${seen_mode}" == "true" ]]; then
          echo "--mode may only be specified once." >&2
          installer_usage
          return 2
        fi
        if (( $# < 2 )) || [[ -z "${2}" || "${2}" == --* ]]; then
          echo "--mode requires a value." >&2
          installer_usage
          return 2
        fi
        mode="$2"
        seen_mode=true
        shift
        ;;
      *)
        echo "Unexpected installer argument." >&2
        installer_usage
        return 2
        ;;
    esac
    shift
  done

  if [[ "${seen_mode}" == "false" ]]; then
    return 0
  fi

  case "${mode}" in
    install | update | reconfigure)
      INSTALLER_MODE="${mode}"
      ;;
    *)
      echo "--mode must be install, update, or reconfigure." >&2
      installer_usage
      return 2
      ;;
  esac
}

require_root_environment() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "This installer must run as root. Use: curl -fsSL <url> | sudo bash" >&2
    return 1
  fi
}

require_interactive_tty() {
  # A readable /dev/tty device node can still be unopenable when the process
  # has no controlling terminal; probe the actual open before any prompt.
  if [[ ! -r "${TTY}" ]] || ! : < "${TTY}" 2>/dev/null; then
    echo "Interactive input requires a TTY." >&2
    return 1
  fi
}

reset_config_state() {
  local key

  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "CURRENT_${key}" '%s' ""
    printf -v "OVERRIDE_${key}" '%s' ""
    printf -v "OVERRIDE_${key}_SET" '%s' "false"
    printf -v "CFG_${key}" '%s' ""
  done
  OVERRIDE_VERSION=""
  OVERRIDE_VERSION_SET=false
  VALIDATED_DOWNLOAD_BASE_URL=""
  INSTALLER_MODE="install"
}

clear_current_config() {
  local key
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "CURRENT_${key}" '%s' ""
  done
}

clear_invocation_overrides() {
  local key
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "OVERRIDE_${key}" '%s' ""
    printf -v "OVERRIDE_${key}_SET" '%s' "false"
  done
  OVERRIDE_VERSION=""
  OVERRIDE_VERSION_SET=false
}

capture_config_override() {
  local key="$1"
  local source_name="$2"

  if [[ -v "${source_name}" ]]; then
    printf -v "OVERRIDE_${key}" '%s' "${!source_name}"
    printf -v "OVERRIDE_${key}_SET" '%s' "true"
  fi
}

# Snapshot invocation values before reading the installed env. Explicitly set
# empty values are retained through the paired *_SET flags.
capture_invocation_overrides() {
  local key

  clear_invocation_overrides
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    # RELEASE_VERSION is saved deployment metadata, not an invocation input.
    # VERSION is the sole explicit release-selection override.
    if [[ "${key}" != "RELEASE_VERSION" ]]; then
      capture_config_override "${key}" "${key}"
    fi
  done

  # Keep the original command interface while storing canonical release
  # metadata. A canonical RELEASE_REPO beats the legacy REPO alias when both
  # are explicitly present.
  if [[ "${OVERRIDE_RELEASE_REPO_SET}" != "true" ]]; then
    capture_config_override RELEASE_REPO REPO
  fi

  # VERSION is a one-shot release-selection override, not persisted config.
  if [[ -v VERSION ]]; then
    OVERRIDE_VERSION="${VERSION}"
    OVERRIDE_VERSION_SET=true
  fi
}

discard_current_config_snapshot() {
  local snapshot="$1"

  if [[ -n "${snapshot}" ]]; then
    rm -f -- "${snapshot}" >/dev/null 2>&1
  fi
}

report_current_config_load_failure() {
  local snapshot="${1:-}"

  clear_current_config
  if ! discard_current_config_snapshot "${snapshot}"; then
    echo "Failed to remove temporary configuration snapshot." >&2
  fi
  # Never include an env path value or sourced output here: either can expose
  # credentials from a malformed file.
  echo "Failed to load existing configuration safely." >&2
}

# Read exactly the expected key/value NUL frames from stdin. A final read is
# deliberately required to reach EOF so source output and any trailing bytes
# cannot be silently accepted.
read_current_config_snapshot() {
  local key loaded_key value trailing=""

  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    if ! IFS= read -r -d '' loaded_key; then
      return 1
    fi
    if [[ "${loaded_key}" != "${key}" ]]; then
      return 1
    fi
    if ! IFS= read -r -d '' value; then
      return 1
    fi
    printf -v "CURRENT_${key}" '%s' "${value}"
  done

  trailing=""
  if IFS= read -r -d '' trailing; then
    return 1
  fi
  [[ -z "${trailing}" ]]
}

validate_config_directory_security() {
  local owner mode mode_number

  if [[ -L "${CONFIG_DIR}" || ! -d "${CONFIG_DIR}" ]]; then
    return 1
  fi
  owner="$(stat -c '%u' -- "${CONFIG_DIR}" 2>/dev/null)" || return 1
  mode="$(stat -c '%a' -- "${CONFIG_DIR}" 2>/dev/null)" || return 1
  if [[ "${owner}" != "${ENV_FILE_EXPECTED_UID}" || ! "${mode}" =~ ^[0-7]{3,4}$ ]]; then
    return 1
  fi
  mode_number=$((8#${mode}))
  # A non-owner must not be able to swap the checked env path before source.
  (( (mode_number & 0022) == 0 ))
}

# Read only the approved deployment fields in a child shell. In particular, an
# installed env cannot set one-shot flags or otherwise mutate this process.
# The trusted-directory, root-owned, mode-0600 env is evaluated in an isolated
# child, whose registered values are synchronously framed into a mode-0600
# temporary file before the parent accepts them.
load_current_config() {
  local key snapshot="" owner mode

  clear_current_config
  if [[ ! -e "${ENV_FILE}" && ! -L "${ENV_FILE}" ]]; then
    if [[ ( -e "${CONFIG_DIR}" || -L "${CONFIG_DIR}" ) ]] &&
       ! validate_config_directory_security; then
      report_current_config_load_failure ""
      return 1
    fi
    return 0
  fi
  if ! validate_config_directory_security; then
    report_current_config_load_failure ""
    return 1
  fi
  if [[ -L "${ENV_FILE}" ]]; then
    report_current_config_load_failure ""
    return 1
  fi
  if [[ ! -f "${ENV_FILE}" ]]; then
    report_current_config_load_failure ""
    return 1
  fi

  if ! owner="$(stat -c '%u' -- "${ENV_FILE}" 2>/dev/null)"; then
    report_current_config_load_failure ""
    return 1
  fi
  if ! mode="$(stat -c '%a' -- "${ENV_FILE}" 2>/dev/null)"; then
    report_current_config_load_failure ""
    return 1
  fi
  if [[ ! "${ENV_FILE_EXPECTED_UID}" =~ ^[0-9]+$ || "${owner}" != "${ENV_FILE_EXPECTED_UID}" || "${mode}" != "600" ]]; then
    report_current_config_load_failure ""
    return 1
  fi

  # Do not honor TMPDIR here: this root-owned snapshot contains sourced
  # credentials, so its parent directory must not be caller-controlled.
  if ! snapshot="$(mktemp "/tmp/${SERVICE_NAME}-config.XXXXXX")"; then
    report_current_config_load_failure ""
    return 1
  fi
  if ! chmod 0600 "${snapshot}"; then
    report_current_config_load_failure "${snapshot}"
    return 1
  fi

  if ! (
    # Do not let a missing field inherit an invocation value in this child.
    unset REPO VERSION ALLOW_INSECURE_DOWNLOAD_BASE_URL SKIP_CHECKSUM INSTALLER_MODE TTY
    unset "${DEPLOYMENT_CONFIG_KEYS[@]}"
    # shellcheck disable=SC1090
    if ! . "${ENV_FILE}"; then
      exit 1
    fi
    for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
      builtin printf '%s\0%s\0' "${key}" "${!key-}" || exit 1
    done
  ) > "${snapshot}" 2>/dev/null; then
    report_current_config_load_failure "${snapshot}"
    return 1
  fi

  if ! read_current_config_snapshot < "${snapshot}"; then
    report_current_config_load_failure "${snapshot}"
    return 1
  fi
  if ! discard_current_config_snapshot "${snapshot}"; then
    clear_current_config
    echo "Failed to remove temporary configuration snapshot." >&2
    return 1
  fi
}

set_cfg_from_precedence() {
  local key="$1"
  local default_value="${2-}"
  local override_name="OVERRIDE_${key}"
  local override_set_name="OVERRIDE_${key}_SET"
  local current_name="CURRENT_${key}"

  if [[ "${!override_set_name}" == "true" ]]; then
    printf -v "CFG_${key}" '%s' "${!override_name}"
  elif [[ -n "${!current_name}" ]]; then
    printf -v "CFG_${key}" '%s' "${!current_name}"
  else
    printf -v "CFG_${key}" '%s' "${default_value}"
  fi
}

set_install_release_version() {
  if [[ "${OVERRIDE_VERSION_SET}" == "true" ]]; then
    printf -v CFG_RELEASE_VERSION '%s' "${OVERRIDE_VERSION}"
    return
  fi
  CFG_RELEASE_VERSION="$(resolve_release_version "" "${CURRENT_RELEASE_VERSION}")" || return
}

require_existing_installation() {
  local mode="$1"

  if [[ -f "${ENV_FILE}" ]]; then
    return 0
  fi

  case "${mode}" in
    update)
      echo "--mode=update requires an existing deployment configuration at ${ENV_FILE}." >&2
      echo "Run --mode=install for a first installation." >&2
      ;;
    reconfigure)
      echo "--mode=reconfigure requires an existing deployment configuration at ${ENV_FILE}." >&2
      echo "Run --mode=install for a first installation." >&2
      ;;
  esac
  return 1
}

adopt_current_config() {
  local key current_name

  # VERSION selects a replacement asset, but it cannot manufacture the
  # release metadata needed to identify an existing deployment safely.
  if [[ -z "${CURRENT_RELEASE_REPO}" || -z "${CURRENT_RELEASE_VERSION}" ]]; then
    echo "--mode=update requires saved RELEASE_REPO and RELEASE_VERSION metadata." >&2
    echo "Run --mode=install to re-establish saved release metadata before retrying --mode=update." >&2
    return 1
  fi

  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    current_name="CURRENT_${key}"
    printf -v "CFG_${key}" '%s' "${!current_name}"
  done
  # update intentionally ignores every deployment runtime override except the
  # explicit VERSION selector captured from the invocation environment.
  if [[ "${OVERRIDE_VERSION_SET}" == "true" ]]; then
    printf -v CFG_RELEASE_VERSION '%s' "${OVERRIDE_VERSION}"
  fi
}

prompt() {
  local label="$1"
  local default_value="${2:-}"
  local value

  if [[ -n "${default_value}" ]]; then
    read -r -p "${label} [${default_value}]: " value < "${TTY}" || return 1
    printf "%s" "${value:-${default_value}}"
  else
    read -r -p "${label}: " value < "${TTY}" || return 1
    printf "%s" "${value}"
  fi
}

prompt_required() {
  local label="$1"
  local default_value="${2:-}"
  local value

  while true; do
    if ! value="$(prompt "${label}" "${default_value}")"; then
      return 1
    fi
    if [[ -n "${value}" ]]; then
      printf "%s" "${value}"
      return
    fi
    echo "This value is required." >&2
  done
}

prompt_secret() {
  local label="$1"
  local has_existing="$2"
  local value

  if [[ "${has_existing}" == "true" ]]; then
    read -r -s -p "${label} [keep existing]: " value < "${TTY}" || return 1
    echo >&2
    printf "%s" "${value}"
  else
    while true; do
      read -r -s -p "${label}: " value < "${TTY}" || return 1
      echo >&2
      if [[ -n "${value}" ]]; then
        printf "%s" "${value}"
        return
      fi
      echo "This value is required." >&2
    done
  fi
}

prompt_optional_secret() {
  local label="$1"
  local has_existing="$2"
  local value

  if [[ "${has_existing}" == "true" ]]; then
    read -r -s -p "${label} [keep existing]: " value < "${TTY}" || return 1
  else
    read -r -s -p "${label} (optional): " value < "${TTY}" || return 1
  fi
  echo >&2
  printf "%s" "${value}"
}

collect_config_interactive() {
  local mode="$1"
  local token_input password_input
  local has_password has_token

  case "${mode}" in
    install)
      set_install_release_version
      ;;
    reconfigure)
      if [[ -z "${CURRENT_RELEASE_VERSION}" ]]; then
        echo "Existing RELEASE_VERSION is required for --mode=reconfigure." >&2
        return 1
      fi
      # Reconfiguration changes deployment settings, never the saved release.
      CFG_RELEASE_VERSION="${CURRENT_RELEASE_VERSION}"
      ;;
    *)
      echo "Invalid interactive installer mode." >&2
      return 1
      ;;
  esac

  set_cfg_from_precedence RELEASE_REPO "${DEFAULT_REPO}"
  set_cfg_from_precedence DOMAIN ""

  echo "BUPT_EC installer"
  echo
  if ! CFG_RELEASE_REPO="$(prompt_required "GitHub repository" "${CFG_RELEASE_REPO}")"; then
    return 1
  fi
  if ! CFG_DOMAIN="$(prompt_required "Domain name" "${CFG_DOMAIN}")"; then
    return 1
  fi

  set_cfg_from_precedence SSL_CERT "/etc/letsencrypt/live/${CFG_DOMAIN}/fullchain.pem"
  if ! CFG_SSL_CERT="$(prompt_required "SSL certificate path" "${CFG_SSL_CERT}")"; then
    return 1
  fi
  set_cfg_from_precedence SSL_KEY "/etc/letsencrypt/live/${CFG_DOMAIN}/privkey.pem"
  if ! CFG_SSL_KEY="$(prompt_required "SSL private key path" "${CFG_SSL_KEY}")"; then
    return 1
  fi

  set_cfg_from_precedence JW_TOKEN ""
  has_token=false
  if [[ -n "${CFG_JW_TOKEN}" ]]; then
    has_token=true
  fi
  if ! token_input="$(prompt_optional_secret "JW token override, usually leave empty" "${has_token}")"; then
    return 1
  fi
  if [[ -n "${token_input}" ]]; then
    CFG_JW_TOKEN="${token_input}"
  fi

  set_cfg_from_precedence JW_USERNAME ""
  if [[ -n "${CFG_JW_TOKEN}" ]]; then
    if ! CFG_JW_USERNAME="$(prompt "BUPT JW username, optional when JW token is set" "${CFG_JW_USERNAME}")"; then
      return 1
    fi
  elif ! CFG_JW_USERNAME="$(prompt_required "BUPT JW username" "${CFG_JW_USERNAME}")"; then
    return 1
  fi

  set_cfg_from_precedence JW_PASSWORD ""
  has_password=false
  if [[ -n "${CFG_JW_PASSWORD}" ]]; then
    has_password=true
  fi
  if [[ -n "${CFG_JW_TOKEN}" ]]; then
    if ! password_input="$(prompt_optional_secret "BUPT JW password" "${has_password}")"; then
      return 1
    fi
  elif ! password_input="$(prompt_secret "BUPT JW password" "${has_password}")"; then
    return 1
  fi
  if [[ -n "${password_input}" ]]; then
    CFG_JW_PASSWORD="${password_input}"
  fi

  set_cfg_from_precedence APP_ADDR "${DEFAULT_APP_ADDR}"
  if ! CFG_APP_ADDR="$(prompt_required "Backend listen address" "${CFG_APP_ADDR}")"; then
    return 1
  fi
  set_cfg_from_precedence DOWNLOAD_BASE_URL ""
  # These runtime settings are supported by the service but deliberately have
  # no interactive prompts.
  set_cfg_from_precedence LOG_CALLER ""
  set_cfg_from_precedence READYZ_DIAGNOSTICS ""
}

shell_quote() {
  local -a pipe_status=()

  printf "'" || return 1
  # Capture both pipeline statuses explicitly; callers often use this function
  # in an OR/! context where errexit alone would not preserve a failed writer.
  if printf "%s" "$1" | sed "s/'/'\\\\''/g"; then
    pipe_status=("${PIPESTATUS[@]}")
  else
    pipe_status=("${PIPESTATUS[@]}")
  fi
  if (( pipe_status[0] != 0 || pipe_status[1] != 0 )); then
    return 1
  fi
  printf "'" || return 1
}

validate_repo() {
  local repo="$1"
  if [[ ! "${repo}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid GitHub repository: ${repo}" >&2
    return 1
  fi
}

resolve_release_version() {
  local explicit_version="${1:-}"
  local current_version="${2:-}"

  if [[ -n "${explicit_version}" ]]; then
    printf "%s" "${explicit_version}"
  elif [[ -n "${current_version}" ]]; then
    printf "%s" "${current_version}"
  else
    printf "latest"
  fi
}

validate_version() {
  local version="$1"
  if [[ "${version}" == "latest" || "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    return
  fi
  echo "VERSION must be latest or a stable tag such as v0.1.4: ${version}" >&2
  if [[ "${version}" == "nightly" ]]; then
    echo "The nightly channel was removed in v0.3.0; rerun with VERSION=latest." >&2
    echo "That also rewrites the saved RELEASE_VERSION in ${ENV_FILE}." >&2
  fi
  return 1
}

validate_domain() {
  local domain="$1"
  if [[ ! "${domain}" =~ ^[A-Za-z0-9.-]+$ || "${domain}" == .* || "${domain}" == *. || "${domain}" == *..* ]]; then
    echo "Invalid domain name: ${domain}" >&2
    return 1
  fi
}

validate_absolute_path() {
  local label="$1"
  local path="$2"
  if [[ "${path}" != /* ]]; then
    echo "${label} must be an absolute path: ${path}" >&2
    return 1
  fi
  if [[ "${path}" == *";"* || "${path}" =~ [[:space:]] ]]; then
    echo "${label} must not contain whitespace or semicolons: ${path}" >&2
    return 1
  fi
  case "${path}" in
    *'$'* | *'#'* | *'{'* | *'}'* | *"'"* | *'"'* | *\\*)
      echo "${label} contains characters that are unsafe in the Nginx configuration." >&2
      return 1
      ;;
  esac
}

is_valid_ipv4_literal() {
  local address="$1"
  local octet
  local -a octets=()

  if [[ ! "${address}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    return 1
  fi
  IFS=. read -r -a octets <<< "${address}"
  for octet in "${octets[@]}"; do
    if (( 10#${octet} > 255 )); then
      return 1
    fi
  done
  return 0
}

is_valid_ipv6_literal() {
  local address="$1"
  local ipv4_tail
  local left right side segment
  local compressed=false
  local count=0
  local -a segments=()

  # IPv4-mapped IPv6 remains an IPv6 literal, but count its dotted tail as
  # two hextets after independently validating the IPv4 syntax.
  if [[ "${address}" == *.* ]]; then
    ipv4_tail="${address##*:}"
    if ! is_valid_ipv4_literal "${ipv4_tail}"; then
      return 1
    fi
    address="${address%"${ipv4_tail}"}0:0"
  fi
  if [[ ! "${address}" =~ ^[0-9A-Fa-f:]+$ || "${address}" != *:* || "${address}" == *:::* ]]; then
    return 1
  fi
  if [[ "${address}" == *::* ]]; then
    compressed=true
    left="${address%%::*}"
    right="${address#*::}"
    if [[ "${right}" == *::* ]]; then
      return 1
    fi
  else
    left="${address}"
    right=""
  fi

  for side in "${left}" "${right}"; do
    if [[ -z "${side}" ]]; then
      continue
    fi
    if [[ "${side}" == :* || "${side}" == *: ]]; then
      return 1
    fi
    segments=()
    IFS=: read -r -a segments <<< "${side}"
    for segment in "${segments[@]}"; do
      if [[ ! "${segment}" =~ ^[0-9A-Fa-f]{1,4}$ ]]; then
        return 1
      fi
      count=$((count + 1))
    done
  done

  if [[ "${compressed}" == "true" ]]; then
    [[ "${count}" -lt 8 ]]
  else
    [[ "${count}" -eq 8 ]]
  fi
}

is_valid_app_hostname() {
  local host="$1"

  if [[ "${host}" == *.* && "${host}" =~ ^[0-9.]+$ ]]; then
    is_valid_ipv4_literal "${host}"
    return
  fi
  [[ "${host}" =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)*$ ]]
}

validate_app_addr() {
  local app_addr="$1"
  local host port

  if [[ "${app_addr}" =~ ^\[([0-9A-Fa-f:.]+)\]:([0-9]{1,5})$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[2]}"
    if ! is_valid_ipv6_literal "${host}"; then
      echo "Invalid backend listen address." >&2
      return 1
    fi
  elif [[ "${app_addr}" =~ ^([^:]+):([0-9]{1,5})$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[2]}"
    if ! is_valid_app_hostname "${host}"; then
      echo "Invalid backend listen address." >&2
      return 1
    fi
  else
    echo "Invalid backend listen address." >&2
    return 1
  fi

  if (( 10#${port} < 1 || 10#${port} > 65535 )); then
    echo "Backend listen port is out of range." >&2
    return 1
  fi
}

current_config_load_guidance() {
  echo "Repair the installed configuration ownership, mode, or syntax before retrying." >&2
  echo "If it cannot be repaired, move it aside and run --mode=install to rebuild it." >&2
}

version_recovery_guidance() {
  case "${INSTALLER_MODE}" in
    update)
      echo "Retry --mode=update with a valid VERSION, or omit VERSION to reuse valid saved release metadata." >&2
      ;;
    reconfigure)
      echo "Run --mode=install with a valid VERSION to repair saved release metadata." >&2
      ;;
  esac
}

config_recovery_guidance() {
  if [[ "${INSTALLER_MODE}" == "update" ]]; then
    echo "Fix the saved deployment configuration with --mode=reconfigure before retrying --mode=update." >&2
  fi
}

validate_config() {
  if [[ ( -e "${CONFIG_DIR}" || -L "${CONFIG_DIR}" ) ]] &&
     ! validate_config_directory_security; then
    echo "The installation configuration directory has unsafe ownership, mode, or symlink layout." >&2
    current_config_load_guidance
    return 1
  fi
  if [[ -z "${CFG_RELEASE_REPO}" ]]; then
    echo "RELEASE_REPO is required." >&2
    config_recovery_guidance
    return 1
  fi
  if ! validate_repo "${CFG_RELEASE_REPO}"; then
    config_recovery_guidance
    return 1
  fi
  if [[ -z "${CFG_RELEASE_VERSION}" ]]; then
    echo "RELEASE_VERSION is required." >&2
    version_recovery_guidance
    return 1
  fi
  if ! validate_version "${CFG_RELEASE_VERSION}"; then
    version_recovery_guidance
    return 1
  fi
  if [[ "${OVERRIDE_VERSION_SET}" == "true" &&
        "${CFG_RELEASE_VERSION}" != "${CURRENT_RELEASE_VERSION}" &&
        -n "${CURRENT_DOWNLOAD_BASE_URL}" ]]; then
    if [[ "${INSTALLER_MODE}" == "update" ]]; then
      echo "A saved custom DOWNLOAD_BASE_URL cannot prove it serves the explicitly selected VERSION." >&2
      echo "Run --mode=install with that VERSION and a matching trusted DOWNLOAD_BASE_URL." >&2
      return 1
    fi
    if [[ "${INSTALLER_MODE}" == "install" && "${OVERRIDE_DOWNLOAD_BASE_URL_SET}" != "true" ]]; then
      echo "Changing VERSION with a saved custom mirror requires an explicit matching DOWNLOAD_BASE_URL." >&2
      return 1
    fi
  fi
  if [[ -z "${CFG_DOMAIN}" ]]; then
    echo "DOMAIN is required." >&2
    config_recovery_guidance
    return 1
  fi
  if ! validate_domain "${CFG_DOMAIN}"; then
    config_recovery_guidance
    return 1
  fi
  if [[ -z "${CFG_SSL_CERT}" ]]; then
    echo "SSL certificate path is required." >&2
    config_recovery_guidance
    return 1
  fi
  if ! validate_absolute_path "SSL certificate path" "${CFG_SSL_CERT}"; then
    config_recovery_guidance
    return 1
  fi
  if [[ -z "${CFG_SSL_KEY}" ]]; then
    echo "SSL private key path is required." >&2
    config_recovery_guidance
    return 1
  fi
  if ! validate_absolute_path "SSL private key path" "${CFG_SSL_KEY}"; then
    config_recovery_guidance
    return 1
  fi
  if [[ -z "${CFG_APP_ADDR}" ]]; then
    echo "Backend listen address is required." >&2
    config_recovery_guidance
    return 1
  fi
  if ! validate_app_addr "${CFG_APP_ADDR}"; then
    config_recovery_guidance
    return 1
  fi
  if [[ -z "${CFG_JW_TOKEN}" && ( -z "${CFG_JW_USERNAME}" || -z "${CFG_JW_PASSWORD}" ) ]]; then
    echo "JW_TOKEN or both JW_USERNAME and JW_PASSWORD are required." >&2
    config_recovery_guidance
    return 1
  fi

  VALIDATED_DOWNLOAD_BASE_URL=""
  if ! validate_download_base_url "${CFG_DOWNLOAD_BASE_URL}"; then
    config_recovery_guidance
    return 1
  fi
  # Persist and download only the normalized form (no userinfo/query/fragment).
  CFG_DOWNLOAD_BASE_URL="${VALIDATED_DOWNLOAD_BASE_URL}"

  if [[ ! -f "${CFG_SSL_CERT}" ]]; then
    echo "SSL certificate not found: ${CFG_SSL_CERT}" >&2
    config_recovery_guidance
    return 1
  fi
  if [[ ! -f "${CFG_SSL_KEY}" ]]; then
    echo "SSL private key not found: ${CFG_SSL_KEY}" >&2
    config_recovery_guidance
    return 1
  fi
}

reset_config_state
