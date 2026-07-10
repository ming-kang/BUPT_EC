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
  -> Go / Gin backend (embedded React + Ant Design UI)
  -> HTTP login to BUPT JW system
  -> todayClassrooms?campusId=01|04
  -> normalized JSON response
```

Endpoints:

- `GET /api/get_data` — the single public API used by the frontend.
- `GET /healthz` — liveness probe.
- `GET /readyz` — readiness probe with runtime diagnostics.

## Deploy to a server

One command on a Debian/Ubuntu server with a domain and TLS certificate already in place.

**Production** — prefer a stable release:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo VERSION=latest bash
```

**Edge** — rolling `nightly` (freshest `main`; also the first-install fallback
when neither `VERSION` nor a saved release choice exists):

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/download/nightly/install.sh | sudo VERSION=nightly bash
```

The installer configures systemd and Nginx, asks for your JW credentials interactively, and starts the service. Upgrading later is the same command (pin a stable tag in production).

- Full setup guide: [docs/deployment.md](docs/deployment.md)
- Upgrading and rollback: [docs/upgrading.md](docs/upgrading.md)
- Day-to-day operation and troubleshooting: [docs/operations.md](docs/operations.md)

## Develop locally

```bash
cd frontend && pnpm install && pnpm build && cd ..
go run ./
# open http://127.0.0.1:8080/
```

Requires Go 1.25+, Node 22, pnpm 9.15.x, and JW credentials from the process environment or an optional `.env` (see `.env.example`). Full guide including tests and an architecture tour: [docs/development.md](docs/development.md).

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

- Credentials come from the process environment or `.env` locally and `/etc/bupt-ec/bupt-ec.env` (root-only) on servers; configuration is snapshotted at startup and tokens are held in memory only.
- Never commit real credentials, tokens, or logs.

## Limitations

- Only Xitucheng and Shahe campuses; only today's availability.
- Depends on the BUPT teaching affairs system: if its login rules, captcha rules, or API formats change, the HTTP login/query logic may need updates.

## License

See [LICENSE](LICENSE).
