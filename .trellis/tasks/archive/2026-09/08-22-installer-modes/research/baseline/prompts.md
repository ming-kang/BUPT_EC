# Installer Prompt Baseline

Current visible sequence in `main()` before mode work:

1. Banner: `BUPT_EC installer`, then blank line.
2. `prompt_required "GitHub repository"` default `REPO > CURRENT_RELEASE_REPO > DEFAULT_REPO`.
3. `prompt_required "Domain name"` default `DOMAIN > CURRENT_DOMAIN`.
4. `prompt_required "SSL certificate path"` default `SSL_CERT > CURRENT_SSL_CERT > /etc/letsencrypt/live/${domain}/fullchain.pem`.
5. `prompt_required "SSL private key path"` default `SSL_KEY > CURRENT_SSL_KEY > /etc/letsencrypt/live/${domain}/privkey.pem`.
6. `prompt_optional_secret "JW token override, usually leave empty"`; `[keep existing]` when `JW_TOKEN > CURRENT_JW_TOKEN` is non-empty. Empty answer preserves that value.
7. Username prompt:
   - token non-empty: `prompt "BUPT JW username, optional when JW token is set"` default `JW_USERNAME > CURRENT_JW_USERNAME`;
   - token empty: `prompt_required "BUPT JW username"` with same default.
8. Password prompt:
   - token non-empty: `prompt_optional_secret "BUPT JW password"`;
   - token empty: `prompt_secret "BUPT JW password"`;
   - `[keep existing]` when `JW_PASSWORD > CURRENT_JW_PASSWORD` is non-empty; empty answer preserves it.
9. Credential rule: token OR username+password.
10. `prompt_required "Backend listen address"` default `APP_ADDR > CURRENT_APP_ADDR > DEFAULT_APP_ADDR`.
11. `DOWNLOAD_BASE_URL` is not prompted; value is `DOWNLOAD_BASE_URL > CURRENT_DOWNLOAD_BASE_URL`.
12. Completion output:
    - blank line
    - `BUPT_EC is installed.`
    - `URL: https://${domain}/`
    - `Service: systemctl status bupt-ec`
    - `Upgrade later: rerun this installer.`

Version is not prompted: `VERSION > CURRENT_RELEASE_VERSION > latest` through `resolve_release_version`.
