# Migrate Frontend to React 19

## Goal

Move the frontend from React 18 to the stable React 19 line so the application stays current with its ecosystem, while preserving all user-visible behavior and keeping the strict TypeScript, test, build, bundle-budget, audit, and embedded-release gates green.

## Background and Confirmed Facts

- Audit backlog item F-02 was intentionally deferred until the strict TypeScript migration completed; that prerequisite is now satisfied.
- `frontend/package.json` currently declares React/React DOM `^18.3.1` and React 18 declaration packages.
- The frontend already uses the modern JSX transform, `react-dom/client.createRoot`, `React.StrictMode`, explicit children types, initialized refs, and Testing Library APIs. Source inspection found no local use of React APIs removed in React 19.
- Stable registry versions verified on 2026-08-22 are React/React DOM 19.2.8, `@types/react` 19.2.18, and `@types/react-dom` 19.2.4.
- antd 5.29.3's peer ranges admit React 19, but official antd v5 guidance requires `@ant-design/v5-patch-for-react-19` for Button wave and dynamic overlay rendering. The current antd version satisfies that package's peer floor.
- React 18 baseline: typecheck and lint pass; 18 test files / 120 tests pass; production assets total 209,594 B gzip against the 230,888 B budget; production and development audits report no known vulnerabilities.
- Baseline test output already contains one Node `TimeoutNaNWarning` and jsdom pseudo-element `getComputedStyle` messages. They are not React migration failures; no new React/ReactDOM/antd/`act`/ref/root warning may be added.

Detailed evidence is in `research/react-19-compatibility.md`.

## Requirements

- **R1 — Atomic runtime and type migration:** Upgrade `react` and `react-dom` together to matching stable 19.2 versions and move both React declaration packages to compatible 19.2 versions. The installed graph must contain one React runtime version.
- **R2 — antd v5 compatibility:** Keep antd on major version 5 and add/load the official React 19 compatibility package in the shipped application and direct jsdom component-test environment.
- **R3 — Minimal dependency scope:** Update only the React runtime/type packages, the required antd compatibility package, and peer-resolution metadata. Supporting dependencies may move only when a reproduced incompatibility proves the smallest necessary update; no broad latest update or unrelated deduplication.
- **R4 — Behavior preservation:** Preserve rendering, classroom querying, selection and preference persistence, settings, loading/error states, Modal behavior, dark mode, accessibility, browser target, API contracts, and bundle chunk boundaries. Introduce no intentional product behavior change.
- **R5 — Narrow compatibility adaptation:** Resolve demonstrated React 19 runtime, type, StrictMode, and test issues without warning suppression, broad casts, weakened compiler/lint rules, or removing StrictMode. This evidence requirement applies to behavioral code/test changes; documentation, path, and version-comment corrections required by R7 are exempt.
- **R6 — Regression evidence:** Exercise existing antd Button/Modal, error-boundary, picker, table, and SWR lifecycle tests under React 19 and compare warning classes with the documented React 18 baseline.
- **R7 — Documentation and specification:** Record the dependency migration in `CHANGELOG.md`, update the current frontend dependency contract, record the post-migration bundle baseline without raising its budget, and correct directly encountered stale current React/TypeScript references. Archived historical evidence remains unchanged.

## Acceptance Criteria

- [x] `frontend/package.json` and `frontend/pnpm-lock.yaml` resolve matching React/React DOM 19.2 runtimes with compatible React 19.2 declaration packages and no React 18 runtime snapshot.
- [x] `@ant-design/v5-patch-for-react-19` is a production dependency and is loaded before application rendering; direct jsdom component tests use the same compatibility bridge.
- [x] antd 5, SWR, Testing Library, and Vite tooling peer requirements are satisfied without forced/ignored peers or unrelated major upgrades.
- [x] Behavioral source or test-assertion changes beyond dependency and bridge wiring are backed by a reproduced React 19 incompatibility and preserve existing behavior; documentation/comment-only corrections remain non-behavioral.
- [x] `pnpm typecheck`, `pnpm lint`, and all existing 120 tests plus any migration-specific test pass.
- [x] Test output introduces no new React, React DOM, antd compatibility, `act`, ref, root, or unmount warning relative to the documented baseline.
- [x] A fresh `pnpm build` and `pnpm size` pass at or below the unchanged 230,888 B gzip budget, with the exact React 19 measurement recorded.
- [x] `pnpm audit:prod` and `pnpm audit:dev` pass under existing thresholds.
- [x] `task check`, `task test`, `task build`, tagless Go build, vulnerability scan, installer tests, and shell lint pass (or any unavailable external tool is reported explicitly rather than marked as run).
- [x] `CHANGELOG.md`, `.trellis/spec/backend/quality-guidelines.md`, bundle-baseline documentation, and directly affected current docs/comments match the final implementation.

## Out of Scope

- React Server Components, SSR, hydration, framework migration, or React Compiler adoption.
- New React 19 APIs or user-facing features unrelated to compatibility.
- antd 6, Vite 8, browser-baseline changes, or the separate F-04 bundle/first-paint optimization backlog.
- API, backend, cache, deployment, or selection-model changes.
- Raising the bundle budget, broad lockfile deduplication, or unrelated dependency churn.
- Remediating the two documented pre-existing test-warning classes.
- Correcting unrelated pre-existing API-spec semantics such as the stale success-`log_id` statement; record it as deferred documentation debt rather than changing it under this migration.
