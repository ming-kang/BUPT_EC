#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="bupt-ec"
DEFAULT_REPO="ming-kang/BUPT_EC"
GITHUB_HOST="github.com"
INSTALL_DIR="/opt/bupt-ec"
CONFIG_DIR="/etc/bupt-ec"
ENV_FILE="${CONFIG_DIR}/bupt-ec.env"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
SYSTEMD_ENABLED_LINK="/etc/systemd/system/multi-user.target.wants/${SERVICE_NAME}.service"
NGINX_SITE="/etc/nginx/sites-available/${SERVICE_NAME}.conf"
NGINX_ENABLED="/etc/nginx/sites-enabled/${SERVICE_NAME}.conf"
APP_USER="bupt-ec"
APP_GROUP="bupt-ec"
DEFAULT_APP_ADDR="127.0.0.1:8080"
DEFAULT_GIN_MODE="release"
TTY="/dev/tty"

CURRENT_RELEASE_REPO=""
CURRENT_RELEASE_VERSION=""
CURRENT_DOMAIN=""
CURRENT_SSL_CERT=""
CURRENT_SSL_KEY=""
CURRENT_JW_USERNAME=""
CURRENT_JW_PASSWORD=""
CURRENT_JW_TOKEN=""
CURRENT_APP_ADDR=""
CURRENT_GIN_MODE=""
CURRENT_DOWNLOAD_BASE_URL=""

INSTALLER_TMP_DIR=""
TRANSACTION_ACTIVE=false
TRANSACTION_BACKUP_DIR=""

