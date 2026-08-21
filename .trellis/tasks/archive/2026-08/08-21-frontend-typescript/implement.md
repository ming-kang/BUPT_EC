# Implementation Plan — Frontend TypeScript Migration (Parent)

Execution is sequential child tasks 1→4. This file owns the cross-child checklist, validation
commands, and rollback points. Each child gets its own lightweight PRD when it starts; per-child
file lists live there, scoped by the map in prd.md.

## Ordered Checklist

### Child 1 — `ts-foundation` ✅ DONE (commit 023b73a, review 6/6 PASS)
- [ ] Create child task (`task.py create ... --parent`), write PRD, start.
- [ ] Add devDeps: `typescript`, `typescript-eslint` (parser+plugin); pnpm install.
- [ ] `tsconfig.json`: strict, noEmit, allowJs, bundler resolution, react-jsx, ES2020+DOM lib,
      include `src`.
- [ ] `package.json`: add `typecheck` script.
- [ ] Wire typecheck into `Taskfile.yml` check + `.github/workflows/quality.yml` frontend job.
- [ ] Extend `eslint.config.js` for ts/tsx (additive; config file itself stays JS).
- [ ] Create `src/api/types.ts` mirroring Go public structs (header: source-of-truth pointer).
- [ ] Convert pure helpers + their tests: classTimeUtils, classroomDataValidity, darkMode,
      darkModeBootstrap, apiError, campusSelection, reloadSchedule.
- [ ] Gates green → commit → archive child.

### Child 2 — `ts-data-layer` ✅ DONE (commit 48f854c, review PASS, zero findings)
- [ ] Create/start child with PRD.
- [ ] Convert todayClassroomsResponse (+test): `normalizeResponse(input: unknown)` keeps runtime
      guards; return type becomes the discriminated typed result (design D3).
- [ ] Convert selectionContext, SelectionProvider, useTodayClassrooms (+tests): type SWR data/
      error tracks; fetcher pair typed against api types.
- [ ] Gates green → commit → archive child.

### Child 3 — `ts-components` ✅ DONE (commit ef4ce29, review PASS after index.html fix)
- [ ] Create/start child with PRD.
- [ ] Convert leaf components first: icons, ToggleButtonGroup, CampusButtonGroup, BuildingPicker,
      GlobalEmpty, Footer, ErrorBoundary (class component), then ClassTimePicker,
      CampusSettingsModal, TodayClassroomTable, then App, main (+ all co-located tests).
- [ ] Prefer antd's own prop types; no behavioral props rewrites.
- [ ] Gates green (incl. jsdom component suites) → commit → archive child.

### Child 4 — `ts-cleanup-protypes`
- [ ] Create/start child with PRD.
- [ ] Delete any remaining PropTypes declarations/imports; remove prop-types from package.json;
      pnpm install to update lockfile.
- [ ] Drop defensive checks made redundant by types ONLY where a test proves behavior (design D3);
      each removal justified in the child PRD.
- [ ] Full parent acceptance sweep (below) → commit → archive child → archive parent.

## Validation Commands (every batch)

```bash
pnpm -C frontend typecheck          # after child 1 introduces it
pnpm -C frontend lint               # --max-warnings 0 via script
pnpm -C frontend test
pnpm -C frontend build              # then node scripts/check-bundle-size.mjs at batch end
go test -race ./...                 # backend untouched, but guards embed boundary
```

Final batch additionally: full `task check` equivalent + bundle budget + rg sweeps:
`rg -n "PropTypes" frontend/src` → empty; `find frontend/src -name '*.js' -o -name '*.jsx'` → only
absent (src only; root configs excluded).

## Risky Files / Rollback Points

| File | Risk | Mitigation |
|------|------|------------|
| useTodayClassrooms.js | SWR generics + refresh schedule logic, largest hook | own child (2), lifecycle tests travel with it |
| todayClassroomsResponse.js | contract boundary; wrong typing hides drift | keep runtime guards verbatim; tests assert both ok/false branches |
| ErrorBoundary.jsx | class component + window event typing | convert early in child 3, small surface |
| eslint.config.js / CI yml | gate misconfig blocks everything | additive edits only; run lint+typecheck locally before push |

Rollback = `git revert` the child's commit range; tree returns to last green batch.

## Follow-up Checks Before task.py start (per child)

1. Child PRD written and reviewed in-session.
2. implement.jsonl / check.jsonl curated with real entries (frontend spec layer if bootstrapped,
   else this parent's design.md + relevant audit research).
3. Previous child's gates confirmed green on HEAD.
