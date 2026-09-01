# Full Repository Strict Code Quality Review

## Verdict

**Not approved.**

The repository has strong baseline engineering discipline and the broad automated checks pass, but it does not meet the supplied strict approval bar. There are two blockers:

1. the new pre-serialized backend cache is published through two independent atomics and can serve bytes from the previous snapshot as if they belonged to the newly validated cache;
2. the project-owned installer and installer test files exceed 1,000 lines without a compelling source-structure justification.

There are also high-severity boundary and atomicity problems in installer configuration persistence, crash recovery, the Safari 14 fetch path, and frontend payload normalization.

## Reviewed Scope and Exclusions

Reviewed project-owned surfaces:

- Go runtime and tests: root entry points, `config/`, `logs/`, `service/`, `service/model/`, and `web/`;
- React/Vite source, tests, styles, API normalization, SWR orchestration, build configuration, and bundle tooling under `frontend/`;
- installer/release/quality surfaces: `scripts/`, `Taskfile.yml`, `.github/workflows/`, `.env.example`, `README.md`, `docs/`, and `CHANGELOG.md`;
- cross-layer flows: JW payload → backend model → JSON → frontend, installer env → `config.Load`, timeout budgets, frontend embedding, and release artifact composition;
- recent substantive commit `daa7b98`: the cache fast path is the source of Blocker 1; its native frontend `Panel`/tag/typography replacements were separately checked and found structurally sound.

Explicit exclusions:

- line-by-line review of managed Trellis/agent harness internals under `.trellis/`, `.agents/`, `.kiro/`, and `.pi/`;
- generated lockfiles and build artifacts as ordinary hand-maintained modules;
- credentialed real-JW integration tests;
- product-code fixes, which require a separate approval boundary.

## Prioritized Findings

### Blocker 1 — the fast-path cache can return a previous snapshot as the current fresh snapshot

**Anchors:** `service/classroom_service.go:41-50`, `service/classroom_service.go:170-211`, `service/realtime_data.go:255-270`, `handler.go:98-104`, `service/export_test.go:13-23`

`ClassroomService` publishes one logical cache generation through two independent atomic pointers:

1. `todayCache.Store(today)` publishes the new model;
2. `updateCachedDataJSON(today)` marshals that model and later stores its JSON bytes.

`GetCachedDataJSON` independently loads and validates `todayCache`, then loads `cachedDataJSON`. A request between the two stores can validate new snapshot **B** while receiving serialized bytes for old snapshot **A**. The marshal between stores makes the mixed-generation window materially larger than a single instruction.

This can briefly return old dates, timestamps, rooms, or partial-campus content under a successful fresh response. At a day boundary, it can defeat the documented cross-day protection: the model check is performed against today's object while the returned bytes may still describe yesterday.

The comment claiming the fields are updated “atomically alongside” each other is incorrect; separate atomic variables provide race freedom per variable, not transactional publication across both.

**Preferred remedy / code-judo move:** replace both fields with one atomic pointer to one immutable cache entry, for example:

```go
type classroomCacheEntry struct {
    data      *model.TodayClassrooms
    freshJSON []byte
}
```

Marshal first, then publish the entry with one store. Every cache decision and fast-path byte read must come from the same loaded entry. A marshal failure can publish `data` with `freshJSON == nil` and naturally fall back to the typed slow path. This deletes the paired-state synchronization problem rather than trying to order two stores.

**Verification gap:** existing tests seed the same two fields sequentially, and handler benchmarks use fake bytes. No test forces a mixed-generation publication or midnight interleaving.

---

### Blocker 2 — the installer source and test suite are both over 1,000 lines and combine too many responsibilities

**Anchors:** `scripts/install.sh:1-1218`, especially function groups at `scripts/install.sh:79-535`, `scripts/install.sh:537-775`, and `scripts/install.sh:777-1115`; `scripts/install_test.sh:1-1104`, especially `scripts/install_test.sh:9-429` and `scripts/install_test.sh:431-1099`

- `scripts/install.sh`: **1,218 lines**
- `scripts/install_test.sh`: **1,104 lines**

