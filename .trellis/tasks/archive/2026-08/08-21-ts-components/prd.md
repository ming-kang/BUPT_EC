# TS Components: all UI modules to .tsx

Child 3 of parent `08-21-frontend-typescript`. After this batch, `src/` production code is 100%
TypeScript; only prop-types removal remains (child 4).

## Goal

Convert the remaining 12 component/entry modules (+ co-located tests) to `.tsx/.ts` with zero
behavior change: icons, ToggleButtonGroup, CampusButtonGroup, BuildingPicker, ClassTimePicker,
TodayClassroomTable, CampusSettingsModal, GlobalEmpty, Footer, ErrorBoundary (class component),
App.jsx, main.jsx.

## Requirements

1. Mechanical conversion; prefer antd's own inferred prop types over hand-annotating wrappers;
   no props renames, no JSX restructuring.
2. Each converted component DROPS its PropTypes declaration (prop-types has no TS declarations →
   strict tsc fails on the import). The dependency itself stays until child 4. This continues the
   precedent set in child 2 (SelectionProvider).
3. App.jsx consumes hook return (`resp`, `spinning`, `reloading`, `isError`, `retry`) — types flow
   from useTodayClassrooms.ts without re-declaration; keep all lazy()/Suspense wiring identical.
4. main.jsx: root render typing; darkModeBootstrap import path already extensionless.
5. Tests convert with byte-identical assertions; minimal annotations where noImplicitAny demands
   (fixture builders, mock implementations). Component tests use jsdom + @testing-library —
   existing render/getBy typings apply automatically.
6. ErrorBoundary class component: type `Component<Props, State>` faithfully from its current
   fields; componentDidCatch/getDerivedStateFromError signatures per React 18 types.

## Acceptance Criteria

- [ ] `pnpm typecheck` strict green — src contains NO unchecked production JS anymore.
- [ ] `pnpm lint --max-warnings 0`, `pnpm test` (119) green.
- [ ] `pnpm build` green; bundle within budget.
- [ ] `find frontend/src -name '*.js' -o -name '*.jsx'` returns only test-less config files
      (expected: none under src).
- [ ] Test diffs vs HEAD are mechanical-only.
