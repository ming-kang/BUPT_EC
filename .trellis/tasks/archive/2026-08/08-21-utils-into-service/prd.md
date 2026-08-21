# Move utils HTTP client into service package

## Goal

Audit item B-08 (from `archive/2026-08/08-07-project-audit-optimize/research/audit-backend.md`): the
`utils/` package currently contains only the JW outbound HTTP helper and acts as a "junk drawer"
boundary that attracts unrelated helpers. Move it into `service/` under a JW-specific name with
idiomatic Go naming, then delete the `utils/` package entirely.

## Background (verified 2026-08-21)

- `utils/http.go` (~94 lines) exports: `HTTPDoer`, `CheckRedirect`, `NewHTTPClient`, `HttpGet`,
  `HttpPostForm`, `HttpPostWithHeader` (+ unexported `httpRequest`, `maxResponseBodyBytes`).
- Production usage:
  - `main.go:45` — `utils.NewHTTPClient()` (composition root).
  - `service/jw_client.go:27,30,52,91,113` — field type `utils.HTTPDoer`; calls
    `HttpPostWithHeader` / `HttpPostForm` / `HttpGet`.
- Test usage: `service/testsupport_test.go:97` (`utils.NewHTTPClient()`); `utils/http_test.go`
  holds 7 tests of the helpers themselves.
- Doc/spec references to sync: `.trellis/spec/backend/directory-structure.md` (layout tree,
  composition-root narrative, signatures block, Correct example),
  `docs/development.md:135,137`, root `AGENTS.md` structure sentence ("Shared packages are
  `config/`, `logs/`, and `utils/`").

## Requirements

1. Move `utils/http.go` → `service/jw_http.go`; move `utils/http_test.go` →
   `service/jw_http_test.go` (package `service`). The `utils/` directory is removed.
2. Rename to idiomatic Go, minimizing exported surface:
   - `NewHTTPClient` → exported `service.NewJWHTTPClient() *http.Client` (needed by `main.go`
     composition root and test support).
   - `HttpGet` / `HttpPostForm` / `HttpPostWithHeader` → unexported `httpGet` / `httpPostForm` /
     `httpPostWithHeader` (only `jw_client.go` calls them after the move).
   - `CheckRedirect` → unexported (`checkRedirect`); keep the explanatory comment about why JW
     outbound HTTP must never follow redirects (credential-forwarding hazard) verbatim in intent.
   - `HTTPDoer` stays exported as `service.HTTPDoer` — it appears in the exported
     `NewJWClient(username, password string, client HTTPDoer)` injection seam used by tests.
3. Behavior must be preserved exactly: redirect rejection (including error text semantics), 5 MiB
   body cap, header defaults (`Accept`), form encoding, context propagation, transport timeouts,
   proxy-from-environment. No functional changes beyond naming/moving.
4. Update all references: `main.go`, `service/jw_client.go`, `service/testsupport_test.go`, plus
   the doc/spec files listed above. After the change, `rg "BUPT_EC/utils"` over tracked files
   returns nothing.
5. No CHANGELOG entry (internal refactor; no user-visible behavior, endpoint, config, or
   deployment change). Docs updates are limited to the internal composition-root descriptions.

## Constraints

- Follow `.trellis/spec/backend/directory-structure.md`: exported names only for cross-package
  APIs; focused file named after its runtime role (`jw_http.go` fits the existing
  `jw_client.go` / `crypto.go` / `urlutil.go` family).
- Do not change `NewJWClient`'s signature shape (username, password, doer injection seam) or the
  nil-doer constructor error contract.
- gofmt-clean; no new dependencies.

## Acceptance Criteria

- [ ] `utils/` package no longer exists; all former behavior lives in `service/jw_http.go` with
      the naming from Requirement 2.
- [ ] `go build ./...` and `go vet ./...` pass; `gofmt -l` reports nothing in changed files.
- [ ] `go test -race ./...` fully green with no test-logic edits needed beyond mechanical renames
      (the 7 moved helper tests keep passing; redirect-rejection and body-cap tests included).
- [ ] `rg -n "BUPT_EC/utils|utils\.(HTTPDoer|NewHTTPClient|HttpGet|HttpPostForm|HttpPostWithHeader|CheckRedirect)"`
      over tracked files returns zero matches.
- [ ] `.trellis/spec/backend/directory-structure.md`, `docs/development.md`, and `AGENTS.md` no
      longer reference a `utils/` package; composition-root examples show
      `service.NewJWHTTPClient()`.
- [ ] Embed build still works: `task build` succeeds (or equivalent
      `go build -tags embed_assets`) proving the composition root wires correctly.
