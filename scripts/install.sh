#!/usr/bin/env bash
set -euo pipefail

APP_NAME="empty-classroom"
SERVICE_NAME="empty-classroom"
DEFAULT_REPO="ming-kang/EmptyClassroom"
GITHUB_HOST="github.com"
GITHUB_IPV6_PROXY_HOST="gh-v6.com"
INSTALL_DIR="/opt/empty-classroom"
CONFIG_DIR="/etc/empty-classroom"
ENV_FILE="${CONFIG_DIR}/empty-classroom.env"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
NGINX_SITE="/etc/nginx/sites-available/${SERVICE_NAME}.conf"
NGINX_ENABLED="/etc/nginx/sites-enabled/${SERVICE_NAME}.conf"
APP_USER="empty-classroom"
APP_GROUP="empty-classroom"
DEFAULT_APP_ADDR="127.0.0.1:8080"
TTY="/dev/tty"

if [[ "${EUID}" -ne 0 ]]; then
  echo "This installer must run as root. Use: curl -fsSL <url> | sudo bash" >&2
  exit 1
fi

if [[ ! -r "${TTY}" ]]; then
  echo "Interactive input requires a TTY." >&2
  exit 1
fi

CURRENT_RELEASE_REPO=""
CURRENT_DOMAIN=""
CURRENT_SSL_CERT=""
CURRENT_SSL_KEY=""
CURRENT_JW_USERNAME=""
CURRENT_JW_PASSWORD=""
CURRENT_JW_TOKEN=""
CURRENT_APP_ADDR=""
CURRENT_DOWNLOAD_BASE_URL=""

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
  set +a
  CURRENT_RELEASE_REPO="${RELEASE_REPO:-}"
  CURRENT_DOMAIN="${DOMAIN:-}"
  CURRENT_SSL_CERT="${SSL_CERT:-}"
  CURRENT_SSL_KEY="${SSL_KEY:-}"
  CURRENT_JW_USERNAME="${JW_USERNAME:-}"
  CURRENT_JW_PASSWORD="${JW_PASSWORD:-}"
  CURRENT_JW_TOKEN="${JW_TOKEN:-}"
  CURRENT_APP_ADDR="${APP_ADDR:-}"
  CURRENT_DOWNLOAD_BASE_URL="${DOWNLOAD_BASE_URL:-}"
fi

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