# Tests source this script and call this explicit helper. Production main never
# reads a path override from the environment, so normal installer execution
# always targets the fixed /opt and /etc locations above.
configure_installer_test_root() {
  local root="$1"
  if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    echo "configure_installer_test_root is only available when install.sh is sourced." >&2
    return 1
  fi
  if [[ "${root}" != /* ]]; then
    echo "Installer test root must be absolute: ${root}" >&2
    return 1
  fi

  INSTALL_DIR="${root}/opt/bupt-ec"
  CONFIG_DIR="${root}/etc/bupt-ec"
  ENV_FILE="${CONFIG_DIR}/bupt-ec.env"
  SERVICE_FILE="${root}/etc/systemd/system/${SERVICE_NAME}.service"
  SYSTEMD_ENABLED_LINK="${root}/etc/systemd/system/multi-user.target.wants/${SERVICE_NAME}.service"
  NGINX_SITE="${root}/etc/nginx/sites-available/${SERVICE_NAME}.conf"
  NGINX_ENABLED="${root}/etc/nginx/sites-enabled/${SERVICE_NAME}.conf"
}

require_installer_environment() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "This installer must run as root. Use: curl -fsSL <url> | sudo bash" >&2
    exit 1
  fi

  if [[ ! -r "${TTY}" ]]; then
    echo "Interactive input requires a TTY." >&2
    exit 1
  fi
}

load_current_config() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    return
  fi

  set -a
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
  set +a
  CURRENT_RELEASE_REPO="${RELEASE_REPO:-}"
  CURRENT_RELEASE_VERSION="${RELEASE_VERSION:-}"
  CURRENT_DOMAIN="${DOMAIN:-}"
  CURRENT_SSL_CERT="${SSL_CERT:-}"
  CURRENT_SSL_KEY="${SSL_KEY:-}"
  CURRENT_JW_USERNAME="${JW_USERNAME:-}"
  CURRENT_JW_PASSWORD="${JW_PASSWORD:-}"
  CURRENT_JW_TOKEN="${JW_TOKEN:-}"
  CURRENT_APP_ADDR="${APP_ADDR:-}"
  CURRENT_GIN_MODE="${GIN_MODE:-}"
  CURRENT_DOWNLOAD_BASE_URL="${DOWNLOAD_BASE_URL:-}"
}

prompt() {
  local label="$1"
  local default_value="${2:-}"
  local value

  if [[ -n "${default_value}" ]]; then
    read -r -p "${label} [${default_value}]: " value < "${TTY}"
    printf "%s" "${value:-${default_value}}"
  else
    read -r -p "${label}: " value < "${TTY}"
    printf "%s" "${value}"
  fi
}

prompt_required() {
  local label="$1"
  local default_value="${2:-}"
  local value

  while true; do
    value="$(prompt "${label}" "${default_value}")"
    if [[ -n "${value}" ]]; then
      printf "%s" "${value}"
      return
    fi
    echo "This value is required."
  done
}

prompt_secret() {
  local label="$1"
  local has_existing="$2"
  local value

  if [[ "${has_existing}" == "true" ]]; then
    read -r -s -p "${label} [keep existing]: " value < "${TTY}"
    echo
    printf "%s" "${value}"
  else
    while true; do
      read -r -s -p "${label}: " value < "${TTY}"
      echo
      if [[ -n "${value}" ]]; then
        printf "%s" "${value}"
        return
      fi
      echo "This value is required."
    done
  fi
}

prompt_optional_secret() {
  local label="$1"
  local has_existing="$2"
  local value

  if [[ "${has_existing}" == "true" ]]; then
    read -r -s -p "${label} [keep existing]: " value < "${TTY}"
  else
    read -r -s -p "${label} (optional): " value < "${TTY}"
  fi
  echo
  printf "%s" "${value}"
}

shell_quote() {
  printf "'"
  printf "%s" "$1" | sed "s/'/'\\\\''/g"
  printf "'"
}

validate_repo() {
  local repo="$1"
  if [[ ! "${repo}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    echo "Invalid GitHub repository: ${repo}" >&2
    exit 1
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
    printf "nightly"
  fi
}

validate_version() {
  local version="$1"
  if [[ "${version}" == "latest" || "${version}" == "nightly" || "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    return
  fi
  echo "VERSION must be latest, nightly, or a stable tag such as v0.1.4: ${version}" >&2
  return 1
}

validate_domain() {
  local domain="$1"
  if [[ ! "${domain}" =~ ^[A-Za-z0-9.-]+$ || "${domain}" == .* || "${domain}" == *. || "${domain}" == *..* ]]; then
    echo "Invalid domain name: ${domain}" >&2
    exit 1
  fi
}

validate_absolute_path() {
  local label="$1"
  local path="$2"
  if [[ "${path}" != /* ]]; then
    echo "${label} must be an absolute path: ${path}" >&2
    exit 1
  fi
  if [[ "${path}" == *";"* || "${path}" =~ [[:space:]] ]]; then
    echo "${label} must not contain whitespace or semicolons: ${path}" >&2
    exit 1
  fi
}

validate_gin_mode() {
  local mode="$1"
  if [[ "${mode}" != "release" && "${mode}" != "debug" && "${mode}" != "test" ]]; then
    echo "GIN_MODE must be release, debug, or test: ${mode}" >&2
    exit 1
  fi
}

validate_app_addr() {
  local app_addr="$1"
  local port

  if [[ "${app_addr}" == :* || "${app_addr}" == *"/"* || "${app_addr}" == *";"* || "${app_addr}" =~ [[:space:]] || ! "${app_addr}" =~ :[0-9]{1,5}$ ]]; then
    echo "Invalid backend listen address: ${app_addr}" >&2
    exit 1
  fi

  port="${app_addr##*:}"
  if (( port < 1 || port > 65535 )); then
    echo "Backend listen port is out of range: ${port}" >&2
    exit 1
  fi
}

validate_download_base_url() {
  local url="$1"
  if [[ "${url}" == *";"* || "${url}" =~ [[:space:]] ]]; then
    echo "DOWNLOAD_BASE_URL must not contain whitespace or semicolons: ${url}" >&2
    exit 1
  fi
  if [[ -z "${url}" || "${url}" =~ ^https:// ]]; then
    return
  fi
  if [[ "${ALLOW_INSECURE_DOWNLOAD_BASE_URL:-false}" == "true" ]]; then
    echo "Warning: using non-HTTPS DOWNLOAD_BASE_URL because ALLOW_INSECURE_DOWNLOAD_BASE_URL=true." >&2
    return
  fi
  echo "DOWNLOAD_BASE_URL must use https://. Set ALLOW_INSECURE_DOWNLOAD_BASE_URL=true only for a trusted local mirror." >&2
  exit 1
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
    apt-get update
    apt-get install -y ca-certificates curl tar nginx
  else
    echo "This installer currently supports apt-based systems such as Debian 12." >&2
    exit 1
  fi
}

create_user() {
  if ! getent group "${APP_GROUP}" >/dev/null 2>&1; then
    groupadd --system "${APP_GROUP}"
  fi

  if ! id "${APP_USER}" >/dev/null 2>&1; then
    useradd --system --home "${INSTALL_DIR}" --shell /usr/sbin/nologin --gid "${APP_GROUP}" "${APP_USER}"
  fi
}

host_reachable() {
  local host="$1"
  curl -fsSIL --connect-timeout 5 --max-time 10 "https://${host}/" >/dev/null 2>&1
}

# Official GitHub releases are the only automatic trust boundary. Operators may
# point DOWNLOAD_BASE_URL at an explicit mirror they already trust; same-origin
# checksums then prove download integrity only, not independent publisher identity.
resolve_download_base_url() {
  local repo="$1"
  local version="$2"
  local override_url="$3"

  if [[ -n "${override_url}" ]]; then
    echo "Using explicit DOWNLOAD_BASE_URL mirror: ${override_url%/}" >&2
    echo "Warning: package and checksums.txt come from this operator-trusted source. Same-origin checksums verify integrity, not independent GitHub publisher identity." >&2
    printf "%s" "${override_url%/}"
    return
  fi

  if ! host_reachable "${GITHUB_HOST}"; then
    echo "GitHub (${GITHUB_HOST}) is not reachable." >&2
    echo "The installer no longer auto-selects third-party proxies." >&2
    echo "Mirror the release assets to an HTTPS location you control, then rerun with:" >&2
    echo "  DOWNLOAD_BASE_URL=https://your-mirror.example/path VERSION=<latest|nightly|vX.Y.Z>" >&2
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

  base_url="$(resolve_download_base_url "${repo}" "${version}" "${download_base_url}")"

  echo "Downloading ${repo} ${version} (${arch}) from ${base_url}..."
  curl -fL "${base_url}/${package_name}" -o "${work_dir}/${package_name}"

  # Checksum verification is required by default (fail-closed).
  # Break-glass only: SKIP_CHECKSUM=1 skips verification with a loud warning.
  if [[ "${SKIP_CHECKSUM:-}" == "1" ]]; then
    echo "WARNING: SKIP_CHECKSUM=1 is set; skipping package checksum verification. This is insecure." >&2
    return
  fi

  if ! curl -fsL "${base_url}/checksums.txt" -o "${work_dir}/checksums.txt"; then
    echo "Failed to download checksums.txt from ${base_url}/checksums.txt; refusing to install without verification." >&2
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

stage_release() {
  local archive="$1"
  local work_dir="$2"
  local staging_dir="$3"
  local extract_dir="${work_dir}/extract"
  local binary_path

  rm -rf "${extract_dir}" "${staging_dir}" || return
  mkdir -p "${extract_dir}" "${staging_dir}" || return
  chmod 0700 "${extract_dir}" "${staging_dir}" || return
  if ! tar -xzf "${archive}" -C "${extract_dir}"; then
    echo "Failed to extract release archive." >&2
    return 1
  fi

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
}

render_env_file() {
  local destination="$1"
  local repo="$2"
  local version="$3"
  local domain="$4"
  local ssl_cert="$5"
  local ssl_key="$6"
  local username="$7"
  local password="$8"
  local token="$9"
  local app_addr="${10}"
  local gin_mode="${11}"
  local download_base_url="${12}"

  (umask 077; cat > "${destination}" <<EOF
RELEASE_REPO=$(shell_quote "${repo}")
RELEASE_VERSION=$(shell_quote "${version}")
DOMAIN=$(shell_quote "${domain}")
SSL_CERT=$(shell_quote "${ssl_cert}")
SSL_KEY=$(shell_quote "${ssl_key}")
JW_USERNAME=$(shell_quote "${username}")
JW_PASSWORD=$(shell_quote "${password}")
JW_TOKEN=$(shell_quote "${token}")
APP_ADDR=$(shell_quote "${app_addr}")
GIN_MODE=$(shell_quote "${gin_mode}")
DOWNLOAD_BASE_URL=$(shell_quote "${download_base_url}")
EOF
  ) || return
  chmod 0600 "${destination}" || return
  chown root:root "${destination}" || return
}

render_systemd_service() {
  local destination="$1"

  cat > "${destination}" <<EOF || return
[Unit]
Description=BUPT_EC
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/bupt-ec
Restart=always
RestartSec=5
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectHome=true
ProtectSystem=full
ProtectClock=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
LockPersonality=true
RestrictSUIDSGID=true
CapabilityBoundingSet=
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
SystemCallArchitectures=native
ReadWritePaths=${INSTALL_DIR}/run_log

[Install]
WantedBy=multi-user.target
EOF
  chmod 0644 "${destination}" || return
  chown root:root "${destination}" || return
}

render_nginx_site() {
  local destination="$1"
  local domain="$2"
  local ssl_cert="$3"
  local ssl_key="$4"
  local app_addr="$5"

  cat > "${destination}" <<EOF || return
limit_req_zone \$binary_remote_addr zone=bupt_ec_api:10m rate=30r/m;

server {
    listen 80;
    listen [::]:80;
    server_name ${domain};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${domain};

    ssl_certificate ${ssl_cert};
    ssl_certificate_key ${ssl_key};
    ssl_protocols TLSv1.2 TLSv1.3;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "same-origin" always;
    add_header X-Frame-Options "DENY" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'" always;

    # Runtime metrics are loopback-only; do not proxy them to the public site.
    location = /metrics {
        return 404;
    }

    location /api/ {
        limit_req zone=bupt_ec_api burst=20 nodelay;
        proxy_pass http://${app_addr};
        proxy_http_version 1.1;
        proxy_connect_timeout 5s;
        proxy_send_timeout 15s;
        # Exceeds ClassroomRefreshLimit (30s) and Go WriteTimeout (45s) so a cold
        # refresh near the backend budget can still return JSON through the proxy.
        proxy_read_timeout 60s;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location / {
        proxy_pass http://${app_addr};
        proxy_http_version 1.1;
        proxy_connect_timeout 5s;
        proxy_send_timeout 15s;
        proxy_read_timeout 30s;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
  chmod 0644 "${destination}" || return
  chown root:root "${destination}" || return
}

prepare_staging() {
  local archive="$1"
  local work_dir="$2"
  local staging_dir="$3"
  local repo="$4"
  local version="$5"
  local domain="$6"
  local ssl_cert="$7"
  local ssl_key="$8"
  local username="$9"
  local password="${10}"
  local token="${11}"
  local app_addr="${12}"
  local gin_mode="${13}"
  local download_base_url="${14}"

  stage_release "${archive}" "${work_dir}" "${staging_dir}" || return
  render_env_file "${staging_dir}/bupt-ec.env" \
    "${repo}" "${version}" "${domain}" "${ssl_cert}" "${ssl_key}" \
    "${username}" "${password}" "${token}" "${app_addr}" "${gin_mode}" "${download_base_url}" || return
  render_systemd_service "${staging_dir}/${SERVICE_NAME}.service" || return
  render_nginx_site "${staging_dir}/${SERVICE_NAME}.conf" \
    "${domain}" "${ssl_cert}" "${ssl_key}" "${app_addr}" || return
}

transaction_targets() {
  printf '%s\t%s\n' \
    binary "${INSTALL_DIR}/bupt-ec" \
    env "${ENV_FILE}" \
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
    if curl -fsS --noproxy '*' --connect-timeout 2 --max-time 2 "${health_url}" >/dev/null; then
      return
    fi
    if (( attempt < 10 )); then
      sleep 1
    fi
  done

  echo "Service health check failed: ${health_url}" >&2
  return 1
}

commit_installation() {
  local staging_dir="$1"
  local app_addr="$2"
  local health_url

  mkdir -p "${INSTALL_DIR}/run_log" "${CONFIG_DIR}" || return
  chmod 0755 "${INSTALL_DIR}" || return
  chown root:root "${INSTALL_DIR}" || return
  chown -R "${APP_USER}:${APP_GROUP}" "${INSTALL_DIR}/run_log" || return
  chmod 0750 "${INSTALL_DIR}/run_log" || return

  atomic_install_file "${staging_dir}/bupt-ec" "${INSTALL_DIR}/bupt-ec" 0755 root:root || return
  atomic_install_file "${staging_dir}/bupt-ec.env" "${ENV_FILE}" 0600 root:root || return
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

main() {
  local repo version arch domain ssl_cert ssl_key username password_input password token app_addr gin_mode download_base_url
  local tmp_dir archive staging_dir backup_dir
  local has_password has_token

  require_installer_environment
  load_current_config

  repo="${REPO:-${CURRENT_RELEASE_REPO:-${DEFAULT_REPO}}}"
  # Explicit VERSION wins; otherwise preserve the installed channel/tag. A
  # first-time install keeps the historical rolling-nightly default.
  version="$(resolve_release_version "${VERSION:-}" "${CURRENT_RELEASE_VERSION}")"

  echo "BUPT_EC installer"
  echo
  repo="$(prompt_required "GitHub repository" "${repo}")"
  domain="$(prompt_required "Domain name" "${DOMAIN:-${CURRENT_DOMAIN}}")"
  ssl_cert="$(prompt_required "SSL certificate path" "${SSL_CERT:-${CURRENT_SSL_CERT:-/etc/letsencrypt/live/${domain}/fullchain.pem}}")"
  ssl_key="$(prompt_required "SSL private key path" "${SSL_KEY:-${CURRENT_SSL_KEY:-/etc/letsencrypt/live/${domain}/privkey.pem}}")"
  has_token=false
  if [[ -n "${JW_TOKEN:-${CURRENT_JW_TOKEN}}" ]]; then
    has_token=true
  fi
  token="$(prompt_optional_secret "JW token override, usually leave empty" "${has_token}")"
  if [[ -z "${token}" ]]; then
    token="${JW_TOKEN:-${CURRENT_JW_TOKEN}}"
  fi

  if [[ -n "${token}" ]]; then
    username="$(prompt "BUPT JW username, optional when JW token is set" "${JW_USERNAME:-${CURRENT_JW_USERNAME}}")"
  else
    username="$(prompt_required "BUPT JW username" "${JW_USERNAME:-${CURRENT_JW_USERNAME}}")"
  fi

  has_password=false
  if [[ -n "${JW_PASSWORD:-${CURRENT_JW_PASSWORD}}" ]]; then
    has_password=true
  fi
  if [[ -n "${token}" ]]; then
    password_input="$(prompt_optional_secret "BUPT JW password" "${has_password}")"
  else
    password_input="$(prompt_secret "BUPT JW password" "${has_password}")"
  fi
  if [[ -n "${password_input}" ]]; then
    password="${password_input}"
  else
    password="${JW_PASSWORD:-${CURRENT_JW_PASSWORD}}"
  fi

  if [[ -z "${token}" && ( -z "${username}" || -z "${password}" ) ]]; then
    echo "JW_TOKEN or both JW_USERNAME and JW_PASSWORD are required." >&2
    exit 1
  fi
  app_addr="$(prompt_required "Backend listen address" "${APP_ADDR:-${CURRENT_APP_ADDR:-${DEFAULT_APP_ADDR}}}")"
  gin_mode="$(prompt_required "Gin mode" "${GIN_MODE:-${CURRENT_GIN_MODE:-${DEFAULT_GIN_MODE}}}")"
  download_base_url="${DOWNLOAD_BASE_URL:-${CURRENT_DOWNLOAD_BASE_URL}}"

  validate_repo "${repo}"
  validate_version "${version}"
  validate_domain "${domain}"
  validate_absolute_path "SSL certificate path" "${ssl_cert}"
  validate_absolute_path "SSL private key path" "${ssl_key}"
  validate_app_addr "${app_addr}"
  validate_gin_mode "${gin_mode}"
  validate_download_base_url "${download_base_url}"

  if [[ ! -f "${ssl_cert}" ]]; then
    echo "SSL certificate not found: ${ssl_cert}" >&2
    exit 1
  fi
  if [[ ! -f "${ssl_key}" ]]; then
    echo "SSL private key not found: ${ssl_key}" >&2
    exit 1
  fi

  arch="$(detect_arch)"
  tmp_dir="$(mktemp -d)"
  chmod 0700 "${tmp_dir}"
  initialize_installer_session "${tmp_dir}"
  staging_dir="${tmp_dir}/staging"
  backup_dir="${tmp_dir}/backup"

  install_packages
  create_user
  download_release "${repo}" "${version}" "${arch}" "${tmp_dir}" "${download_base_url}"
  archive="${tmp_dir}/bupt-ec-linux-${arch}.tar.gz"
  prepare_staging "${archive}" "${tmp_dir}" "${staging_dir}" \
    "${repo}" "${version}" "${domain}" "${ssl_cert}" "${ssl_key}" \
    "${username}" "${password}" "${token}" "${app_addr}" "${gin_mode}" "${download_base_url}"
  perform_install_transaction "${staging_dir}" "${backup_dir}" "${app_addr}"

  echo
  echo "BUPT_EC is installed."
  echo "URL: https://${domain}/"
  echo "Service: systemctl status ${SERVICE_NAME}"
  echo "Upgrade later: rerun this installer."
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
