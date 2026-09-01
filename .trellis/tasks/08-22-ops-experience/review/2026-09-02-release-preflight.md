# v0.3.0 local release preflight — 2026-09-02

## Gate results

| Command | Result |
| --- | --- |
| `task check` | Passed after remediation: gofmt/vet/module checks; generator, recursive Bash syntax, Installer (41 scenarios/44 functions), CLI (10 scenarios), release-layout, ShellCheck; frontend lint/typecheck; Vitest (18 files/127 tests); production and development audits. |
| `task test` | Passed: `go test -race ./...` for root, config, logs, service, and web packages. |
| `task installer:check` | Passed independently: generator drift, recursive syntax/ShellCheck, Installer/CLI behavior suites, stable and `main-<sha>` release-layout simulation. |
| `pnpm -C frontend build && pnpm -C frontend size` | Passed: 1,496 modules; total gzip size 164,862 B, below the 230,888 B budget. |
| `go build ./...` | Passed (tagless bare-clone build). |
| `rm -rf web/dist && cp -r frontend/dist web/dist && go build -tags embed_assets ./...` | Passed (fresh embedded-assets build). |
| `GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` | Passed: no reachable vulnerabilities. |
| `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.10 .github/workflows/ci.yml .github/workflows/quality.yml .github/workflows/release.yml` | Passed with no findings. |
| `git diff --check` | Passed (the existing `task.json` CRLF-normalization warning is emitted by Git but is not a whitespace error). |
| `python ./.trellis/scripts/task.py validate 08-22-ops-experience` | Passed: four real entries in each context manifest. |

## Remediated failure (not hidden)

The first `task check` failed only at `pnpm -C frontend audit:dev`: two high-severity `browserslist@4.28.5` advisories (`GHSA-c83g-rgw3-j3cx`, `GHSA-73wf-gq98-2v4g`) arrived through the development-only Babel/Vite path. The initial literal `actionlint` command also could not run because the executable is not installed on this host; the pinned `go run ...@v1.7.10` equivalent above completed successfully.

The dependency issue was fixed narrowly by adding the single pnpm override `browserslist: 4.28.7`, regenerating `frontend/pnpm-lock.yaml` with pnpm 9.15.0, and adding an Unreleased security-fix note. A frozen install refreshed the local dependency tree. The complete gate sequence above was then rerun successfully; both frontend audits report no known vulnerabilities.

## Artifact hygiene

The frontend build was copied to ignored `web/dist` only for the embed-build gate. No credentials, tokens, deployment environment data, or large test logs were written into this task evidence.

## Release boundary

This review is local-only. It authorizes the next planned normal `main` push once the evidence/remediation commit is made and the tree is clean; it does not authorize `scripts/release.sh v0.3.0`, a v0.3.0 tag/release, or nightly deletion. Those remain blocked by the explicitly deferred real Linux E2E gate.
