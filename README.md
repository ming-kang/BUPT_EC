# BUPT_EC

BUPT_EC is a lightweight BUPT empty-classroom query service. It shows today's available classrooms for the Xitucheng and Shahe campuses by querying the BUPT teaching affairs system directly from a Go backend.

The current version no longer uses a local timetable database. Classroom availability comes from the official same-day `todayClassrooms` endpoint, and the backend obtains the required teaching affairs token automatically through a pure HTTP login flow.

## Features

- Shows available classrooms for today only.
- Supports Xitucheng and Shahe campuses.
- Filters by campus, building, and class period.
- Automatically logs in to the BUPT teaching affairs system and refreshes the token when needed.
- Uses a small in-memory same-day cache to reduce external requests.
- Serves the React frontend from the Go binary.

## Architecture

```text
Browser
  -> Go / Gin backend
  -> HTTP login to BUPT JW system
  -> todayClassrooms?campusId=01|04
  -> normalized JSON response
  -> React / Ant Design UI
```

The public API currently used by the frontend is:

- `GET /api/get_data`

Successful responses contain a `date`, refresh timestamps, stale status, campuses, buildings, rooms, and class-period nodes.

## Requirements

- Go 1.21+
- Node.js 18+
- pnpm
- A valid BUPT teaching affairs account

## Configuration

Create a `.env` file from `.env.example`:

```bash
JW_USERNAME=your_username
JW_PASSWORD=your_password
# Optional debug fallback only. Leave empty for automatic HTTP login.
JW_TOKEN=
```

Variables:

- `JW_USERNAME`: BUPT teaching affairs username.
- `JW_PASSWORD`: BUPT teaching affairs password.
- `JW_TOKEN`: optional emergency/debug token override. Leave it empty for normal automatic login.
- `APP_ADDR`: optional listen address. Use `127.0.0.1:8080` when running behind Nginx.

Do not commit real credentials, tokens, cookies, logs, or private config files.

## Local Development

Install frontend dependencies:

```bash
cd frontend
pnpm install
```

Build the frontend so the Go backend can embed and serve it:

```bash
cd frontend
pnpm build
cd ..
```

Run the backend:

```bash
go run ./
```

Open:

```text
http://127.0.0.1:8080/
```

Useful checks:

```bash
go test ./...
cd frontend && pnpm lint
cd frontend && pnpm build
```

## Production Build

Build the frontend first:

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..
```

Then build the Go binary:

```bash
go build -o bupt-ec -v ./
```

Run it with a `.env` file in the same directory, or provide the variables through your process manager:

```bash
./bupt-ec
```

By default, the server listens on `:8080`. Set `APP_ADDR=127.0.0.1:8080` when a reverse proxy handles public traffic.

## Releases

GitHub Releases are created automatically by the `Release` workflow when you push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds the frontend, embeds it into Linux binaries, and uploads:

- `bupt-ec-linux-amd64.tar.gz`
- `bupt-ec-linux-arm64.tar.gz`
- `checksums.txt`
- `install.sh`

The server does not need Go, Node.js, or pnpm when using release assets.

## Server Deployment

The recommended deployment path is the one-command installer on Debian/Ubuntu servers.

Assumptions:

- A domain such as `ec.example.com` already points to the server.
- SSL certificate and private key already exist on the server.
- The server can reach GitHub and the BUPT teaching affairs service over the network.

Run the latest installer from a release:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo bash
```

On IPv6-only servers that cannot reach GitHub directly, fetch the installer through `gh-v6.com`:

```bash
curl -fsSL https://gh-v6.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | sudo bash
```

The script interactively asks for:

- GitHub repository, default `ming-kang/BUPT_EC`
- domain name
- SSL certificate path
- SSL private key path
- BUPT teaching affairs username and password
- optional token override
- backend listen address, default `127.0.0.1:8080`

It then installs required system packages, downloads the latest matching Linux release for `amd64` or `arm64`, writes `/etc/bupt-ec/bupt-ec.env`, configures `systemd`, configures Nginx on ports `80` and `443`, and starts the service.

To upgrade later, rerun the same command. Existing configuration is reused as defaults, and the password can be kept by pressing Enter at the password prompt.

You can install a specific version or a fork by setting environment variables:

```bash
curl -fsSL https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh | \
  sudo REPO=ming-kang/BUPT_EC VERSION=v0.1.0 bash
```

Some IPv6-only servers cannot reach GitHub release downloads because GitHub's main release download endpoints may not be IPv6 reachable from every network. The installer automatically falls back to `gh-v6.com` when direct GitHub access fails. If the installer itself cannot be downloaded from GitHub, use the `gh-v6.com` installer command above. If both GitHub and `gh-v6.com` are unavailable, mirror the release files to an IPv6-reachable HTTPS directory and set `DOWNLOAD_BASE_URL`:

```bash
curl -fsSL https://your-ipv6-reachable.example/install.sh | \
  sudo DOWNLOAD_BASE_URL=https://your-ipv6-reachable.example/releases/v0.1.0 bash
```

The custom directory must contain `bupt-ec-linux-amd64.tar.gz` or `bupt-ec-linux-arm64.tar.gz`; `checksums.txt` is optional but recommended.

The service can be managed with:

```bash
sudo systemctl status bupt-ec
sudo systemctl restart bupt-ec
sudo journalctl -u bupt-ec -f
```

### Manual Systemd Deployment

A manual Linux deployment can use `systemd`.

Example directory:

```text
/opt/bupt-ec/
  bupt-ec
  .env
```

Example service file:

```ini
[Unit]
Description=BUPT_EC
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/bupt-ec
ExecStart=/opt/bupt-ec/bupt-ec
Restart=always
RestartSec=5
EnvironmentFile=/opt/bupt-ec/bupt-ec.env

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now bupt-ec
sudo systemctl status bupt-ec
```

If you want to expose it through a domain, put Nginx or another reverse proxy in front of `127.0.0.1:8080`.

Minimal Nginx example:

```nginx
server {
    listen 80;
    server_name your.domain.example;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Caching Behavior

The backend keeps a same-day in-memory cache:

- fresh cache TTL: about 5 minutes
- stale cache: allowed only until the end of the same day
- cross-day cache reuse: not allowed

The cache is refreshed on demand when the frontend requests `GET /api/get_data`; there is no background scheduler.

If the teaching affairs system is temporarily unavailable but today's cached data exists, the API returns `stale=true` and the frontend shows a warning.

## Security Notes

- The backend never needs a browser, Playwright, Selenium, or Chromium.
- Tokens are stored in memory only.
- The generic `.ec.gob` cache path from the old implementation is no longer used.
- Keep `.env`, logs, and runtime artifacts out of version control.

## Limitations

- Only Xitucheng and Shahe are supported.
- Only today's classroom availability is supported.
- The service depends on the BUPT teaching affairs system. If login rules, captcha rules, or API formats change, the HTTP login/query logic may need updates.
