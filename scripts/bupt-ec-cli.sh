#!/usr/bin/env bash
# shellcheck shell=bash
# shellcheck disable=SC2034 # fixed paths are redirected only by the sourced-test seam
#
# Thin operations command. Deployment archives, checksums, staging, commit, and
# rollback remain the generated install.sh responsibility.
set -euo pipefail

SERVICE_NAME="bupt-ec"
INSTALL_DIR="/opt/bupt-ec"
CONFIG_DIR="/etc/bupt-ec"
PRIVATE_ENV_FILE="${CONFIG_DIR}/bupt-ec.env"
DEPLOYMENT_METADATA_FILE="${CONFIG_DIR}/deployment.meta"
CLI_FILE="/usr/local/bin/bupt-ec"
CLI_EXPECTED_UID=0
CLI_MIN_RELEASE="v0.3.0"
CLI_TTY="/dev/tty"

# This is the only CLI build-version marker. Release composition replaces its
# dev value with the same tag/main-sha value passed to Go through -ldflags.
CLI_BUILD_VERSION="dev" # BUPT_EC_CLI_BUILD_VERSION

DEPLOYMENT_CONFIG_KEYS=(
  RELEASE_REPO RELEASE_VERSION DOMAIN SSL_CERT SSL_KEY
  JW_USERNAME JW_PASSWORD JW_TOKEN APP_ADDR DOWNLOAD_BASE_URL
  LOG_CALLER READYZ_DIAGNOSTICS
)
readonly DEPLOYMENT_CONFIG_KEYS

CLI_SOURCED=false
if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "$0" ]]; then
  CLI_SOURCED=true
fi

