# React 19 Compatibility Research

Date: 2026-08-22

## Decision Summary

Migrate this repository as one medium-sized compatibility task:

- `react` / `react-dom`: stable `19.2.8` line (matching runtime versions)
- `@types/react`: stable `19.2.18`
- `@types/react-dom`: stable `19.2.4` (peers on `@types/react ^19.2.0`)
- keep antd on the existing v5 line (`5.29.3`)
- add the official runtime compatibility package `@ant-design/v5-patch-for-react-19@^1.0.3` and import it at the application entry before rendering
- do not upgrade Vite, Vitest, SWR, Testing Library, or antd unless an observed post-upgrade failure proves a supporting update is necessary

No product decision is blocked. This is a behavior-preserving dependency migration, not adoption of React 19 features.

## Repository Baseline

### Runtime and source

- `frontend/package.json:18-30` declares React/React DOM `^18.3.1` and React 18 type packages.
- `frontend/src/main.tsx:1-30` already uses `react-dom/client.createRoot` and `React.StrictMode`; no legacy root migration is needed.
- `frontend/tsconfig.json` uses the modern `react-jsx` transform and strict TypeScript.
- A repository-wide source/test audit found no local use of removed React 19 APIs or migration hazards: no `ReactDOM.render`, `hydrate`, `findDOMNode`, string refs, callback-ref cleanup assumptions, `defaultProps`, `propTypes`, `react-dom/test-utils`, global `JSX.*`, or `react-test-renderer`.
- The only local `useRef` (`frontend/src/SelectionProvider.tsx`) already supplies an initial value. Component children are explicitly typed, and the class error boundary uses supported APIs.
- Tests use `@testing-library/react`; lifecycle tests import its `act`, not the removed/deprecated test-utils surface.

### Dependency graph

Current installed metadata and `frontend/pnpm-lock.yaml` resolve one React 18.3.1 / React DOM 18.3.1 runtime. Relevant current peer ranges:

| Dependency | Current | React compatibility |
| --- | ---: | --- |
| antd | 5.29.3 | `react` / `react-dom >=16.9.0` |
| SWR | 2.4.2 | React `^16.11 || ^17 || ^18 || ^19` |
| `use-sync-external-store` | 1.6.0 | React `^16.8 || ^17 || ^18 || ^19` |
| Testing Library React | 16.3.2 | React/DOM and type peers `^18 || ^19` |
| Vite React plugin | 5.2.0 | no React peer; supports the repository's Vite 7 line |

The npm registry's stable tags on 2026-08-22 are React/React DOM `19.2.8`, `@types/react 19.2.18`, and `@types/react-dom 19.2.4`. `react-dom@19.2.8` peers on `react ^19.2.8`; the selected versions satisfy that pair exactly.

### Ant Design v5 compatibility requirement

Ant Design's v5 React 19 guide states that React 19's `react-dom` export changes break antd v5's dynamic render path: button wave effects do not work and static Modal/Notification/Message methods fail. The guide recommends the compatibility package over a custom `unstableSetRender` implementation:

- upstream guide: `ant-design/ant-design`, branch `5.x-stable`, `docs/react/v5-for-19.en-US.md`
- package: `@ant-design/v5-patch-for-react-19@1.0.3`
- peer requirements: antd `>=5.22.6`, React/DOM `>=19.0.0` — all satisfied here
- usage: side-effect import `@ant-design/v5-patch-for-react-19` at the application entry

The app does not call static Modal/Notification/Message APIs, but it does render and click antd Buttons, so the wave-effect path makes the patch relevant. Staying on antd 5 avoids folding a separate antd 6 migration into this task.

## React 18 Baseline Gates

Run from `frontend/` before any dependency edits:

- `pnpm typecheck`: pass
- `pnpm lint`: pass
- `pnpm test`: 18 files / 120 tests pass
- `pnpm build`: pass, 1,491 modules transformed
- `pnpm size`: **209,594 B gzip** / 230,888 B budget
- `pnpm audit:prod`: no known vulnerabilities
- `pnpm audit:dev`: no known vulnerabilities

Known pre-migration test noise, to distinguish from React 19 regressions:

- one Node `TimeoutNaNWarning` in the lifecycle suite
- jsdom's `Not implemented: Window's getComputedStyle() method: with pseudo-elements` during existing antd Modal tests

These warnings predate React 19. The migration must introduce no new React, React DOM, antd compatibility, `act`, ref, or unmounted-root warnings; removing the unrelated baseline noise is not part of this task.

`pnpm dedupe --check` is also non-green on the React 18 baseline because it proposes broad, unrelated transitive deduplication (`csstype`, ES shims, and `rc-virtual-list`). It is therefore a diagnostic only, not an acceptance gate for this migration; running `pnpm dedupe` would violate the minimal-churn scope.

## Expected Code Impact

Mandatory expected changes:

1. `frontend/package.json` and `frontend/pnpm-lock.yaml` for the four React runtime/type packages plus the official antd v5 patch.
2. `frontend/src/main.tsx` to load the compatibility patch before application rendering; retain `createRoot` and StrictMode.
3. `frontend/src/test/setup.ts` to await the same antd renderer only in jsdom for direct component tests, which do not import the application entrypoint; pure node tests skip the antd graph.
4. Narrow source/test adaptations only if React 19 types or runtime output demonstrate a real incompatibility.
5. Changelog, dependency-line spec, and distinct historical/current/post-migration bundle-baseline documentation synchronization.

No local source migration is currently predicted for hooks, context, refs, error boundaries, portals, or tests. Existing component, Modal, picker, error-boundary, and SWR lifecycle suites provide the behavioral regression net.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Partial runtime/type upgrade creates mixed React peer snapshots | Upgrade all four packages in one pnpm operation; frozen install; inspect `pnpm list` / `pnpm why`; reject React 18 lock entries and duplicate runtimes. |
| antd v5 dynamic rendering loses waves or warns under React 19 | Install/import the official v5 patch; exercise real Button and Modal tests; reject new compatibility warnings. |
| React 19 type changes reveal hidden assumptions | Run typecheck immediately after resolve; fix only demonstrated errors without broad casts or compiler-rule weakening. |
| StrictMode/effect timing changes expose lifecycle bugs | Run the existing lifecycle and component suites first, then all 120+ tests. |
| Dependency change exceeds bundle budget | Build and measure using the repository's gzip-9 script; do not raise 230,888 B as part of this task. |
| Unrelated major updates obscure attribution | Keep antd 5, Vite 7/plugin 5, Vitest 3, and current SWR/Testing Library lines unless a concrete incompatibility requires the smallest supporting bump. |

## References

- React 19 upgrade guide: <https://react.dev/blog/2024/04/25/react-19-upgrade-guide>
- Ant Design v5 React 19 guide source: <https://github.com/ant-design/ant-design/blob/5.x-stable/docs/react/v5-for-19.en-US.md>
- Original project audit: `.trellis/tasks/archive/2026-08/08-07-project-audit-optimize/research/audit-frontend.md` (F-02)
- Dependency and validation contract: `.trellis/spec/backend/quality-guidelines.md`
