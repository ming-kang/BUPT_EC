# shellcheck shell=bash
# shellcheck disable=SC2034,SC2317,SC2329

test_version_policy() {
  assert_eq "latest" "$(resolve_release_version "" "")" "first install defaults to latest"
  assert_eq "latest" "$(resolve_release_version "latest" "v0.1.4")" "explicit version wins"
  assert_eq "v0.1.4" "$(resolve_release_version "" "v0.1.4")" "saved version is reused"

  local version
  for version in latest v0.1.4; do
    validate_version "${version}"
  done
  for version in "" nightly latest/asset v1 v1.2 v1.2.3.4 'v1.2.3;rm'; do
    assert_invalid_version "${version}"
  done

  # The nightly channel was removed; a machine still carrying
  # RELEASE_VERSION=nightly must fail with the migration command, not a bare
  # "invalid value" that leaves the operator guessing.
  local status output_file
  output_file="${TEST_TMP}/nightly-rejected.log"
  set +e
  validate_version nightly >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "removed nightly channel was accepted by validate_version"
  fi
  assert_contains "${output_file}" "VERSION must be latest or a stable tag" \
    "nightly rejection states the accepted values"
  assert_contains "${output_file}" "rerun with VERSION=latest" \
    "nightly rejection gives the migration command"

  host_reachable() { return 0; }
  assert_eq \
    "https://github.com/ming-kang/BUPT_EC/releases/latest/download" \
    "$(resolve_download_base_url "ming-kang/BUPT_EC" "latest" "")" \
    "latest release URL"
  assert_eq \
    "https://github.com/ming-kang/BUPT_EC/releases/download/v0.1.4" \
    "$(resolve_download_base_url "ming-kang/BUPT_EC" "v0.1.4" "")" \
    "stable tag release URL"
  assert_eq \
    "https://mirror.example/releases/v0.1.4" \
    "$(resolve_download_base_url "ignored/repo" "v0.1.4" "https://mirror.example/releases/v0.1.4/")" \
    "custom download URL"

  output_file="${TEST_TMP}/github-unreachable.log"
  set +e
  (
    host_reachable() { return 1; }
    resolve_download_base_url "ming-kang/BUPT_EC" "latest" ""
  ) >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "unreachable GitHub unexpectedly selected a download base"
  fi
  assert_contains "${output_file}" "no longer auto-selects third-party proxies" \
    "unreachable GitHub explains removed auto-proxy"
  assert_contains "${output_file}" "DOWNLOAD_BASE_URL=" \
    "unreachable GitHub suggests explicit mirror"
  assert_not_contains "${output_file}" "gh-v6.com" \
    "unreachable GitHub must not mention third-party proxy host"
  assert_not_contains "${output_file}" "https://gh-v6.com" \
    "unreachable GitHub must not select third-party proxy URL"

  output_file="${TEST_TMP}/explicit-mirror.log"
  set +e
  (
    host_reachable() { return 0; }
    resolve_download_base_url "ignored/repo" "v0.1.4" "https://mirror.example/releases/v0.1.4/"
  ) >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status != 0 )); then
    fail "explicit HTTPS mirror failed unexpectedly"
  fi
  assert_contains "${output_file}" "operator-configured HTTPS mirror host mirror.example" \
    "explicit mirror announces safe host label"
  assert_contains "${output_file}" "not independent GitHub publisher identity" \
    "explicit mirror explains checksum integrity boundary"
  assert_not_contains "${output_file}" "user:password@" \
    "explicit mirror log must not contain userinfo"

  output_file="${TEST_TMP}/insecure-mirror-rejected.log"
  unset ALLOW_INSECURE_DOWNLOAD_BASE_URL || true
  set +e
  (validate_download_base_url "http://mirror.example/releases/v0.1.4") >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "HTTP mirror without insecure opt-in was accepted"
  fi
  assert_contains "${output_file}" "must use https://" \
    "HTTP mirror rejection names HTTPS requirement"

  output_file="${TEST_TMP}/insecure-mirror-allowed.log"
  set +e
  (
    ALLOW_INSECURE_DOWNLOAD_BASE_URL=true
    validate_download_base_url "http://mirror.example/releases/v0.1.4"
  ) >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status != 0 )); then
    fail "HTTP mirror with insecure opt-in was rejected"
  fi
  assert_contains "${output_file}" "ALLOW_INSECURE_DOWNLOAD_BASE_URL=true" \
    "HTTP insecure opt-in prints warning"

  test_download_base_url_matrix
  test_download_base_url_secret_redaction
  test_curl_proto_args
}