This is not merely a line-count problem. One source file owns interactive input, migration of saved state, validation, mirror policy, download/checksum handling, extraction, three template renderers, transaction snapshots, file replacement, rollback, health validation, cleanup traps, and top-level orchestration. Its test file combines a large mock operating system, shared fixtures/assertions, policy tests, render tests, transaction tests, rollback tests, and entrypoint tests.

The published installer must remain a self-contained release asset, but that constrains the **artifact**, not the hand-maintained source. Keeping all source logic in one file makes unrelated policy changes touch the transaction engine and encourages the existing 11-/13-positional-argument APIs.

**Preferred remedy / code-judo move:** maintain focused installer source fragments and generate the one-file release artifact deterministically:

- input/config migration and validated deployment model;
- release/mirror/download/checksum policy;
- env/systemd/Nginx rendering;
- transaction snapshot/commit/recovery;
- a small orchestration entrypoint.

Add a regeneration/drift check, and run syntax, ShellCheck, and behavior suites against the assembled artifact that is actually released. Split tests into a shared mock/fixture layer plus policy, rendering/config, and transaction/recovery suites.

This is not a request to distribute runtime helper files. The final `install.sh` remains self-contained; only source ownership changes.

---

### High 1 — installer upgrades silently delete supported runtime settings

**Anchors:** `config/config.go:13-21`, `config/config.go:39-46`, `.env.example:7-12`, `docs/operations.md:87-88`, `docs/operations.md:143`, `scripts/install.sh:19-28`, `scripts/install.sh:79-97`, `scripts/install.sh:611-635`, `scripts/install_test.sh:388-403`

The runtime supports and documents `LOG_CALLER` and `READYZ_DIAGNOSTICS`, including setting them in `/etc/bupt-ec/bupt-ec.env`. The installer sources that file but copies only ten hard-coded values into `CURRENT_*`, then completely rewrites the environment file from the same incomplete list. A successful upgrade therefore deletes both supported flags without warning.

A targeted probe seeded both values in an installed environment file and confirmed that the rendered upgrade candidate omitted them.

The 11 positional arguments to `render_env_file` and 13 to `prepare_staging` are symptoms of the missing model: there is no canonical definition of the persisted deployment configuration.

**Preferred remedy / code-judo move:** define one validated deployment-config carrier and one explicit ownership policy for each key. Load, validate, render, and preservation-test that model through one path. This removes `CURRENT_*` fan-out, positional argument shifting, and silent key loss together.

`ALLOW_INSECURE_DOWNLOAD_BASE_URL` is deliberately **not** included in this finding: requiring renewed explicit consent for an HTTP mirror is a defensible security boundary under the current documentation.

---

### High 2 — the installer transaction rolls back ordinary command failures but is not crash-durable

**Anchors:** `scripts/install.sh:836-864`, `scripts/install.sh:1033-1058`, `scripts/install.sh:1064-1108`, `scripts/install.sh:1193-1207`, `docs/upgrading.md:24`, `docs/release.md:25`

Each individual target replacement is a same-filesystem atomic rename, but the installation is a sequential update of the binary, env file, systemd unit, Nginx site, symlink, and runtime activation state. Backup state is stored under a session `mktemp -d`, and recovery depends on in-process globals plus an `EXIT` trap.

`SIGKILL`, power loss, kernel termination, or host failure after any replacement can leave a mixed installation generation. The trap cannot run, the transaction has no durable journal, and the next installer invocation has no recovery discovery path. Existing tests cover command failures that return control to the shell; they cannot establish interruption atomicity.

This is a strict-quality crash-consistency risk beyond the current documented command-failure rollback matrix. The existing transaction is strong against ordinary returned failures, but it should not be described or reasoned about as an all-or-nothing multi-file commit under non-trappable interruption.

**Preferred remedy:** write a root-only transaction journal and backup to a stable path before the first replacement. Persist stage transitions, fsync where needed, and make every new invocation recover or complete the prior transaction before beginning another. Add subprocess tests that terminate after each commit/activation stage and then invoke recovery in a fresh process.

---

### High 3 — the advertised Safari 14 target cannot execute the fetch path, and transport failures bypass the typed error boundary

**Anchors:** `frontend/vite.config.js:30-33`, `frontend/src/useTodayClassrooms.ts:133-168`, `frontend/src/useTodayClassrooms.lifecycle.test.tsx:275-305`

