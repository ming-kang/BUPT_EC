# Implementation Plan — React 19 Compatibility Migration

## Ordered Checklist

### 1. Confirm baseline and transaction boundaries

- [x] Confirm the working tree contains only this task's planning artifacts.
- [x] Confirm pnpm is 9.15.x and the registry targets remain stable, non-canary versions documented in `research/react-19-compatibility.md`.
- [x] Treat the recorded React 18 baseline as the comparison point: 120 tests, 209,594 B gzip, and only the two documented pre-existing warning classes.

Rollback point: no product files changed.

### 2. Upgrade the minimal dependency set atomically

Edit all five ranges in `frontend/package.json` as one manifest change, then perform one pnpm 9.15 resolve from `frontend/`:

```bash
pnpm install
pnpm install --frozen-lockfile
```

The manifest transaction sets React/DOM `^19.2.8`, the antd patch `^1.0.3`, `@types/react ^19.2.18`, and `@types/react-dom ^19.2.4` before the lockfile is regenerated. If the single resolve fails, restore both manifest and lockfile and run the old frozen install to restore `node_modules` before continuing; do not leave mixed runtime/type state.

- [x] Review `package.json` and lockfile diff: only the requested packages, required peer snapshots, and unavoidable resolution metadata may change.
- [x] Do **not** run `pnpm update --latest` or `pnpm dedupe`; baseline dedupe findings are unrelated.
- [x] Verify installed runtime/type versions and one React runtime graph:

```bash
pnpm list react react-dom @types/react @types/react-dom --depth 100
pnpm why react react-dom @ant-design/v5-patch-for-react-19
node -e "for (const p of ['react','react-dom','@types/react','@types/react-dom']) console.log(p, require(p + '/package.json').version)"
```

Rollback point: restore `frontend/package.json` and `frontend/pnpm-lock.yaml`; no source adaptation yet.

### 3. Install the antd v5 render bridge in runtime and tests

- [x] Add `import "@ant-design/v5-patch-for-react-19";` before application imports/rendering in `frontend/src/main.tsx`.
- [x] In `frontend/src/test/setup.ts`, await the same patch only when `window` exists: direct jsdom component tests bypass `main.tsx` but must use the production antd renderer, while pure node tests must not load the antd graph. Keep actual DOM stubs guarded.
- [x] Keep the existing `createRoot`, `React.StrictMode`, root ErrorBoundary, browser target, and chunk rules.
- [x] Correct the stale `App.jsx` comment in `main.tsx` while touching the entrypoint.

### 4. Run fast compatibility feedback and adapt only demonstrated failures

Run in this order:

```bash
pnpm typecheck
pnpm lint
pnpm exec vitest run \
  src/components/ToggleButtonGroup.test.tsx \
  src/components/BuildingPicker.test.tsx \
  src/components/CampusButtonGroup.test.tsx \
  src/components/ClassTimePicker.test.tsx \
  src/components/CampusSettingsModal.test.tsx \
  src/components/TodayClassroomTable.test.tsx \
  src/components/ErrorBoundary.test.tsx \
  src/useTodayClassrooms.lifecycle.test.tsx
```

- [x] Compare output against the documented baseline; reject new React/ReactDOM/antd compatibility, `act`, ref, root, or unmount warnings.
- [x] If React 19 exposes a real source/type incompatibility, add the smallest source/test fix and record its cause in the task notes or diff.
- [x] Do not weaken TypeScript/ESLint, remove StrictMode, add broad casts, suppress warnings globally, or adopt unrelated React 19 APIs.
- [x] Keep existing behavioral assertions for selection, retries, Modal content, error boundaries, and accessibility intact unless React 19 changes only the test harness contract and the change is justified.

Rollback point: revert the narrow compatibility adaptation independently before reconsidering dependency scope.

### 5. Synchronize current documentation and specs

- [x] Add a `[Unreleased]` dependency entry to `CHANGELOG.md` for React 19.2 and the official antd v5 bridge.
- [x] Update `.trellis/spec/backend/quality-guidelines.md` from the React `^18.3` line lock to `^19.2`, document the mandatory antd v5 patch, and preserve all other major-line/browser-target rules.
- [x] Preserve the dated 2026-07-27 budget basis (209,898 B) in `frontend/scripts/check-bundle-size.mjs`; add the current 2026-08-22 React 18 baseline (209,594 B) and the separately measured React 19 total without changing `BUDGET_BYTES = 230_888`.
- [x] Correct **every nonhistorical current-source extension reference** in `docs/development.md`, `.trellis/spec/backend/quality-guidelines.md`, `.trellis/spec/backend/api-contract.md`, and `.trellis/spec/backend/directory-structure.md` (`.js`/`.jsx` → actual `.ts`/`.tsx`, including code-fence path labels and test names), plus `frontend/src/main.tsx:9` and `frontend/src/components/ErrorBoundary.test.tsx:55`. Preserve genuine JavaScript tooling paths such as `vite.config.js` / `eslint.config.js` and do not rewrite archived task/audit history.
- [x] Use an explicit repo-wide `rg` sweep to account for every remaining `.js`/`.jsx` hit in current docs/specs as either a real JS config, historical release text, or a defect to correct. Path-only edits in `api-contract.md` must not alter its separate stale success-`log_id` semantics, which are documentation debt from B-11 and remain outside this React migration.

