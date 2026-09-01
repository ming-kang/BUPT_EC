# shellcheck shell=bash

render_env_file() {
  local destination="$1"
  local key value_name

  (
    umask 077 || exit 1
    {
      for key in "${DEPLOYMENT_CONFIG_KEYS[@]}"; do
        value_name="CFG_${key}"
        printf '%s=' "${key}" || exit 1
        shell_quote "${!value_name-}" || exit 1
        printf '\n' || exit 1
      done
    } > "${destination}" || exit 1
  ) || return 1
  chmod 0600 "${destination}" || return 1
  chown root:root "${destination}" || return 1
}

# Public metadata intentionally contains only values needed by rootless CLI
# probes. It is rendered from validated CFG_* values, never copied from env.
render_deployment_metadata() {
  local destination="$1"

  (
    umask 077 || exit 1
    {
      printf 'RELEASE_VERSION=%s\n' "${CFG_RELEASE_VERSION}" || exit 1
      printf 'APP_ADDR=%s\n' "${CFG_APP_ADDR}" || exit 1
    } > "${destination}" || exit 1
  ) || return 1
  chmod 0644 "${destination}" || return 1
  chown root:root "${destination}" || return 1
}

# The marker makes commit behavior explicit; it must not infer a legacy remove
# action from a missing staging candidate.
render_cli_action() {
  local destination="$1"
  local action="$2"

  case "${action}" in
    install | remove) ;;
    *) return 1 ;;
  esac
  (
    umask 077 || exit 1
    printf '%s\n' "${action}" > "${destination}" || exit 1
  ) || return 1
  chmod 0600 "${destination}" || return 1
  chown root:root "${destination}" || return 1
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

  cat > "${destination}" <<EOF || return
limit_req_zone \$binary_remote_addr zone=bupt_ec_api:10m rate=30r/m;

server {
    listen 80;
    listen [::]:80;
    server_name ${CFG_DOMAIN};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${CFG_DOMAIN};

    ssl_certificate ${CFG_SSL_CERT};
    ssl_certificate_key ${CFG_SSL_KEY};
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
        proxy_pass http://${CFG_APP_ADDR};
        proxy_http_version 1.1;
        proxy_connect_timeout 5s;
        proxy_send_timeout 15s;
        # Exceeds ClassroomRefreshLimit (30s) and Go WriteTimeout (15s) so a cold
        # refresh near the backend budget can still return JSON through the proxy.
        proxy_read_timeout 60s;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location / {
        proxy_pass http://${CFG_APP_ADDR};
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

  stage_release "${archive}" "${work_dir}" "${staging_dir}" || return
  render_env_file "${staging_dir}/bupt-ec.env" || return
  render_systemd_service "${staging_dir}/${SERVICE_NAME}.service" || return
  render_nginx_site "${staging_dir}/${SERVICE_NAME}.conf" || return
  if is_cli_bearing_release "${CFG_RELEASE_VERSION}"; then
    render_deployment_metadata "${staging_dir}/deployment.meta" || return
    render_cli_action "${staging_dir}/cli.action" install || return
  else
    render_cli_action "${staging_dir}/cli.action" remove || return
  fi
}