shell_quote() {
  printf "'"
  printf "%s" "$1" | sed "s/'/'\\\\''/g"
  printf "'"
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

resolve_download_base_url() {
  local repo="$1"
  local version="$2"
  local override_url="$3"
  local host="${GITHUB_HOST}"

  if [[ -n "${override_url}" ]]; then
    printf "%s" "${override_url%/}"
    return
  fi

  if ! host_reachable "${GITHUB_HOST}"; then
    echo "GitHub is not reachable directly; trying ${GITHUB_IPV6_PROXY_HOST}." >&2
    if host_reachable "${GITHUB_IPV6_PROXY_HOST}"; then
      host="${GITHUB_IPV6_PROXY_HOST}"
    else
      echo "Neither ${GITHUB_HOST} nor ${GITHUB_IPV6_PROXY_HOST} is reachable." >&2
      exit 1
    fi
  fi

  if [[ "${version}" == "latest" ]]; then
    printf "https://%s/%s/releases/latest/download" "${host}" "${repo}"
  else
    printf "https://%s/%s/releases/download/%s" "${host}" "${repo}" "${version}"
  fi
}

download_release() {
  local repo="$1"
  local version="$2"
  local arch="$3"
  local work_dir="$4"
  local download_base_url="$5"
  local package_name="EmptyClassroom-linux-${arch}.tar.gz"
  local base_url

  base_url="$(resolve_download_base_url "${repo}" "${version}" "${download_base_url}")"

  echo "Downloading ${repo} ${version} (${arch}) from ${base_url}..."
  curl -fL "${base_url}/${package_name}" -o "${work_dir}/${package_name}"

  if curl -fsL "${base_url}/checksums.txt" -o "${work_dir}/checksums.txt"; then
    (cd "${work_dir}" && grep " ${package_name}$" checksums.txt | sha256sum -c -)
  else
    echo "checksums.txt not found; skipping checksum verification."
  fi
}

install_binary() {
  local archive="$1"
  local work_dir="$2"
  local extract_dir="${work_dir}/extract"
  local binary_path

  rm -rf "${extract_dir}"
  mkdir -p "${extract_dir}"
  tar -xzf "${archive}" -C "${extract_dir}"

  binary_path="$(find "${extract_dir}" -type f -name EmptyClassroom | head -n 1)"
  if [[ -z "${binary_path}" ]]; then
    echo "Release archive does not contain EmptyClassroom binary." >&2
    exit 1
  fi

  mkdir -p "${INSTALL_DIR}/run_log"
  install -m 0755 "${binary_path}" "${INSTALL_DIR}/EmptyClassroom"
  chown -R "${APP_USER}:${APP_GROUP}" "${INSTALL_DIR}"
}

write_env() {
  local repo="$1"
  local domain="$2"
  local ssl_cert="$3"
  local ssl_key="$4"
  local username="$5"
  local password="$6"
  local token="$7"
  local app_addr="$8"
  local download_base_url="$9"

  mkdir -p "${CONFIG_DIR}"
  cat > "${ENV_FILE}" <<EOF
RELEASE_REPO=$(shell_quote "${repo}")
DOMAIN=$(shell_quote "${domain}")
SSL_CERT=$(shell_quote "${ssl_cert}")
SSL_KEY=$(shell_quote "${ssl_key}")
JW_USERNAME=$(shell_quote "${username}")
JW_PASSWORD=$(shell_quote "${password}")
JW_TOKEN=$(shell_quote "${token}")
APP_ADDR=$(shell_quote "${app_addr}")
DOWNLOAD_BASE_URL=$(shell_quote "${download_base_url}")
EOF
  chmod 0600 "${ENV_FILE}"
  chown root:root "${ENV_FILE}"
}

write_systemd_service() {
  cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=EmptyClassroom
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${INSTALL_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/EmptyClassroom
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=full
ReadWritePaths=${INSTALL_DIR}/run_log

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "${SERVICE_NAME}"
}

write_nginx_site() {
  local domain="$1"
  local ssl_cert="$2"
  local ssl_key="$3"
  local app_addr="$4"

  mkdir -p /etc/nginx/sites-available /etc/nginx/sites-enabled
  cat > "${NGINX_SITE}" <<EOF
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

    location / {
        proxy_pass http://${app_addr};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF

  ln -sfn "${NGINX_SITE}" "${NGINX_ENABLED}"
  nginx -t
}

main() {
  local repo version arch domain ssl_cert ssl_key username password_input password token app_addr download_base_url tmp_dir archive

  repo="${REPO:-${CURRENT_RELEASE_REPO:-${DEFAULT_REPO}}}"
  version="${VERSION:-latest}"

  echo "EmptyClassroom installer"
  echo
  repo="$(prompt_required "GitHub repository" "${repo}")"
  domain="$(prompt_required "Domain name" "${DOMAIN:-${CURRENT_DOMAIN}}")"
  ssl_cert="$(prompt_required "SSL certificate path" "${SSL_CERT:-${CURRENT_SSL_CERT:-/etc/letsencrypt/live/${domain}/fullchain.pem}}")"
  ssl_key="$(prompt_required "SSL private key path" "${SSL_KEY:-${CURRENT_SSL_KEY:-/etc/letsencrypt/live/${domain}/privkey.pem}}")"
  username="$(prompt_required "BUPT JW username" "${JW_USERNAME:-${CURRENT_JW_USERNAME}}")"

  password_input="$(prompt_secret "BUPT JW password" "$([[ -n "${JW_PASSWORD:-${CURRENT_JW_PASSWORD}}" ]] && echo true || echo false)")"
  if [[ -n "${password_input}" ]]; then
    password="${password_input}"
  else
    password="${JW_PASSWORD:-${CURRENT_JW_PASSWORD}}"
  fi

  token="$(prompt "JW token override, usually leave empty" "${JW_TOKEN:-${CURRENT_JW_TOKEN}}")"
  app_addr="$(prompt_required "Backend listen address" "${APP_ADDR:-${CURRENT_APP_ADDR:-${DEFAULT_APP_ADDR}}}")"
  download_base_url="${DOWNLOAD_BASE_URL:-${CURRENT_DOWNLOAD_BASE_URL}}"

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
  trap 'rm -rf "${tmp_dir}"' EXIT

  install_packages
  create_user
  write_env "${repo}" "${domain}" "${ssl_cert}" "${ssl_key}" "${username}" "${password}" "${token}" "${app_addr}" "${download_base_url}"
  download_release "${repo}" "${version}" "${arch}" "${tmp_dir}" "${download_base_url}"
  archive="${tmp_dir}/EmptyClassroom-linux-${arch}.tar.gz"
  install_binary "${archive}" "${tmp_dir}"
  write_systemd_service
  write_nginx_site "${domain}" "${ssl_cert}" "${ssl_key}" "${app_addr}"

  systemctl restart "${SERVICE_NAME}"
  systemctl reload nginx

  echo
  echo "EmptyClassroom is installed."
  echo "URL: https://${domain}/"
  echo "Service: systemctl status ${SERVICE_NAME}"
  echo "Upgrade later: rerun this installer."
}

main "$@"