### 6. Full frontend validation

From `frontend/`:

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test
pnpm build
pnpm size
pnpm audit:prod
pnpm audit:dev
```

- [x] All 120 existing tests plus any migration-specific tests pass.
- [x] No new migration warning class appears.
- [x] Fresh build remains at or below 230,888 B gzip; record the exact total in the bundle script comment and task report.
- [x] Inspect generated filenames/module output to confirm `react-vendor` and `antd-vendor` chunk boundaries still exist and the patch did not create a new eager chunk.
- [x] Both audit thresholds remain green, and pure node tests still pass despite the test-setup compatibility registration.

### 7. Repository integration and review

From repository root:

```bash
task check
task test
task build
go build ./...
rm -rf web/dist && cp -r frontend/dist web/dist && go build -tags embed_assets ./...
task vuln
bash scripts/install_test.sh
shellcheck scripts/*.sh
git diff --check
git status --short
```

- [x] Frozen install, Go formatting/vet/tidy/verify, frontend gates, race tests, tagless and embedded-assets builds pass.
- [x] CI-only vulnerability, installer, and shell-lint gates pass locally; if a required external tool is unavailable, record that environment limitation explicitly rather than claiming the gate ran.
- [x] Inspect the final diff for unintended dependency churn, generated assets, warning suppression, bundle-budget changes, or product behavior changes.
- [x] Run an independent Trellis quality review against PRD/design/research and resolve every finding before commit.

### 8. Finish-work gates

- [x] Complete the required spec synchronization pass.
- [x] Add a concise journal/task outcome including exact versions, tests, warning comparison, bundle delta, and rollback shape.
- [x] Commit one scoped Conventional Commit (planned: `chore(frontend): migrate to React 19`).
- [x] Archive the Trellis task only after the committed tree is clean and all acceptance criteria are met.

## Execution Results — 2026-08-22

- Resolved one runtime graph: React/React DOM 19.2.8, `@types/react` 19.2.18, `@types/react-dom` 19.2.4, antd 5.29.3, and official patch 1.0.3. Lockfile package-key changes are limited to those replacements, scheduler 0.23.2 → 0.27.0, patch addition, and removal of the no-longer-needed `@types/prop-types`.
- The application imports the patch before rendering. Vitest setup awaits it only under jsdom; an initial static setup import was rejected because it needlessly loaded the full antd graph in pure node tests.
- Frontend gates pass: frozen install, typecheck, lint, 18 files / 120 tests, build (1,494 modules), size, and both audits. Output contains only the documented pre-existing `TimeoutNaNWarning` and jsdom pseudo-element `getComputedStyle` messages.
- Production assets measure **224,343 B gzip**, +14,749 B from the 209,594 B React 18 baseline and 6,545 B below the unchanged 230,888 B budget. `react-vendor` and `antd-vendor` remain the only framework/vendor chunks; the patch is in `antd-vendor`.
- Repository/CI-equivalent gates pass: `task check`, `task test`, `task build`, tagless build, explicit `-tags embed_assets ./...` build, pinned govulncheck (`GOTOOLCHAIN=go1.25.13`), installer tests, ShellCheck, and `git diff --check`.
- Two independent implementation reviews passed after the only finding (a shifted bundle-script line reference) was corrected. Final review: PASS, no acceptance blocker.

## Risky Files and Review Focus

| File | Risk | Review focus |
| --- | --- | --- |
| `frontend/pnpm-lock.yaml` | broad transitive churn or mixed peer snapshots | requested package set only; one React 19 graph; frozen install |
| `frontend/src/main.tsx` | patch loads after antd, or root behavior changes | side-effect import first; retain StrictMode/ErrorBoundary/createRoot |
| `frontend/src/test/setup.ts` | test environment diverges or node tests touch DOM | same bridge as production; registration only; DOM guards retained |
| React component/lifecycle tests | warnings hidden or timing assertions weakened | no global suppression; baseline warning comparison; assertions preserved |
| `frontend/scripts/check-bundle-size.mjs` | budget silently raised | update measured baseline only; keep 230,888 B |
| `.trellis/spec/backend/quality-guidelines.md` | dependency rules drift from manifest | React 19.2 + antd v5 patch, all other line locks unchanged |

## Stop Conditions

Return to planning instead of expanding scope if any of these is required:

- antd 6 or Vite 8 migration
- removal of StrictMode
- a browser-target increase
- bundle-budget increase
- product/API behavior change
- warning suppression or compiler/lint weakening
- unrelated lockfile deduplication