# Tests source this file and explicitly redirect every fixed installed path.
# Production execution deliberately does not accept environment path overrides.
configure_cli_test_root() {
  local root="${1:-}"

  if [[ "${CLI_SOURCED}" != "true" ]]; then
    echo "configure_cli_test_root is only available when bupt-ec-cli.sh is sourced." >&2
    return 1
  fi
  if (( $# != 1 )) || [[ "${root}" != /* ]]; then
    echo "CLI test root must be one absolute path." >&2
    return 1
  fi

  INSTALL_DIR="${root}/opt/bupt-ec"
  CONFIG_DIR="${root}/etc/bupt-ec"
  PRIVATE_ENV_FILE="${CONFIG_DIR}/bupt-ec.env"
  DEPLOYMENT_METADATA_FILE="${CONFIG_DIR}/deployment.meta"
  CLI_FILE="${root}/usr/local/bin/bupt-ec"
  CLI_EXPECTED_UID="${EUID}"
}

cli_usage() {
  cat <<'EOF'
Usage: bupt-ec <command> [options]

Commands:
  update [VERSION]    Update without prompts (latest or vX.Y.Z >= v0.3.0)
  status              Show service state, configured selector, and probes
  version             Compare configured, running, and CLI versions
  health              Probe /healthz and /readyz
  logs [-f] [-n N]    Show the bupt-ec journal (default: 50 lines)
  start|stop|restart  Control the bupt-ec service
  config              Interactively reconfigure the saved deployment
  config show         Show saved configuration with secrets redacted
  -h, --help          Show this help
EOF
}

cli_usage_error() {
  echo "$1" >&2
  cli_usage >&2
  return 2
}

require_cli_root() {
  local command="$1"
  local argument

  if [[ "${EUID}" -ne 0 ]]; then
    printf 'bupt-ec %s requires root. Retry with: sudo bupt-ec %q' "${command}" "${command}" >&2
    shift
    for argument in "$@"; do
      printf ' %q' "${argument}" >&2
    done
    printf '\n' >&2
    return 1
  fi
}

clear_private_deployment_config() {
  local key
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    printf -v "PRIVATE_${key}" '%s' ""
  done
}

clear_public_deployment_metadata() {
  PUBLIC_RELEASE_VERSION=""
  PUBLIC_APP_ADDR=""
}

validate_config_directory_security() {
  local owner mode mode_number

  if [[ -L "${CONFIG_DIR}" || ! -d "${CONFIG_DIR}" ]]; then
    return 1
  fi
  owner="$(stat -c '%u' -- "${CONFIG_DIR}" 2>/dev/null)" || return 1
  mode="$(stat -c '%a' -- "${CONFIG_DIR}" 2>/dev/null)" || return 1
  if [[ "${owner}" != "${CLI_EXPECTED_UID}" || ! "${mode}" =~ ^[0-7]{3,4}$ ]]; then
    return 1
  fi
  mode_number=$((8#${mode}))
  (( (mode_number & 0022) == 0 ))
}

discard_cli_temporary_file() {
  local path="${1:-}"

  [[ -z "${path}" ]] && return 0
  rm -f -- "${path}" >/dev/null 2>&1
}

private_config_load_failure() {
  local snapshot="${1:-}"

  clear_private_deployment_config
  if ! discard_cli_temporary_file "${snapshot}"; then
    # The frame is mode 0600, but do not silently claim it was removed when a
    # filesystem failure leaves recovery work for root.
    echo "Failed to remove temporary configuration snapshot." >&2
  fi
  # Do not name contents, source output, or a credential-bearing path here.
  echo "Failed to load existing configuration safely." >&2
}

read_private_config_frame() {
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
    printf -v "PRIVATE_${key}" '%s' "${value}"
  done

  if IFS= read -r -d '' trailing; then
    return 1
  fi
  [[ -z "${trailing}" ]]
}

# The private env is root-controlled code, but never code for the CLI parent.
# A child frames only the approved values; source stdout/stderr and EXIT traps
# either remain in the protected frame or make its exact framing invalid.
load_private_deployment_config() {
  local key snapshot="" owner mode

  clear_private_deployment_config
  if [[ ! -e "${PRIVATE_ENV_FILE}" && ! -L "${PRIVATE_ENV_FILE}" ]]; then
    private_config_load_failure ""
    return 1
  fi
  if ! validate_config_directory_security || [[ -L "${PRIVATE_ENV_FILE}" ]] ||
     [[ ! -f "${PRIVATE_ENV_FILE}" ]]; then
    private_config_load_failure ""
    return 1
  fi
  owner="$(stat -c '%u' -- "${PRIVATE_ENV_FILE}" 2>/dev/null)" || {
    private_config_load_failure ""
    return 1
  }
  mode="$(stat -c '%a' -- "${PRIVATE_ENV_FILE}" 2>/dev/null)" || {
    private_config_load_failure ""
    return 1
  }
  if [[ "${owner}" != "${CLI_EXPECTED_UID}" || "${mode}" != "600" ]]; then
    private_config_load_failure ""
    return 1
  fi

  if ! snapshot="$(mktemp "/tmp/${SERVICE_NAME}-cli-config.XXXXXX")"; then
    private_config_load_failure ""
    return 1
  fi
  if ! chmod 0600 "${snapshot}"; then
    private_config_load_failure "${snapshot}"
    return 1
  fi

  if ! (
    # Every unset is checked explicitly. This child can inherit readonly
    # variables when the CLI is sourced for tests; accepting one would let a
    # caller value survive into the fixed frame when the env omits that key.
    unset REPO VERSION ALLOW_INSECURE_DOWNLOAD_BASE_URL SKIP_CHECKSUM || exit 1
    unset INSTALLER_MODE CLI_SOURCED CLI_BUILD_VERSION CLI_TTY \
      CLI_VALIDATED_DOWNLOAD_BASE_URL || exit 1
    for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
      unset "${key}" "PRIVATE_${key}" || exit 1
    done
    # shellcheck disable=SC1090
    if ! . "${PRIVATE_ENV_FILE}"; then
      exit 1
    fi
    for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
      builtin printf '%s\0%s\0' "${key}" "${!key-}" || exit 1
    done
  ) > "${snapshot}" 2>/dev/null; then
    private_config_load_failure "${snapshot}"
    return 1
  fi

  if ! read_private_config_frame < "${snapshot}"; then
    private_config_load_failure "${snapshot}"
    return 1
  fi
  if ! discard_cli_temporary_file "${snapshot}"; then
    clear_private_deployment_config
    echo "Failed to remove temporary configuration snapshot." >&2
    echo "Failed to load existing configuration safely." >&2
    return 1
  fi
}

is_valid_release_version() {
  [[ "$1" == "latest" || "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

# latest always carries the current CLI. Stable versions at or above v0.3.0 do
# too; the direct current installer handles the legacy remove action below it.
is_cli_bearing_release() {
  local version="$1"
  local major minor patch floor_major floor_minor floor_patch

  if [[ "${version}" == "latest" ]]; then
    return 0
  fi
  if [[ ! "${version}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    return 1
  fi
  major=$((10#${BASH_REMATCH[1]}))
  minor=$((10#${BASH_REMATCH[2]}))
  patch=$((10#${BASH_REMATCH[3]}))
  if [[ ! "${CLI_MIN_RELEASE}" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    return 1
  fi
  floor_major=$((10#${BASH_REMATCH[1]}))
  floor_minor=$((10#${BASH_REMATCH[2]}))
  floor_patch=$((10#${BASH_REMATCH[3]}))
  if (( major != floor_major )); then
    (( major > floor_major ))
    return
  fi
  if (( minor != floor_minor )); then
    (( minor > floor_minor ))
    return
  fi
  (( patch >= floor_patch ))
}

validate_release_repo() {
  [[ "$1" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]
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
}

is_valid_ipv6_literal() {
  local address="$1"
  local ipv4_tail left right side segment
  local compressed=false
  local count=0
  local -a segments=()

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
    [[ "${right}" != *::* ]] || return 1
  else
    left="${address}"
    right=""
  fi

  for side in "${left}" "${right}"; do
    [[ -z "${side}" ]] && continue
    [[ "${side}" != :* && "${side}" != *: ]] || return 1
    segments=()
    IFS=: read -r -a segments <<< "${side}"
    for segment in "${segments[@]}"; do
      [[ "${segment}" =~ ^[0-9A-Fa-f]{1,4}$ ]] || return 1
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
    is_valid_ipv6_literal "${host}" || return 1
  elif [[ "${app_addr}" =~ ^([^:]+):([0-9]{1,5})$ ]]; then
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[2]}"
    is_valid_app_hostname "${host}" || return 1
  else
    return 1
  fi
  (( 10#${port} >= 1 && 10#${port} <= 65535 ))
}

public_metadata_load_failure() {
  clear_public_deployment_metadata
  echo "Failed to load public deployment metadata safely." >&2
}

read_public_metadata_records() {
  local first="" second="" extra=""

  IFS= read -r first || return 1
  IFS= read -r second || return 1
  if IFS= read -r extra; then
    return 1
  fi
  [[ -z "${extra}" ]] || return 1
  [[ "${first}" == RELEASE_VERSION=* ]] || return 1
  [[ "${second}" == APP_ADDR=* ]] || return 1

  PUBLIC_RELEASE_VERSION="${first#RELEASE_VERSION=}"
  PUBLIC_APP_ADDR="${second#APP_ADDR=}"
  if ! is_valid_release_version "${PUBLIC_RELEASE_VERSION}" ||
     ! validate_app_addr "${PUBLIC_APP_ADDR}"; then
    return 1
  fi

  # Bash read drops NUL bytes. Compare the original bytes with the only valid
  # rendering so a binary/trailing-byte metadata file cannot masquerade as the
  # exact two-line public contract.
  cmp -s -- "${DEPLOYMENT_METADATA_FILE}" <(
    printf 'RELEASE_VERSION=%s\nAPP_ADDR=%s\n' \
      "${PUBLIC_RELEASE_VERSION}" "${PUBLIC_APP_ADDR}"
  ) || return 1
}

# Metadata is deliberately parsed, never sourced. It is the only configuration
# source for non-root status/version/health commands.
load_public_deployment_metadata() {
  local owner mode

  clear_public_deployment_metadata
  if [[ ! -e "${DEPLOYMENT_METADATA_FILE}" && ! -L "${DEPLOYMENT_METADATA_FILE}" ]]; then
    public_metadata_load_failure
    return 1
  fi
  if ! validate_config_directory_security || [[ -L "${DEPLOYMENT_METADATA_FILE}" ]] ||
     [[ ! -f "${DEPLOYMENT_METADATA_FILE}" ]]; then
    public_metadata_load_failure
    return 1
  fi
  owner="$(stat -c '%u' -- "${DEPLOYMENT_METADATA_FILE}" 2>/dev/null)" || {
    public_metadata_load_failure
    return 1
  }
  mode="$(stat -c '%a' -- "${DEPLOYMENT_METADATA_FILE}" 2>/dev/null)" || {
    public_metadata_load_failure
    return 1
  }
  if [[ "${owner}" != "${CLI_EXPECTED_UID}" || "${mode}" != "644" ]]; then
    public_metadata_load_failure
    return 1
  fi
  if ! read_public_metadata_records < "${DEPLOYMENT_METADATA_FILE}"; then
    public_metadata_load_failure
    return 1
  fi
}

show_config() {
  local key value_name

  load_private_deployment_config || return
  for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
    case "${key}" in
      JW_PASSWORD | JW_TOKEN)
        printf '%s=***\n' "${key}"
        ;;
      *)
        value_name="PRIVATE_${key}"
        printf '%s=%q\n' "${key}" "${!value_name}"
        ;;
    esac
  done
}

is_two_xx() {
  [[ "$1" =~ ^2[0-9][0-9]$ ]]
}

extract_readyz_version() {
  local body_file="$1"
  local http_code="$2"
  local body expected_status

  case "${http_code}" in
    200) expected_status="OK" ;;
    503) expected_status="Service Unavailable" ;;
    *) return 1 ;;
  esac

  body="$(< "${body_file}")"
  if [[ "${body}" =~ ^\{.*\}$ &&
        "${body}" =~ \"status\"[[:space:]]*:[[:space:]]*\"${expected_status}\" &&
        "${body}" =~ \"version\"[[:space:]]*:[[:space:]]*\"([A-Za-z0-9._-]+)\" ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

PROBE_HTTP_CODE="unavailable"
PROBE_READY_VERSION=""

# A probe deliberately does not use -L. curl returns success for HTTP 503, so
# callers can show readiness separately from transport failure.
probe_endpoint() {
  local endpoint="$1"
  local parse_version="$2"
  local body="" code

  PROBE_HTTP_CODE="unreachable"
  PROBE_READY_VERSION=""
  if ! body="$(mktemp "/tmp/${SERVICE_NAME}-cli-probe.XXXXXX")"; then
    PROBE_HTTP_CODE="unavailable"
    return 1
  fi
  if ! chmod 0600 "${body}"; then
    # The probe is already unusable; still make a checked cleanup attempt.
    if ! discard_cli_temporary_file "${body}"; then
      PROBE_READY_VERSION=""
    fi
    PROBE_HTTP_CODE="unavailable"
    return 1
  fi

  if ! code="$(curl --disable --silent --show-error --noproxy '*' --connect-timeout 2 --max-time 5 \
    --max-filesize 65536 --max-redirs 0 --proto '=http' --proto-redir '=http' \
    --output "${body}" --write-out '%{http_code}' "http://${PUBLIC_APP_ADDR}${endpoint}" 2>/dev/null)"; then
    if ! discard_cli_temporary_file "${body}"; then
      PROBE_HTTP_CODE="unavailable"
    fi
    return 1
  fi
  if [[ ! "${code}" =~ ^[0-9]{3}$ ]]; then
    if ! discard_cli_temporary_file "${body}"; then
      PROBE_HTTP_CODE="unavailable"
    fi
    PROBE_HTTP_CODE="unavailable"
    return 1
  fi

  PROBE_HTTP_CODE="${code}"
  if [[ "${parse_version}" == "true" ]]; then
    PROBE_READY_VERSION="$(extract_readyz_version "${body}" "${code}" 2>/dev/null || true)"
  fi
  if ! discard_cli_temporary_file "${body}"; then
    PROBE_HTTP_CODE="unavailable"
    PROBE_READY_VERSION=""
    return 1
  fi
}

HEALTH_PROBE_CODE="unavailable"
READY_PROBE_CODE="unavailable"
READY_PROBE_VERSION=""

probe_pair() {
  probe_endpoint "/healthz" false || true
  HEALTH_PROBE_CODE="${PROBE_HTTP_CODE}"
  probe_endpoint "/readyz" true || true
  READY_PROBE_CODE="${PROBE_HTTP_CODE}"
  READY_PROBE_VERSION="${PROBE_READY_VERSION}"
}

print_probe_result() {
  local label="$1"
  local code="$2"

  if is_two_xx "${code}"; then
    printf '%s: HTTP %s (ok)\n' "${label}" "${code}"
  elif [[ "${code}" == "503" ]]; then
    printf '%s: HTTP 503 (degraded/not ready)\n' "${label}"
  elif [[ "${code}" =~ ^[0-9]{3}$ ]]; then
    printf '%s: HTTP %s (failed)\n' "${label}" "${code}"
  else
    printf '%s: %s\n' "${label}" "${code}"
  fi
}

show_health() {
  load_public_deployment_metadata || return
  probe_pair
  print_probe_result "/healthz" "${HEALTH_PROBE_CODE}"
  print_probe_result "/readyz" "${READY_PROBE_CODE}"
  is_two_xx "${HEALTH_PROBE_CODE}" && is_two_xx "${READY_PROBE_CODE}"
}

show_status() {
  local active="inactive" enabled="disabled" running="unavailable"

  load_public_deployment_metadata || return
  if systemctl is-active --quiet "${SERVICE_NAME}" >/dev/null 2>&1; then
    active="active"
  fi
  if systemctl is-enabled --quiet "${SERVICE_NAME}" >/dev/null 2>&1; then
    enabled="enabled"
  fi
  probe_pair
  if [[ -n "${READY_PROBE_VERSION}" ]]; then
    running="${READY_PROBE_VERSION}"
  fi

  printf 'Service: %s (%s)\n' "${active}" "${enabled}"
  printf 'Configured selector: %s\n' "${PUBLIC_RELEASE_VERSION}"
  printf 'Running version: %s\n' "${running}"
  print_probe_result "/healthz" "${HEALTH_PROBE_CODE}"
  print_probe_result "/readyz" "${READY_PROBE_CODE}"

  [[ "${active}" == "active" ]] && is_two_xx "${HEALTH_PROBE_CODE}" &&
    is_two_xx "${READY_PROBE_CODE}"
}

show_version() {
  local running="unavailable"

  load_public_deployment_metadata || return
  probe_endpoint "/readyz" true || true
  if [[ ( "${PROBE_HTTP_CODE}" == "200" || "${PROBE_HTTP_CODE}" == "503" ) &&
        -n "${PROBE_READY_VERSION}" ]]; then
    running="${PROBE_READY_VERSION}"
  fi

  printf 'Configured selector: %s\n' "${PUBLIC_RELEASE_VERSION}"
  printf 'Running version: %s\n' "${running}"
  printf 'CLI version: %s\n' "${CLI_BUILD_VERSION}"
  [[ "${running}" != "unavailable" ]]
}

LOG_FOLLOW=false
LOG_LINES=50

parse_logs_args() {
  local seen_follow=false seen_lines=false

  LOG_FOLLOW=false
  LOG_LINES=50
  while (( $# > 0 )); do
    case "$1" in
      -f)
        if [[ "${seen_follow}" == "true" ]]; then
          cli_usage_error "logs accepts -f at most once."
          return 2
        fi
        seen_follow=true
        LOG_FOLLOW=true
        ;;
      -n)
        if [[ "${seen_lines}" == "true" ]] || (( $# < 2 )) ||
           [[ ! "${2}" =~ ^[1-9][0-9]*$ ]]; then
          cli_usage_error "logs -n requires one positive integer."
          return 2
        fi
        seen_lines=true
        LOG_LINES="$2"
        shift
        ;;
      *)
        cli_usage_error "Unknown logs option."
        return 2
        ;;
    esac
    shift
  done
}

show_logs() {
  parse_logs_args "$@" || return $?
  if [[ "${LOG_FOLLOW}" == "true" ]]; then
    journalctl -u "${SERVICE_NAME}" -n "${LOG_LINES}" -f
  else
    journalctl -u "${SERVICE_NAME}" -n "${LOG_LINES}"
  fi
}

control_service() {
  local action="$1"

  systemctl "${action}" "${SERVICE_NAME}" || return
  if systemctl is-active --quiet "${SERVICE_NAME}" >/dev/null 2>&1; then
    printf 'Service after %s: active\n' "${action}"
  else
    printf 'Service after %s: inactive\n' "${action}"
  fi
}

normalize_cli_download_base_url() {
  local url="$1"
  local scheme rest authority path host port host_display normalized

  if [[ -z "${url}" ]]; then
    printf ''
    return 0
  fi
  if [[ "${url}" == *";"* || "${url}" =~ [[:space:]] || "${url}" == *"?"* ||
        "${url}" == *"#"* || "${url}" == *"@"* ]]; then
    return 1
  fi
  if [[ ! "${url}" =~ ^([A-Za-z][A-Za-z0-9+.-]*)://(.*)$ ]]; then
    return 1
  fi
  scheme="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:upper:]' '[:lower:]')"
  rest="${BASH_REMATCH[2]}"
  case "${scheme}" in
    https) ;;
    http)
      [[ "${ALLOW_INSECURE_DOWNLOAD_BASE_URL:-}" == "true" ]] || return 1
      ;;
    *) return 1 ;;
  esac
  [[ -n "${rest}" ]] || return 1

  if [[ "${rest}" == */* ]]; then
    authority="${rest%%/*}"
    path="/${rest#*/}"
  else
    authority="${rest}"
    path=""
  fi
  [[ -n "${authority}" ]] || return 1

  host=""
  port=""
  host_display=""
  if [[ "${authority}" == \[* ]]; then
    [[ "${authority}" =~ ^\[([0-9A-Fa-f:]+)\](:([0-9]{1,5}))?$ ]] || return 1
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[3]:-}"
    [[ -n "${host}" ]] || return 1
    is_valid_ipv6_literal "${host}" || return 1
    host_display="[${host}]"
  else
    if [[ "${authority}" == *:* ]]; then
      host="${authority%:*}"
      port="${authority##*:}"
      [[ -n "${host}" && "${host}" != *:* ]] || return 1
    else
      host="${authority}"
    fi
    [[ -n "${host}" ]] || return 1
    if [[ ! "${host}" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ &&
          ! "${host}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
      return 1
    fi
    host_display="${host}"
  fi
  if [[ -n "${port}" ]]; then
    [[ "${port}" =~ ^[0-9]+$ ]] || return 1
    (( 10#${port} >= 1 && 10#${port} <= 65535 )) || return 1
  fi
  while [[ "${path}" == */ ]]; do
    path="${path%/}"
  done
  [[ -z "${path}" || "${path}" =~ ^/[A-Za-z0-9._~/-]+$ ]] || return 1

  normalized="${scheme}://${host_display}"
  [[ -z "${port}" ]] || normalized+=":${port}"
  [[ -z "${path}" ]] || normalized+="${path}"
  printf '%s' "${normalized}"
}

validate_private_bootstrap_config() {
  if ! validate_release_repo "${PRIVATE_RELEASE_REPO}" ||
     ! is_valid_release_version "${PRIVATE_RELEASE_VERSION}"; then
    echo "Saved deployment release metadata is invalid." >&2
    return 1
  fi
  if ! CLI_VALIDATED_DOWNLOAD_BASE_URL="$(normalize_cli_download_base_url "${PRIVATE_DOWNLOAD_BASE_URL}")"; then
    echo "Saved download mirror is invalid or requires HTTPS." >&2
    return 1
  fi
}

validate_current_installer_asset() {
  local installer="$1"
  local first_line="" second_line=""

  if [[ -L "${installer}" || ! -f "${installer}" || ! -s "${installer}" ]]; then
    return 1
  fi
  {
    IFS= read -r first_line || return 1
    IFS= read -r second_line || return 1
  } < "${installer}" || return 1
  [[ "${first_line}" == "#!/usr/bin/env bash" &&
     "${second_line}" == "# Code generated by scripts/generate-install.sh; DO NOT EDIT." ]]
}

fetch_current_installer() {
  local destination="$1"
  local url scheme
  local -a proto_args=(--proto '=https' --proto-redir '=https')

  validate_private_bootstrap_config || return
  if [[ -n "${CLI_VALIDATED_DOWNLOAD_BASE_URL}" ]]; then
    url="${CLI_VALIDATED_DOWNLOAD_BASE_URL}/install.sh"
    scheme="${CLI_VALIDATED_DOWNLOAD_BASE_URL%%://*}"
    if [[ "${scheme}" == "http" ]]; then
      proto_args=(--proto '=http,https' --proto-redir '=http,https')
    fi
  else
    url="https://github.com/${PRIVATE_RELEASE_REPO}/releases/latest/download/install.sh"
  fi

  if ! curl --disable -fsSL --connect-timeout 10 --max-time 120 "${proto_args[@]}" \
    --output "${destination}" "${url}" >/dev/null 2>&1; then
    echo "Failed to download the current Installer." >&2
    return 1
  fi
  if ! validate_current_installer_asset "${destination}"; then
    echo "Failed to download the current Installer." >&2
    return 1
  fi
  chmod 0700 "${destination}" || return 1
}

cleanup_current_installer_session() {
  local status="$1"
  local session="$2"

  trap - EXIT
  if ! rm -rf -- "${session}" >/dev/null 2>&1; then
    echo "Failed to remove secure Installer session." >&2
    if (( status == 0 )); then
      status=1
    fi
  fi
  exit "${status}"
}

run_current_installer() {
  local mode="$1"
  local target_version="${2:-}"
  local session installer

  if ! session="$(mktemp -d "/tmp/${SERVICE_NAME}-cli.XXXXXX")"; then
    echo "Failed to create a secure Installer session." >&2
    return 1
  fi
  if ! chmod 0700 "${session}"; then
    if ! rm -rf -- "${session}" >/dev/null 2>&1; then
      echo "Failed to remove secure Installer session." >&2
    fi
    echo "Failed to create a secure Installer session." >&2
    return 1
  fi
  installer="${session}/install.sh"

  (
    trap 'cleanup_current_installer_session "$?" "${session}"' EXIT
    fetch_current_installer "${installer}" || exit 1
    case "${mode}" in
      update)
        VERSION="${target_version}" bash "${installer}" --mode=update
        ;;
      reconfigure)
        # Reconfigure preserves the saved selector; do not leak a caller's
        # one-shot update selector into the bootstrapped Installer process.
        unset VERSION
        bash "${installer}" --mode=reconfigure
        ;;
      *)
        exit 2
        ;;
    esac
  )
}

print_legacy_installer_fallback() {
  local target_version="$1"

  echo "bupt-ec update supports latest or stable releases ${CLI_MIN_RELEASE} and newer." >&2
  echo "For a legacy target, use the current/latest Installer fallback: fetch install.sh from the official source or configured mirror, then run:" >&2
  echo "  sudo VERSION=${target_version} bash install.sh --mode=update" >&2
}

run_update() {
  local requested_version="${1:-}"
  local target_version

  # An explicit legacy target needs no private snapshot to explain the
  # compatibility boundary. Reject it before any file/bootstrap work as well
  # as before curl, so a broken old installation cannot hide the fallback.
  if [[ -n "${requested_version}" ]]; then
    if ! is_valid_release_version "${requested_version}"; then
      echo "Saved release selector is invalid." >&2
      return 1
    fi
    if ! is_cli_bearing_release "${requested_version}"; then
      print_legacy_installer_fallback "${requested_version}"
      return 1
    fi
  fi

  load_private_deployment_config || return
  target_version="${requested_version}"
  if [[ -z "${target_version}" ]]; then
    target_version="${PRIVATE_RELEASE_VERSION}"
  fi
  if ! is_valid_release_version "${target_version}"; then
    echo "Saved release selector is invalid." >&2
    return 1
  fi
  # A saved legacy selector still needs the fixed current Installer fallback,
  # but never reaches bootstrap URL validation or curl.
  if ! is_cli_bearing_release "${target_version}"; then
    print_legacy_installer_fallback "${target_version}"
    return 1
  fi
  validate_private_bootstrap_config || return
  run_current_installer update "${target_version}"
}

require_cli_tty() {
  if [[ ! -r "${CLI_TTY}" ]] || ! : < "${CLI_TTY}" 2>/dev/null; then
    echo "Interactive configuration requires a TTY." >&2
    return 1
  fi
}

run_reconfigure() {
  # This is an interaction gate, not a bootstrap detail: do not even load the
  # private snapshot when there is no usable terminal for reconfiguration.
  require_cli_tty || return
  load_private_deployment_config || return
  validate_private_bootstrap_config || return
  run_current_installer reconfigure
}

cli_main() {
  local command="${1:-}"
  local target_version=""

  if (( $# == 0 )); then
    cli_usage
    return 0
  fi
  case "${command}" in
    -h | --help)
      if (( $# == 1 )); then
        cli_usage
        return 0
      fi
      cli_usage_error "Help does not accept additional arguments."
      return 2
      ;;
    update)
      shift
      if (( $# > 1 )); then
        cli_usage_error "update accepts at most one VERSION."
        return 2
      fi
      if (( $# == 1 )) && [[ -z "${1}" ]]; then
        cli_usage_error "update VERSION must be latest or vX.Y.Z."
        return 2
      fi
      target_version="${1:-}"
      if [[ -n "${target_version}" ]] && ! is_valid_release_version "${target_version}"; then
        cli_usage_error "update VERSION must be latest or vX.Y.Z."
        return 2
      fi
      if [[ -n "${target_version}" ]]; then
        require_cli_root update "${target_version}" || return
      else
        require_cli_root update || return
      fi
      run_update "${target_version}"
      ;;
    status | version | health)
      if (( $# != 1 )); then
        cli_usage_error "${command} does not accept arguments."
        return 2
      fi
      case "${command}" in
        status) show_status ;;
        version) show_version ;;
        health) show_health ;;
      esac
      ;;
    logs)
      shift
      show_logs "$@"
      ;;
    start | stop | restart)
      if (( $# != 1 )); then
        cli_usage_error "${command} does not accept arguments."
        return 2
      fi
      require_cli_root "${command}" || return
      control_service "${command}"
      ;;
    config)
      shift
      if (( $# == 0 )); then
        require_cli_root config || return
        run_reconfigure
      elif (( $# == 1 )) && [[ "$1" == "show" ]]; then
        require_cli_root config show || return
        show_config
      else
        cli_usage_error "config accepts no argument or the single subcommand show."
        return 2
      fi
      ;;
    *)
      cli_usage_error "Unknown command."
      return 2
      ;;
  esac
}

if [[ "${CLI_SOURCED}" != "true" ]]; then
  cli_main "$@"
fi