The frontend explicitly targets Safari 14 to support older iOS devices, but `fetchTodayClassrooms` unconditionally calls `AbortSignal.timeout`. That API is unavailable in Safari 14. Vite transpiles JavaScript syntax; it does not polyfill missing browser APIs.

On those supported clients, signal creation throws before a request starts. The catch converts only `TimeoutError`; the resulting `TypeError` is rethrown unchanged. Generic network `TypeError`s follow the same path, and `errorAsEnvelope` then surfaces browser-provided text instead of the controlled `ApiError` channel promised by the data-layer contract.

The lifecycle timeout test stubs an existing `AbortSignal.timeout`, so it cannot detect the supported-browser failure.

**Preferred remedy / code-judo move:** create one fetch-boundary timeout helper that feature-detects native `AbortSignal.timeout` and otherwise uses `AbortController` plus a cleared timer. Convert every non-`ApiError` transport failure into one safe, localized `ApiError`. This replaces separate timeout and generic-error behavior with one explicit boundary.

---

### High 4 — frontend normalization performs a shallow check and then casts untrusted wire data to the complete domain model

**Anchors:** `frontend/src/api/types.ts:1-10`, `frontend/src/todayClassroomsResponse.ts:92-137`, `frontend/src/classroomDataValidity.ts:13-45`, `frontend/src/App.tsx:43-61`, `frontend/src/components/BuildingPicker.tsx:18-31`, `frontend/src/components/TodayClassroomTable.tsx:48-81`

`normalizeResponse` checks only that `data` is an object and `campuses` is an array, then casts the body to `TodayClassroomsData`. `isUsableBusinessDaySnapshot` adds date and `stale_until` validation, but does not validate campus, building, room, or node entries.

For example, a same-day payload with `campuses: [null]` passes both boundaries. `App.tsx` then executes `campuses.find((campus) => campus.id === ...)` and can throw. Other malformed nested fields reach sorting, mapping, selection, and table paths. The scattered `Array.isArray` guards in components are evidence that the payload was not actually normalized into a trusted domain model.

This contradicts the comment in `api/types.ts` that runtime validation at the payload boundary is mandatory and makes the cast hide rather than establish the invariant.

**Preferred remedy / code-judo move:** make `todayClassroomsResponse.ts` the single canonical wire decoder. Validate and normalize every field the render paths consume, including explicit backward compatibility such as `display_name || name`, then return a trusted domain snapshot. Components should consume that trusted model and delete defensive parsing/casts instead of owning partial definitions of valid data.

---

### Medium 1 — `Shutdown` does not close worker admission when `Run` was never called

**Anchors:** `service/warmup.go:119-124`, `service/warmup.go:196-232`, `service/refresh_coordinator.go:111-139`, `service/warmup_test.go:318-350`

The admission gate rejects refreshes only when a non-nil lifecycle has been canceled. On a service that never called `Run`, `Shutdown` has no cancel function, unlocks `backgroundMu`, and begins `refreshWorkers.Wait`; another caller can then acquire the lock and call `refreshWorkers.Add(1)`.

That contradicts the function comment and runtime spec claiming `Shutdown` is safe on a never-run service and prevents `Add` after draining begins. It can return before a newly admitted worker finishes and violates the intended `WaitGroup.Add`/`Wait` ordering.

Production currently calls `Run` before shutdown, which reduces immediate exposure, but the public lifecycle contract and tests explicitly support never-run services.

**Preferred remedy:** add a terminal shutdown/admission state under `backgroundMu`, set it before releasing the lock in every `Shutdown` path, and reject new workers based on that state independently of `lifecycleCtx`.

---

### Medium 2 — timeout comments and source-backed toolchain guidance have drifted from executable policy

**Anchors:** `main.go:110-118`, `frontend/src/useTodayClassrooms.ts:17-21`, `scripts/install.sh:721-729`, `docs/operations.md:51-63`, `main_test.go:7-18`, `.trellis/spec/backend/quality-guidelines.md:273-282`, `go.mod:3`, `.github/workflows/quality.yml:71-80`

