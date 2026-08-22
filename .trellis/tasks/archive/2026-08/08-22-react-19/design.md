# Design — React 19 Compatibility Migration

## Scope and Boundaries

This is a single, medium-sized frontend dependency migration. Runtime architecture, API contracts, state ownership, component structure, chunk topology, and browser target remain unchanged.

```text
frontend/src/main.tsx
  ├─ side-effect import: @ant-design/v5-patch-for-react-19
  │    └─ registers antd v5 dynamic rendering through react-dom/client.createRoot
  └─ existing React root: createRoot(#root) → StrictMode → ErrorBoundary → App

frontend/src/test/setup.ts
  └─ jsdom-only compatibility import for component tests that bypass main.tsx
```

The production patch import is required because antd v5's Button wave and static overlay render paths otherwise use removed React DOM behavior. The test setup awaits the same patch only when `window` exists, so direct jsdom component tests use the production bridge while pure node tests avoid loading the antd graph.

No parent/child split is warranted: package/lockfile resolution, the entrypoint bridge, compatibility fixes, docs, and validation form one atomic, independently releasable outcome. Partial delivery (for example React 19 types with a React 18 runtime) is invalid.

## Dependency Contract

Target the stable registry versions observed during planning:

```json
{
  "dependencies": {
    "@ant-design/v5-patch-for-react-19": "^1.0.3",
    "react": "^19.2.8",
    "react-dom": "^19.2.8"
  },
  "devDependencies": {
    "@types/react": "^19.2.18",
    "@types/react-dom": "^19.2.4"
  }
}
```

Rules:

1. Edit all runtime, declaration, and patch ranges together, then regenerate the lockfile with one pnpm 9.15 resolve; never leave a mixed runtime/type manifest.
2. Keep `react` and `react-dom` on matching versions; resolve one runtime instance only.
3. Keep antd `^5.29.3`, Vite `^7`, Vite React plugin `^5`, Vitest `^3`, SWR, and Testing Library on their existing lines unless a reproduced incompatibility requires the smallest supporting update.
4. Do not run `pnpm update --latest` or `pnpm dedupe`; the React 18 baseline already has unrelated dedupe opportunities, and accepting them would destroy migration attribution.
5. Keep the explicit Vite browser target (`es2020`, Safari 14) and existing `react-vendor` / `antd-vendor` chunk boundaries.

## Key Decisions

### D1. Stable React 19.2, not canary/RC/backport

The npm `latest` runtime is 19.2.8 and the selected React DOM package peers on that exact runtime line. Using stable latest keeps the migration on the supported ecosystem path; canary/experimental tags add no product value.

### D2. Official antd v5 patch, not antd 6 or a local renderer

Ant Design explicitly recommends `@ant-design/v5-patch-for-react-19` for v5 applications. The package supports antd `>=5.22.6` and React/DOM `>=19`, so the current antd 5.29.3 qualifies. A custom `unstableSetRender` copy would duplicate upstream compatibility code; upgrading to antd 6 is a separate major migration and remains out of scope.

The patch is a production dependency because it executes in the shipped browser bundle. Import it before application rendering. Tests that mount antd directly also await the patch from `frontend/src/test/setup.ts` in jsdom so wave/overlay behavior is exercised under the production bridge without slowing pure node suites.

### D3. Compatibility-only source changes

The repository already uses the new JSX transform, `createRoot`, explicit children types, initialized refs, and Testing Library's modern APIs. Do not preemptively rewrite hooks or components. Run typecheck and focused runtime tests after dependency resolution, then change source only for a demonstrated React 19 type/runtime failure.

Forbidden workarounds:

- weakening TypeScript or ESLint settings
- broad `any`/casts solely to silence React 19 types
- global console-warning suppression
- removing StrictMode
- adopting new React 19 product APIs during the migration

### D4. Compare warning classes, not a false zero-warning baseline

The React 18 baseline already emits one Node `TimeoutNaNWarning` and jsdom pseudo-element `getComputedStyle` messages. They are unrelated and remain outside scope. The migration gate is no **new** React/ReactDOM/antd compatibility, `act`, ref, root, or unmount warning. Existing targeted spies in error-boundary/lifecycle tests stay narrow.

### D5. Preserve bundle budget and document the new baseline

The current 2026-08-22 pre-migration total is 209,594 B gzip under the repository's gzip-9 metric; the fixed budget remains 230,888 B. The existing script separately records the historical 2026-07-27 budget basis of 209,898 B, which must be preserved rather than overwritten. The patch is expected to land in the existing antd vendor chunk and React 19 in the existing React vendor chunk; verify those chunk boundaries from the generated build, record a distinct post-migration measurement, and do not raise the budget.

## Compatibility and Migration Notes

- React 19 type packages may expose errors hidden by React 18 declarations even though source APIs are modern. Address these one at a time and retain runtime behavior.
- `skipLibCheck` is an existing project setting; this migration neither weakens nor expands it. A one-off full library check may be used diagnostically but is not a new permanent gate.
- The official antd patch registers global render behavior. Loading it in the application entry and conditionally in the isolated jsdom setup is intentional: those are separate module graphs, and component tests do not import `main.tsx`.
- No backend JSON, endpoint, cache, or deployment configuration changes.
- Existing same-day cache, selection persistence, Modal behavior, accessibility assertions, ErrorBoundary behavior, and SWR lifecycle tests are the regression contract.

## Documentation and Spec Synchronization

Update only migration-relevant current documentation:

- `CHANGELOG.md` `[Unreleased]` dependencies: React 18.3 → 19.2 and the antd v5 compatibility bridge.
- `.trellis/spec/backend/quality-guidelines.md`: change the deliberate React dependency line from `^18.3` to `^19.2`, retain antd 5 and other line locks, and add the required patch/import rule.
- `frontend/scripts/check-bundle-size.mjs`: preserve the 230,888 B budget and record the measured React 19 baseline.
- Fix the explicitly inventoried stale current-source extensions in `docs/development.md`, the quality/API/directory specs, `main.tsx`, and the ErrorBoundary test comment; archived task/audit evidence remains historical.

No API or runtime-state behavior changes are needed. A separate stale success-`log_id` statement discovered in `api-contract.md` predates this task and is deferred rather than silently folded into the React migration.

## Rollout and Rollback

Delivery is one scoped commit after all checks pass. Rollback is one revert restoring React 18 runtime/types, removing the antd patch and its imports, and restoring dependency documentation. No data migration, configuration migration, or server rollback coordination is required.

If a blocking third-party incompatibility appears:

1. prove it with a minimal failing test/output;
2. try the smallest compatible update inside the existing dependency major line;
3. if resolution requires antd 6, Vite 8, weakened gates, or product behavior change, stop and return to planning rather than expanding scope silently.
