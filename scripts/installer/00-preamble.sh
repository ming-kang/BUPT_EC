# shellcheck shell=bash
# shellcheck disable=SC2034

SERVICE_NAME="bupt-ec"
DEFAULT_REPO="ming-kang/BUPT_EC"
GITHUB_HOST="github.com"
INSTALL_DIR="/opt/bupt-ec"
CONFIG_DIR="/etc/bupt-ec"
ENV_FILE="${CONFIG_DIR}/bupt-ec.env"
# Installed configuration is trusted only when root owns this exact file.
# configure_installer_test_root changes this test seam for portable fixtures.
ENV_FILE_EXPECTED_UID=0
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
SYSTEMD_ENABLED_LINK="/etc/systemd/system/multi-user.target.wants/${SERVICE_NAME}.service"
NGINX_SITE="/etc/nginx/sites-available/${SERVICE_NAME}.conf"
NGINX_ENABLED="/etc/nginx/sites-enabled/${SERVICE_NAME}.conf"
APP_USER="bupt-ec"
APP_GROUP="bupt-ec"
DEFAULT_APP_ADDR="127.0.0.1:8080"
TTY="/dev/tty"

# This is the complete persisted deployment contract. Runtime-only controls
# (for example SKIP_CHECKSUM) deliberately do not appear here.
DEPLOYMENT_CONFIG_KEYS=(
  RELEASE_REPO RELEASE_VERSION DOMAIN SSL_CERT SSL_KEY
  JW_USERNAME JW_PASSWORD JW_TOKEN APP_ADDR DOWNLOAD_BASE_URL
  LOG_CALLER READYZ_DIAGNOSTICS
)
readonly DEPLOYMENT_CONFIG_KEYS

INSTALLER_MODE="install"

INSTALLER_TMP_DIR=""
TRANSACTION_ACTIVE=false
TRANSACTION_BACKUP_DIR=""

# Captured at top-level load time. Inside functions, stdin-fed scripts report a
# fake BASH_SOURCE[0] (e.g. "main"), so entrypoint detection must not run there.
# - ./install.sh or curl|bash: not sourced → run main
# - source install.sh (unit tests): sourced → skip main
# Under `set -u`, BASH_SOURCE[0] is unbound for scripts fed on stdin — use :- .
INSTALLER_SOURCED=false
if [[ -n "${BASH_SOURCE[0]:-}" && "${BASH_SOURCE[0]}" != "$0" ]]; then
  INSTALLER_SOURCED=true
fi

# Tests source this script and call this explicit helper. Production main never
# reads a path override from the environment, so normal installer execution
# always targets the fixed /opt and /etc locations above.
configure_installer_test_root() {
  local root="$1"
  if [[ "${INSTALLER_SOURCED}" != "true" ]]; then
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
  # Test fixtures are owned by the invoking test user rather than root.
  ENV_FILE_EXPECTED_UID="${EUID}"
  SERVICE_FILE="${root}/etc/systemd/system/${SERVICE_NAME}.service"
  SYSTEMD_ENABLED_LINK="${root}/etc/systemd/system/multi-user.target.wants/${SERVICE_NAME}.service"
  NGINX_SITE="${root}/etc/nginx/sites-available/${SERVICE_NAME}.conf"
  NGINX_ENABLED="${root}/etc/nginx/sites-enabled/${SERVICE_NAME}.conf"
}