The executable timeout contract is 5s cold wait, 15s Go `WriteTimeout`, 30s background refresh, and 60s Nginx API timeout. Comments in `main.go`, the frontend hook, and generated Nginx template still describe a cold handler waiting for the full 30s refresh and a 45s Go write timeout. The tests and operator documentation say the opposite.

Separately, the source-backed quality spec still mandates Go 1.25.12 while `go.mod`, CI, AGENTS, README, and development docs use 1.25.13. This matters because future agents are explicitly instructed to treat the spec as canonical.

**Preferred remedy:** make the operations timeout table and `go.mod`/CI version the canonical sources, remove duplicated numeric prose where possible, and update the generated template/spec in the same change.

---

### Medium 3 — the allocation benchmark is not a usable quality gate and the fast path lacks contract tests

**Anchors:** `handler.go:94-103`, `handler_test.go:573-636`, `service/classroom_service.go:195-211`, `router.go:58-80`

The benchmark calls `GetData`, which emits an info log on every iteration. The full benchmark produced more than 20 MB of output in seconds and was killed by the harness. A later isolated run still flooded the output before producing a measurement. That makes routine comparison noisy and brittle.

More importantly, the benchmark fake supplies pre-serialized bytes directly; it does not exercise publication from `ClassroomService`, mixed generations, cross-day behavior, or exact fast/slow-path parity. Broad tests passing therefore provide no evidence against Blocker 1.

**Preferred remedy:** install a discard logger for benchmarks, separate serializer/response-writer microbenchmarks from handler behavior, and add deterministic contract tests for:

- model+JSON generation identity;
- midnight/cross-day rejection;
- partial-to-full and full-to-full publication interleavings;
- exact headers and decoded envelope parity between fast and slow paths.

## Code-Judo Opportunities

1. **One immutable cache entry** removes the cache race, paired atomics, misleading comments, and special test seeding in one move.
2. **One deployment-config model** removes `CURRENT_*` fan-out, 11-/13-argument calls, and silently dropped runtime keys.
3. **Generated self-contained installer artifact** preserves release UX while allowing focused source modules and test suites.
4. **One canonical frontend decoder and one fetch boundary** removes deep untrusted casts, scattered component guards, Safari feature assumptions, and browser-message leakage.
5. **Directly stream the pre-serialized payload** after the cache publication is fixed: `writePreserializedGetData` currently copies the entire cached tree into a pooled `bytes.Buffer`, and the pool can retain response-sized arrays. Writing small escaped envelope fragments and `dataJSON` directly to the response removes that copy and the large-buffer retention.

## File-Size Assessment

| File | Lines | Assessment |
| --- | ---: | --- |
| `scripts/install.sh` | 1,218 | Blocking project-owned source; multiple unrelated responsibilities; decompose source and generate artifact. |
| `scripts/install_test.sh` | 1,104 | Blocking project-owned test source; split fixtures and scenario suites. |
| `frontend/pnpm-lock.yaml` | 5,642 | Generated lockfile; justified exemption. |
| `.pi/extensions/trellis/index.ts` | 1,976 | Managed/generated Trellis integration; outside product-code line review and justified exemption. |

No Go or frontend production module approaches 1,000 lines. The largest frontend production source is `useTodayClassrooms.ts` at 373 lines; its complexity is concentrated but currently has substantial focused tests.

## Verification Results

Canonical backend checks were rerun with `GOTOOLCHAIN=go1.25.13` (the `go.mod`/CI toolchain). Canonical frontend package-manager checks used pnpm 9.15.0 through Corepack; the available Node runtime was v24.13.0 rather than CI's Node 22. Shell checks used Bash 5.3.15 and ShellCheck 0.11.0.

