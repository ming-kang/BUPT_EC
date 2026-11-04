# BUPT_EC

BUPT_EC is a lightweight BUPT empty-classroom query service. It shows today's available classrooms for the Xitucheng and Shahe campuses by querying the BUPT teaching affairs system directly from a Go backend — no local timetable database, no browser automation.

## Features

- Today's available classrooms for both campuses, filterable by building and class period.
- Automatic HTTP login to the teaching affairs system with in-memory token refresh.
- Same-day in-memory cache with stale-while-revalidate, so the page stays up even when the upstream system flakes.
- Single static binary: the React frontend is embedded in and served by the Go backend.

## Architecture

```text
Browser
  -> Go backend (embedded React + Ant Design UI)
  -> HTTP login to BUPT JW system
  -> todayClassrooms?campusId=01|04
  -> normalized JSON response
```

Endpoints:

- `GET /api/get_data` — the single public API used by the frontend.
- `GET /healthz` — liveness probe.
- `GET /readyz` — readiness probe.

## Deploy to a server

One command on a Debian/Ubuntu server with a domain and TLS certificate already in place:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=latest bash
```

The installer never prompts: a first install takes its settings from
environment variables (for example `DOMAIN`, `JW_USERNAME`, `JW_PASSWORD`;
certificate paths and `APP_ADDR` have defaults), and an existing installation
reuses its saved values. Stable `vX.Y.Z` tags are the only release channel;
`VERSION=vX.Y.Z` selects an immutable release, while a first install with no
saved or explicit version selects `latest`.

| Mode | Use | Version selection and behavior |
| --- | --- | --- |
| omitted / `--mode=install` | First install or reinstall with explicit/saved settings | `VERSION` → saved version → `latest`; installs apt packages |
| `--mode=update` | Existing deployment update | `VERSION` → saved version; no apt package install |
| `--mode=reconfigure` | Change saved deployment settings | Keeps the saved version and ignores `VERSION` |

Manage and inspect the installation with the standard tools:

```bash
sudo systemctl status bupt-ec      # service state
sudo systemctl restart bupt-ec     # apply a config change by hand
sudo journalctl -u bupt-ec -f      # logs
curl -s http://127.0.0.1:8080/readyz   # readiness (minimal status+version body)
```

To update to a new release (or roll back to an earlier stable one), rerun the
current Installer in update mode:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo bash -s -- --mode=update
# Or pin a specific stable release (also the rollback selector):
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo VERSION=v0.2.0 bash -s -- --mode=update
```

See [docs/upgrading.md](docs/upgrading.md) for mirror and rollback rules.

If update reports repairable saved settings, rerun with `--mode=reconfigure`
and the corrected variables; if saved release metadata is missing, use the default
install mode to rebuild it. If the installer says it cannot load the env safely,
repair its ownership/mode/syntax as root or move it aside and run default
install; see the upgrade guide for details.

Supported production topology is **one** `bupt-ec` process behind Nginx on a
single host. Cache and JW refresh state are process-local; multi-instance
active/active is not recommended (see [operations.md](docs/operations.md#deployment-topology-supported-today)).

- Full setup guide: [docs/deployment.md](docs/deployment.md)
- Upgrading and rollback: [docs/upgrading.md](docs/upgrading.md)
- Day-to-day operation and troubleshooting: [docs/operations.md](docs/operations.md)

## Develop locally

```bash
task build   # frontend build + embed + Go build → ./bupt-ec (bupt-ec.exe on Windows)
./bupt-ec
# open http://127.0.0.1:8080/
```

`go run ./` also works but compiles without the `embed_assets` build tag, so it serves the API plus a placeholder page instead of the UI; the native equivalent of `task build` is `cd frontend && pnpm install && pnpm build && cd .. && rm -rf web/dist && cp -r frontend/dist web/dist && go build -tags embed_assets -o bupt-ec ./` — without the `-trimpath -ldflags "-s -w -X main.version=..."` that `task build` adds, so the binary reports version `dev`.

Requires Go 1.26.9+, Node 22, pnpm 9.15.x, [Task](https://taskfile.dev) for the `task` entry points, and JW credentials from the process environment or an optional `.env` (see `.env.example`). Full guide including tests and an architecture tour: [docs/development.md](docs/development.md).

## Documentation

| Document | Contents |
|---|---|
| [docs/deployment.md](docs/deployment.md) | Server prerequisites, TLS, one-command install, manual systemd/Nginx setup |
| [docs/upgrading.md](docs/upgrading.md) | Upgrading, verifying, rollback, certificate renewal |
| [docs/operations.md](docs/operations.md) | Service management, health endpoints, logs and `log_id` tracing, caching, troubleshooting |
| [docs/development.md](docs/development.md) | Local setup, tests, project structure, backend/frontend architecture |
| [docs/release.md](docs/release.md) | Versioning, changelog conventions, release pipeline |
| [CHANGELOG.md](CHANGELOG.md) | Notable changes per version |

## Security

- Credentials come from the process environment or `.env` locally and `/etc/bupt-ec/bupt-ec.env` (root-owned mode `0600`) on servers; configuration is snapshotted at startup and tokens are held in memory only.
- Never commit real credentials, tokens, or logs.

## Limitations

- Only Xitucheng and Shahe campuses; only today's availability.
- Depends on the BUPT teaching affairs system: if its login rules, captcha rules, or API formats change, the HTTP login/query logic may need updates.

## License

See [LICENSE](LICENSE).
