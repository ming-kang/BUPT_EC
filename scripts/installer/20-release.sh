# shellcheck shell=bash
# shellcheck disable=SC2034

# normalize_download_base_url validates and normalizes DOWNLOAD_BASE_URL.
# Empty input prints an empty string (official GitHub path). Invalid input exits
# non-zero with a rule-only error that never echoes the raw URL (no secrets).
# On success prints scheme://host[:port][/path] with trailing slashes stripped.
normalize_download_base_url() {
  local url="$1"
  local scheme rest authority path host port host_display normalized

  if [[ -z "${url}" ]]; then
    printf ""
    return 0
  fi

  if [[ "${url}" == *";"* || "${url}" =~ [[:space:]] ]]; then
    echo "DOWNLOAD_BASE_URL must not contain whitespace or semicolons." >&2
    return 1
  fi
  if [[ "${url}" == *"?"* ]]; then
    echo "DOWNLOAD_BASE_URL must not contain a query string." >&2
    return 1
  fi
  if [[ "${url}" == *"#"* ]]; then
    echo "DOWNLOAD_BASE_URL must not contain a URL fragment." >&2
    return 1
  fi
  if [[ "${url}" == *"@"* ]]; then
    echo "DOWNLOAD_BASE_URL must not contain userinfo credentials." >&2
    return 1
  fi

  if [[ ! "${url}" =~ ^([A-Za-z][A-Za-z0-9+.-]*)://(.*)$ ]]; then
    echo "DOWNLOAD_BASE_URL must be an absolute http(s) URL." >&2
    return 1
  fi
  scheme="$(printf '%s' "${BASH_REMATCH[1]}" | tr '[:upper:]' '[:lower:]')"
  rest="${BASH_REMATCH[2]}"

  case "${scheme}" in
    https) ;;
    http)
      if [[ "${ALLOW_INSECURE_DOWNLOAD_BASE_URL:-false}" != "true" ]]; then
        echo "DOWNLOAD_BASE_URL must use https://. Set ALLOW_INSECURE_DOWNLOAD_BASE_URL=true only for a trusted local mirror." >&2
        return 1
      fi
      ;;
    *)
      # Insecure opt-in only widens HTTPS→HTTP; never file/ftp/data/etc.
      echo "DOWNLOAD_BASE_URL scheme must be https (or http only with ALLOW_INSECURE_DOWNLOAD_BASE_URL=true)." >&2
      return 1
      ;;
  esac

  if [[ -z "${rest}" ]]; then
    echo "DOWNLOAD_BASE_URL must include a non-empty host." >&2
    return 1
  fi

  if [[ "${rest}" == *"/"* ]]; then
    authority="${rest%%/*}"
    path="/${rest#*/}"
  else
    authority="${rest}"
    path=""
  fi

  if [[ -z "${authority}" ]]; then
    echo "DOWNLOAD_BASE_URL must include a non-empty host." >&2
    return 1
  fi

  host=""
  port=""
  host_display=""
  if [[ "${authority}" == \[* ]]; then
    if [[ ! "${authority}" =~ ^\[([0-9A-Fa-f:]+)\](:([0-9]{1,5}))?$ ]]; then
      echo "DOWNLOAD_BASE_URL IPv6 host must be bracketed and may include a valid port." >&2
      return 1
    fi
    host="${BASH_REMATCH[1]}"
    port="${BASH_REMATCH[3]:-}"
    if [[ -z "${host}" ]]; then
      echo "DOWNLOAD_BASE_URL must include a non-empty host." >&2
      return 1
    fi
    if ! is_valid_ipv6_literal "${host}"; then
      echo "DOWNLOAD_BASE_URL IPv6 host is invalid." >&2
      return 1
    fi
    host_display="[${host}]"
  else
    if [[ "${authority}" == *:* ]]; then
      host="${authority%:*}"
      port="${authority##*:}"
      if [[ -z "${host}" || "${host}" == *:* ]]; then
        echo "DOWNLOAD_BASE_URL host is invalid; use brackets for IPv6 addresses." >&2
        return 1
      fi
    else
      host="${authority}"
      port=""
    fi
    if [[ -z "${host}" ]]; then
      echo "DOWNLOAD_BASE_URL must include a non-empty host." >&2
      return 1
    fi
    # Hostname / IPv4: labels, dots, hyphens only (no credentials or odd markup).
    if [[ ! "${host}" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ && ! "${host}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
      echo "DOWNLOAD_BASE_URL host is invalid." >&2
      return 1
    fi
    host_display="${host}"
  fi

  if [[ -n "${port}" ]]; then
    if [[ ! "${port}" =~ ^[0-9]+$ ]] || ((10#${port} < 1 || 10#${port} > 65535)); then
      echo "DOWNLOAD_BASE_URL port is out of range." >&2
      return 1
    fi
  fi

  if [[ -n "${path}" ]]; then
    # Strip trailing slashes; empty path after strip means authority only.
    while [[ "${path}" == */ ]]; do
      path="${path%/}"
    done
    if [[ -n "${path}" && ! "${path}" =~ ^/[A-Za-z0-9._~/-]+$ ]]; then
      echo "DOWNLOAD_BASE_URL path contains unsupported characters." >&2
      return 1
    fi
  fi

  normalized="${scheme}://${host_display}"
  if [[ -n "${port}" ]]; then
    normalized+=":${port}"
  fi
  if [[ -n "${path}" ]]; then
    normalized+="${path}"
  fi

  printf "%s" "${normalized}"
  return 0
}

# Scheme of a normalized download base (http or https). Empty → https default.
download_base_scheme() {
  local url="${1:-}"
  case "${url}" in
    http://*)
      printf 'http'
      ;;
    *)
      printf 'https'
      ;;
  esac
}

# Safe host label from a normalized base: hostname, IPv4, or bracketed IPv6.
download_base_safe_host() {
  local url="${1:-}"
  local rest authority
  if [[ -z "${url}" ]]; then
    printf ''
    return 0
  fi
  rest="${url#*://}"
  authority="${rest%%/*}"
  if [[ "${authority}" == \[* ]]; then
    if [[ "${authority}" =~ ^(\[[0-9A-Fa-f:]+\]) ]]; then
      printf '%s' "${BASH_REMATCH[1]}"
      return 0
    fi
    printf ''
    return 0
  fi
  printf '%s' "${authority%%:*}"
}

validate_download_base_url() {
  local url="$1"
  local normalized

  VALIDATED_DOWNLOAD_BASE_URL=""
  if ! normalized="$(normalize_download_base_url "${url}")"; then
    return 1
  fi
  if [[ -z "${normalized}" ]]; then
    return 0
  fi
  if [[ "$(download_base_scheme "${normalized}")" == "http" ]]; then
    echo "Warning: using non-HTTPS DOWNLOAD_BASE_URL because ALLOW_INSECURE_DOWNLOAD_BASE_URL=true." >&2
  fi
  # Expose normalized base for main to persist and download without re-parsing.
  VALIDATED_DOWNLOAD_BASE_URL="${normalized}"
}

# curl_download_proto_args prints --proto / --proto-redir tokens for the
# validated scheme (or defaults to HTTPS-only for official GitHub downloads).
curl_download_proto_args() {
  local scheme="${1:-https}"
  if [[ "${scheme}" == "http" ]]; then
    printf '%s\0' --proto '=http,https' --proto-redir '=http,https'
  else
    printf '%s\0' --proto '=https' --proto-redir '=https'
  fi
}

safe_download_base_label() {
  local base_url="${1:-}"
  local scheme host
  scheme="$(download_base_scheme "${base_url}")"
  host="$(download_base_safe_host "${base_url}")"
  if [[ -n "${host}" ]]; then
    printf 'operator-configured %s mirror host %s' "$(printf '%s' "${scheme}" | tr '[:lower:]' '[:upper:]')" "${host}"
  else
    printf 'configured mirror'
  fi
}

detect_arch() {
  local machine
  machine="$(uname -m)"
  case "${machine}" in
    x86_64 | amd64)
      printf "amd64"
      ;;
    aarch64 | arm64)
      printf "arm64"
      ;;
    *)
      echo "Unsupported CPU architecture: ${machine}" >&2
      exit 1
      ;;
  esac
}

install_packages() {
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update || return
    apt-get install -y ca-certificates curl tar nginx || return
  else
    echo "This installer currently supports apt-based systems such as Debian 12." >&2
    return 1
  fi
}

create_user() {
  if ! getent group "${APP_GROUP}" >/dev/null 2>&1; then
    groupadd --system "${APP_GROUP}" || return
  fi

  if ! id "${APP_USER}" >/dev/null 2>&1; then
    useradd --system --home "${INSTALL_DIR}" --shell /usr/sbin/nologin --gid "${APP_GROUP}" "${APP_USER}" || return
  fi
}

host_reachable() {
  local host="$1"
  curl -fsSIL --connect-timeout 5 --max-time 10 "https://${host}/" >/dev/null 2>&1
}

# Official GitHub releases are the only automatic trust boundary. Operators may
# point DOWNLOAD_BASE_URL at an explicit mirror they already trust; same-origin
# checksums then prove download integrity only, not independent publisher identity.
# override_url must already be normalized (validate_download_base_url) when set.
resolve_download_base_url() {
  local repo="$1"
  local version="$2"
  local override_url="$3"
  local normalized label

  if [[ -n "${override_url}" ]]; then
    # Always re-validate so callers cannot bypass shape checks; never log raw input.
    if ! normalized="$(normalize_download_base_url "${override_url}")"; then
      exit 1
    fi
    label="$(safe_download_base_label "${normalized}")"
    echo "Using ${label}." >&2
    echo "Warning: package and checksums.txt come from this operator-trusted source. Same-origin checksums verify integrity, not independent GitHub publisher identity." >&2
    printf "%s" "${normalized}"
    return
  fi

  if ! host_reachable "${GITHUB_HOST}"; then
    echo "GitHub (${GITHUB_HOST}) is not reachable." >&2
    echo "The installer no longer auto-selects third-party proxies." >&2
    echo "Mirror the release assets to an HTTPS location you control, then rerun with:" >&2
    echo "  DOWNLOAD_BASE_URL=https://your-mirror.example/path VERSION=<latest|vX.Y.Z>" >&2
    echo "Package and checksums.txt must both be present under that base URL." >&2
    exit 1
  fi

  if [[ "${version}" == "latest" ]]; then
    printf "https://%s/%s/releases/latest/download" "${GITHUB_HOST}" "${repo}"
  else
    printf "https://%s/%s/releases/download/%s" "${GITHUB_HOST}" "${repo}" "${version}"
  fi
}

download_release() {
  local repo="$1"
  local version="$2"
  local arch="$3"
  local work_dir="$4"
  local download_base_url="$5"
  local package_name="bupt-ec-linux-${arch}.tar.gz"
  local base_url
  local -a proto_args=()
  local scheme="https"
  local arg

  if ! base_url="$(resolve_download_base_url "${repo}" "${version}" "${download_base_url}")"; then
    return 1
  fi
  scheme="$(download_base_scheme "${base_url}")"
  while IFS= read -r -d '' arg; do
    proto_args+=("${arg}")
  done < <(curl_download_proto_args "${scheme}")

  if [[ -n "${download_base_url}" ]]; then
    echo "Downloading ${repo} ${version} (${arch}) from configured mirror..."
  else
    echo "Downloading ${repo} ${version} (${arch}) from official GitHub releases..."
  fi
  curl -fL "${proto_args[@]}" "${base_url}/${package_name}" -o "${work_dir}/${package_name}" || return

  # Checksum verification is required by default (fail-closed).
  # Break-glass only: SKIP_CHECKSUM=1 skips verification with a loud warning.
  if [[ "${SKIP_CHECKSUM:-}" == "1" ]]; then
    echo "WARNING: SKIP_CHECKSUM=1 is set; skipping package checksum verification. This is insecure." >&2
    return
  fi

  if ! curl -fsL "${proto_args[@]}" "${base_url}/checksums.txt" -o "${work_dir}/checksums.txt"; then
    echo "Failed to download checksums.txt from configured source; refusing to install without verification." >&2
    echo "Set SKIP_CHECKSUM=1 only as an explicit break-glass to skip verification." >&2
    exit 1
  fi

  if ! grep -q " ${package_name}$" "${work_dir}/checksums.txt"; then
    echo "checksums.txt has no entry for ${package_name}; refusing to install." >&2
    exit 1
  fi

  echo "Verifying package checksum..."
  (cd "${work_dir}" && grep " ${package_name}$" checksums.txt | sha256sum -c -)
}

# latest and stable releases at or above the first CLI-bearing release stage
# the independently packaged command. Legacy archives deliberately stage a
# remove action instead, so direct current-installer rollback stays consistent.
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

stage_release() {
  local archive="$1"
  local work_dir="$2"
  local staging_dir="$3"
  local extract_dir="${work_dir}/extract"
  local binary_path cli_path

  rm -rf "${extract_dir}" "${staging_dir}" || return
  mkdir -p "${extract_dir}" "${staging_dir}" || return
  chmod 0700 "${extract_dir}" "${staging_dir}" || return
  if ! tar -xzf "${archive}" -C "${extract_dir}"; then
    echo "Failed to extract release archive." >&2
    return 1
  fi

  # Keep this exact service-binary selection separate from the CLI member.
  if ! binary_path="$(find "${extract_dir}" -type f -name bupt-ec -print -quit)"; then
    echo "Failed to inspect extracted release archive." >&2
    return 1
  fi
  if [[ -z "${binary_path}" ]]; then
    echo "Release archive does not contain bupt-ec binary." >&2
    return 1
  fi
  install -m 0755 "${binary_path}" "${staging_dir}/bupt-ec" || return
  chown root:root "${staging_dir}/bupt-ec" || return

  if is_cli_bearing_release "${CFG_RELEASE_VERSION}"; then
    if ! cli_path="$(find "${extract_dir}" -type f -name bupt-ec-cli -print -quit)"; then
      echo "Failed to inspect extracted release archive." >&2
      return 1
    fi
    if [[ -z "${cli_path}" ]]; then
      echo "CLI-bearing release archive does not contain bupt-ec-cli." >&2
      return 1
    fi
    install -m 0755 "${cli_path}" "${staging_dir}/bupt-ec-cli" || return
    chown root:root "${staging_dir}/bupt-ec-cli" || return
  fi
}