| Check | Result |
| --- | --- |
| `git diff --check` | Passed. |
| `GOTOOLCHAIN=go1.25.13 gofmt -l .` | Passed; no files reported. |
| `GOTOOLCHAIN=go1.25.13 go vet ./...` | Passed. |
| `GOTOOLCHAIN=go1.25.13 go mod tidy -diff` | Passed. |
| `GOTOOLCHAIN=go1.25.13 go mod verify` | Passed. |
| `GOTOOLCHAIN=go1.25.13 go test -race ./...` | Passed for all Go packages. |
| `GOTOOLCHAIN=go1.25.13 go build ./...` | Passed. |
| `GOTOOLCHAIN=go1.25.13 go build -tags embed_assets ./...` after copying frontend dist | Passed. |
| `corepack pnpm@9.15.0 -C frontend install --frozen-lockfile` | Passed with the repository's pinned pnpm line. |
| pnpm 9.15 frontend lint/typecheck | Passed. |
| pnpm 9.15 frontend tests | Passed: 18 files, 127 tests. |
| pnpm 9.15 frontend build | Passed. |
| Bundle-size check | Passed: **164,862 B gzip** against 230,888 B budget. |
| pnpm 9.15 production/toolchain audits | Passed; no known vulnerabilities. |
| `bash -n` for scripts | Passed. |
| `bash scripts/install_test.sh` | Passed. |
| `shellcheck scripts/*.sh` | Passed. |
| `GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` | Passed; no vulnerabilities found. |
| Same vulnerability scan with local Go 1.26.4 | Failed on six reachable standard-library advisories fixed by Go 1.26.6; this is a local toolchain issue, not a repository dependency finding. |
| `go test -run '^$' -bench 'Benchmark(GetDataSuccess|GetDataHandlerOnly|Healthz)$' -benchmem ./` | Failed as a practical gate: emitted more than 20 MB of per-request logs and the harness killed it; see Medium 3. |
| Real JW integration tests | Not run; credentials/network access are out of scope. |

Passing broad checks does not clear the blockers: the cache issue is a logical cross-generation race not detected by the race detector, and the installer configuration/crash cases are outside current test scenarios.

## Confirmed Healthy Areas / Rejected False Positives

- Token replacement and auth-recovery singleflight recheck current state under lock; delayed replacement and metrics tests cover the risky paths.
- Refresh backoff/jitter and stale/partial outcome ownership are generally cohesive and well tested outside the split cache publication.
- SWR polling/retry/visibility orchestration is intentionally complex but centralized and strongly tested; the remaining effects synchronize real external state or reducer convergence rather than duplicating ordinary render state.
- The new native `Panel`, tag, and typography replacements are direct, reusable, and materially reduced the bundle. They are not thin-wrapper regressions.
- GitHub Actions use pinned action SHAs, reusable quality gates, timeouts, and consistent frontend artifact names/paths.
- `Taskfile.yml` intentionally omits the build-dependent bundle-size gate from `task check` and documents the manual command; this is not accidental drift.
- No tracked credentials, runtime logs, binaries, or dist artifacts were found.

## Recommended Remediation Order

1. Replace the paired cache atomics with one immutable cache entry and add publication/parity tests.
2. Introduce the deployment-config model and preserve `LOG_CALLER` plus `READYZ_DIAGNOSTICS` explicitly.
3. Decompose installer/test source while generating the same self-contained release artifact.
4. Add durable installer transaction recovery for non-trappable interruption.
5. Fix the Safari-compatible timeout/transport boundary.
6. Implement one canonical deep frontend decoder and remove scattered defensive parsing.
7. Close never-run shutdown admission and repair timeout/toolchain documentation drift.
8. Make benchmarks quiet, deterministic, and contract-relevant.

## Final Approval Decision

**Rejected under the requested strict code-quality bar.** The repository should not be considered structurally approved until both blockers are resolved and the high-severity boundary defects have concrete remediation plans or fixes.

## Local Audit Inputs

- `.trellis/tasks/09-01-perf-quality-review/research/repository-audit-map.md`
- `.trellis/spec/backend/index.md`
- `.trellis/spec/backend/directory-structure.md`
- `.trellis/spec/backend/runtime-state-and-cache.md`
- `.trellis/spec/backend/api-contract.md`
- `.trellis/spec/backend/error-handling.md`
- `.trellis/spec/backend/logging-guidelines.md`
- `.trellis/spec/backend/quality-guidelines.md`
- `.trellis/spec/guides/code-reuse-thinking-guide.md`
- `.trellis/spec/guides/cross-layer-thinking-guide.md`

## Sources

- [MDN: AbortSignal.timeout()](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal/timeout_static)
- [Can I Use: AbortSignal.timeout()](https://caniuse.com/wf-abortsignal-timeout)