test_download_base_url_matrix() {
  local url normalized status output_file

  # Accepted shapes (normalization strips trailing slash).
  for url in \
    "https://mirror.example" \
    "https://mirror.example/releases/v0.1.4" \
    "https://mirror.example/releases/v0.1.4/" \
    "https://127.0.0.1:8443/releases" \
    "https://[::1]:8443/releases"
  do
    unset ALLOW_INSECURE_DOWNLOAD_BASE_URL || true
    if ! normalized="$(normalize_download_base_url "${url}")"; then
      fail "expected accept: ${url}"
    fi
    if [[ "${normalized}" == */ ]]; then
      fail "normalized URL still has trailing slash: ${normalized}"
    fi
  done

  # HTTP requires the exact documented opt-in; broad truthy parsing would
  # silently widen this security boundary.
  unset ALLOW_INSECURE_DOWNLOAD_BASE_URL || true
  if normalized="$(normalize_download_base_url "http://mirror.local/releases" 2>/dev/null)"; then
    fail "HTTP without opt-in was accepted"
  fi
  for invalid_opt_in in 1 TRUE True yes; do
    ALLOW_INSECURE_DOWNLOAD_BASE_URL="${invalid_opt_in}"
    if normalized="$(normalize_download_base_url "http://mirror.local/releases" 2>/dev/null)"; then
      fail "HTTP accepted non-exact insecure opt-in: ${invalid_opt_in}"
    fi
  done
  ALLOW_INSECURE_DOWNLOAD_BASE_URL=true
  if ! normalized="$(normalize_download_base_url "http://mirror.local/releases")"; then
    fail "HTTP with opt-in was rejected"
  fi
  assert_eq "http://mirror.local/releases" "${normalized}" "HTTP opt-in normalizes"
  unset ALLOW_INSECURE_DOWNLOAD_BASE_URL || true

  # Rejected shapes: never accept non-HTTP(S) even with insecure opt-in.
  for url in \
    "file:///srv/releases" \
    "ftp://mirror.example/releases" \
    "data:text/plain,hi" \
    "gopher://mirror.example/releases" \
    "https://user:secret@mirror.example/releases" \
    "https://mirror.example/releases?token=secret" \
    "https://mirror.example/releases#fragment" \
    "https://[::::]/releases" \
    "https:///missing-host" \
    "https://mirror.example/releases;rm" \
    "https://mirror.example/re leases"
  do
    ALLOW_INSECURE_DOWNLOAD_BASE_URL=true
    output_file="${TEST_TMP}/reject-$(printf '%s' "${url}" | wc -c).log"
    set +e
    (normalize_download_base_url "${url}") >"${output_file}" 2>&1
    status=$?
    set -e
    if (( status == 0 )); then
      fail "expected reject: ${url}"
    fi
  done
}

test_download_base_url_secret_redaction() {
  local output_file status
  output_file="${TEST_TMP}/secret-userinfo.log"
  set +e
  (validate_download_base_url "https://user:s3cret-pass@mirror.example/releases") \
    >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "userinfo URL was accepted"
  fi
  assert_not_contains "${output_file}" "s3cret-pass" "userinfo password never printed"
  assert_not_contains "${output_file}" "user:s3cret" "userinfo never printed"
  assert_contains "${output_file}" "userinfo" "userinfo rejection names the rule"

  output_file="${TEST_TMP}/secret-query.log"
  set +e
  (validate_download_base_url "https://mirror.example/releases?token=tok_LIVE_secret") \
    >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "query URL was accepted"
  fi
  assert_not_contains "${output_file}" "tok_LIVE_secret" "query token never printed"
  assert_contains "${output_file}" "query" "query rejection names the rule"

  output_file="${TEST_TMP}/secret-resolve.log"
  set +e
  (
    resolve_download_base_url "ignored/repo" "v0.1.4" \
      "https://user:s3cret-pass@mirror.example/releases?token=tok_LIVE_secret"
  ) >"${output_file}" 2>&1
  status=$?
  set -e
  if (( status == 0 )); then
    fail "resolve accepted secret URL"
  fi
  assert_not_contains "${output_file}" "s3cret-pass" "resolve log has no password"
  assert_not_contains "${output_file}" "tok_LIVE_secret" "resolve log has no token"
}

