# Frontend TypeScript Migration (Parent)

## Goal

Audit item F-01: migrate `frontend/src` from JavaScript (+ prop-types) to strict TypeScript in
staged, independently verifiable batches, so that backend/frontend API contract drift is caught at
compile time, the zero-information PropTypes layer is deleted, and the codebase is ready for a
future React 19 upgrade (F-02, explicitly NOT part of this parent).

User value: earlier detection of contract drift between `service/model/realtime_data.go` and the
frontend consumer; better IDE/refactor safety; smaller dependency tree (prop-types removed).
Zero user-visible behavior change.

## Background (verified 2026-08-21)

- Scale: `frontend/src` = 38 files / 4,616 lines total; production code ≈ 2,600 lines across ~20
  modules; tests ≈ 2,000 lines. No `.ts/.tsx` files exist; no `tsconfig.json`/`jsconfig.json`.
- Runtime deps: antd 5.29.3, react 18.3.1, react-dom 18.3.1, swr 2.4.2, **prop-types 15.8.1**
  (imported by 10 files). `@types/react` + `@types/react-dom` (18.x) are ALREADY in devDependencies.
- Tooling: Vite 7 (esbuild transpiles TS natively), Vitest 3 (node env default, per-file jsdom
  directive), ESLint 9 flat config (`eslint.config.js`, JS parser only), pnpm 9.
- Import style: extensionless relative imports everywhere → renaming `.js(x)` to `.ts(x)` requires
  NO import-path updates under bundler module resolution.
- API contract source of truth: `service/model/realtime_data.go` public structs (envelope
  `{code,msg,data}` + `TodayClassrooms`/`CampusInfo`/`BuildingInfo`/`RoomInfo`/`NodeInfo`/
  `FreeTime`/`APIError`). The frontend already funnels all payload trust through ONE runtime
  validator: `normalizeResponse()` in `src/todayClassroomsResponse.js` (discriminated
  `{ok:true,resp}|{ok:false,reason}`, no throw-as-control-flow).
- Gates: CI quality.yml runs `pnpm lint` + `pnpm test`; bundle budget script caps dist at
  230,888 B gzip (current 210,394 B). TS types are erased at build → budget unaffected.
- Audit recommendation (backlog item 8): staged migration, no big bang; children split as
  类型生成 / 核心模块 / 组件层 / 删 prop-types; React 19 hangs at the end or as its own task.

## Requirements

- R1. Introduce TypeScript infrastructure: `tsconfig.json` (strict, bundler resolution, noEmit),
  a `typecheck` script wired into `package.json`, `Taskfile.yml` check, and CI quality.yml.
- R2. Add an `api-types.ts` mirroring the public Go API structs; document the Go file as source of
  truth. Payload boundaries stay runtime-validated (`normalizeResponse` keeps validating `unknown`
  input); types describe the POST-validation shapes.
- R3. Convert every `frontend/src` production module and its co-located test files from
  `.js/.jsx` to `.ts/.tsx` in dependency order (pure helpers → data layer → components), in
  separate reviewable batches, with zero behavior change: existing Vitest suites pass without
  assertion edits (mechanical import/rename changes only).
- R4. ESLint gains TS support (typescript-eslint parser+plugin in the existing flat config) and
  stays warning-clean (`--max-warnings 0`) throughout.
- R5. Remove `prop-types` from package.json and delete all PropTypes declarations once every
  component is converted.
- R6. Each batch lands as its own Trellis child task with green gates before the next starts.

## Child Task Map

| # | Child slug | Scope | Verifiable outcome |
|---|------------|-------|--------------------|
| 1 | `ts-foundation` | tsconfig + eslint TS support + typecheck wiring (CI/Taskfile) + `api-types.ts` + convert pure helper modules (classTimeUtils, classroomDataValidity, darkMode(+Bootstrap), apiError, campusSelection, reloadSchedule) | helpers are .ts, gates green incl. new typecheck |
| 2 | `ts-data-layer` | todayClassroomsResponse, selectionContext, SelectionProvider, useTodayClassrooms | SWR fetcher/envelope typed; hook tests green |
| 3 | `ts-components` | icons, ToggleButtonGroup, CampusButtonGroup, BuildingPicker, ClassTimePicker, TodayClassroomTable, CampusSettingsModal, GlobalEmpty, Footer, ErrorBoundary, App, main | all UI .tsx; component tests green |
| 4 | `ts-cleanup-protypes` | delete remaining PropTypes, remove prop-types dep, drop dead defensive checks made redundant by types (only where a test proves behavior), final full-scope check | prop-types gone; audit/dev gates clean |

Ordering: strictly sequential 1→4 (each builds on the previous). Detailed per-child PRDs are
written when each child starts (progressive elaboration); this parent owns cross-child acceptance.

## Acceptance Criteria (parent-level)

- [ ] `frontend/src` contains only `.ts/.tsx` sources (tooling configs `vite.config.js`,
      `eslint.config.js` excluded by design).
- [ ] `pnpm typecheck` (tsc --noEmit, strict) passes and runs in Taskfile check + CI quality.yml.
- [ ] `pnpm lint --max-warnings 0`, `pnpm test`, `pnpm build` all green at every batch boundary.
- [ ] Bundle size within 230,888 B budget after final build (expect unchanged ≈210 KB).
- [ ] `prop-types` absent from package.json dependencies; zero PropTypes imports remain.
- [ ] Zero behavior change: pre-existing test assertions unmodified (renames/imports excepted);
      no new runtime dependencies added.
- [ ] Docs synced: `docs/development.md` (frontend section), AGENTS.md if it references frontend
      file conventions, CHANGELOG only if any user-visible slip occurs (none expected).

## Out of Scope

- React 19 upgrade (F-02) — follow-up task after this parent archives.
- `capacity || "未知"` semantics (F-05) and building display_name (F-07) — separate
  classroom-display-contract task.
- antd bundle slimming (F-04), converting root tooling configs to TS, adding runtime schema
  validation libraries (zod etc.), reworking the normalizeResponse validation strategy.
- Any visual/UX change.

## Open Questions

None blocking — scope boundary (React 19 exclusion) ratified in planning summary 2026-08-21.