test_curl_proto_args() {
  local -a https_args=()
  local -a http_args=()
  local arg

  while IFS= read -r -d '' arg; do
    https_args+=("${arg}")
  done < <(curl_download_proto_args "https")
  assert_eq 4 "${#https_args[@]}" "https proto arg count"
  assert_eq "--proto" "${https_args[0]}" "https --proto flag"
  assert_eq "=https" "${https_args[1]}" "https allows only https"
  assert_eq "--proto-redir" "${https_args[2]}" "https --proto-redir flag"
  assert_eq "=https" "${https_args[3]}" "https redirects only https"

  while IFS= read -r -d '' arg; do
    http_args+=("${arg}")
  done < <(curl_download_proto_args "http")
  assert_eq 4 "${#http_args[@]}" "http proto arg count"
  assert_eq "=http,https" "${http_args[1]}" "http break-glass allows http+https"
  assert_eq "=http,https" "${http_args[3]}" "http redirects allow http+https"
}

test_app_addr_validation() {
  local app_addr

  for app_addr in \
    "${DEFAULT_APP_ADDR}" \
    "api.internal.example:443" \
    "192.0.2.10:65535" \
    "[::1]:8080" \
    "[2001:db8::1]:8443" \
    "[::ffff:192.0.2.10]:8443"
  do
    if ! validate_app_addr "${app_addr}"; then
      fail "valid backend listen address was rejected: ${app_addr}"
    fi
  done

  for app_addr in \
    "\${host}:80" \
    "\$host:80" \
    'api.internal.example:80; return 200' \
    'api.internal.example:80 # injected' \
    'api.internal.example:0' \
    'api.internal.example:65536' \
    '999.999.999.999:80' \
    '::1:8080' \
    '[::1]8080' \
    '[not-an-ipv6]:80'
  do
    if validate_app_addr "${app_addr}" >/dev/null 2>&1; then
      fail "invalid backend listen address was accepted: ${app_addr}"
    fi
  done

  for unsafe_path in \
    "/etc/tls/\$host.pem" \
    '/etc/tls/#comment.pem' \
    '/etc/tls/{variable}.pem' \
    '/etc/tls/quote".pem' \
    "/etc/tls/quote'.pem" \
    '/etc/tls/back\\slash.pem'
  do
    if validate_absolute_path "TLS path" "${unsafe_path}" >/dev/null 2>&1; then
      fail "Nginx-unsafe certificate path was accepted: ${unsafe_path}"
    fi
  done
}

test_download_release_stops_after_base_resolution_failure() (
  local case_dir work_dir output_file resolution_log

  case_dir="${TEST_TMP}/download-base-resolution-failure"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  work_dir="${case_dir}/work"
  output_file="${case_dir}/output.log"
  resolution_log="${case_dir}/resolution.log"
  mkdir -p "${work_dir}"
  : > "${resolution_log}"
  resolve_download_base_url() {
    printf 'resolve\n' >> "${resolution_log}"
    return 1
  }

  if download_release "ming-kang/BUPT_EC" v1.2.3 amd64 "${work_dir}" "" > "${output_file}" 2>&1; then
    fail "download continued after download-base resolution failed"
  fi
  assert_command_count 1 resolve "${resolution_log}" "download base resolution count"
  assert_command_count 0 "curl " "${MOCK_COMMAND_LOG}" "curl after download-base resolution failure"
)

test_checksum_bypass_requires_exact_one() (
  local case_dir work_dir output_file

  case_dir="${TEST_TMP}/checksum-bypass-exact-one"
  mkdir -p "${case_dir}"
  setup_case "${case_dir}"
  work_dir="${case_dir}/work"
  output_file="${case_dir}/output.log"
  mkdir -p "${work_dir}"

  SKIP_CHECKSUM=true
  download_release "ming-kang/BUPT_EC" v1.2.3 amd64 "${work_dir}" \
    "https://mirror.example/releases" > "${output_file}" 2>&1
  assert_command_count 2 "curl " "${MOCK_COMMAND_LOG}" \
    "SKIP_CHECKSUM=true does not bypass package and checksum downloads"
  assert_contains "${output_file}" "Verifying package checksum" \
    "SKIP_CHECKSUM=true still verifies checksum"

  reset_mock_state "${case_dir}/exact-one"
  mkdir -p "${case_dir}/exact-one"
  work_dir="${case_dir}/exact-one/work"
  mkdir -p "${work_dir}"
  SKIP_CHECKSUM=1
  download_release "ming-kang/BUPT_EC" v1.2.3 amd64 "${work_dir}" \
    "https://mirror.example/releases" > "${output_file}" 2>&1
  assert_command_count 1 "curl " "${MOCK_COMMAND_LOG}" \
    "SKIP_CHECKSUM=1 bypasses checksum download only"
  assert_contains "${output_file}" "SKIP_CHECKSUM=1" \
    "exact checksum bypass remains loud"
)
