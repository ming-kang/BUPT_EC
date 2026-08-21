# TS Foundation: toolchain, api types, pure helper modules

Child 1 of parent `08-21-frontend-typescript` (see its prd.md/design.md/implement.md for the
campaign-level decisions D1–D6). Scope here is limited to the foundation batch.

## Goal

Introduce the TypeScript toolchain (strict, wired into every gate), add the typed mirror of the
backend public API structs, and convert the seven dependency-free helper modules (+ their tests)
to `.ts` with zero behavior change.

## Requirements

1. DevDependencies: add `typescript` and `typescript-eslint`; no other dep changes.
2. `tsconfig.json`: strict, `noEmit`, `allowJs`, `moduleResolution: "bundler"`, `jsx: react-jsx`,
   target/lib ES2020 + DOM, include `src`. Root configs (`vite.config.js`, `eslint.config.js`)
   stay JS.
3. `package.json`: `"typecheck": "tsc --noEmit"` script; wire into `Taskfile.yml` check (next to
   frontend lint/test) and `.github/workflows/quality.yml` frontend job (step after lint).
4. `eslint.config.js` gains typescript-eslint support additively: recommended (non-type-checked)
   ruleset for ts/tsx, file pattern widened to `**/*.{js,jsx,ts,tsx}`; lint stays
   `--max-warnings 0` clean.
5. New `frontend/src/api/types.ts`: interfaces mirroring `service/model/realtime_data.go` public
   structs + API envelope (verify exact envelope fields incl. `log_id` in `handler.go` before
   writing); header comment names the Go file as source of truth ("update both" contract).
6. Convert to TypeScript (rename + type annotations only, NO logic edits): `classTimeUtils.js`,
   `classroomDataValidity.js`, `darkMode.js`, `darkModeBootstrap.js`, `apiError.js`,
   `campusSelection.js`, `reloadSchedule.js`, plus each one's co-located test file.
   - Extensionless relative imports mean zero import-path updates elsewhere.
   - Test assertions must remain byte-identical apart from mechanical changes (imports/renames);
     if a test seems to require an assertion change, stop and treat it as a defect.
7. No CHANGELOG entry (no user-visible change).

## Acceptance Criteria

- [ ] `pnpm -C frontend typecheck` passes (strict) and is enforced by Taskfile check + CI yml.
- [ ] `pnpm -C frontend lint` green at `--max-warnings 0` with the TS ruleset active.
- [ ] `pnpm -C frontend test` fully green; `git diff` of converted test files shows only
      mechanical changes.
- [ ] `pnpm -C frontend build` green; bundle size within budget (expect unchanged ≈210,394 B;
      run check-bundle-size.mjs once at the end).
- [ ] The 7 modules + tests exist only as `.ts` files (`rg -n "PropTypes" src/` still returns the
      pre-existing component hits — none of these helpers used PropTypes).
- [ ] Backend untouched: `go build ./...` / `go vet ./...` green (embed boundary intact).

## Out of Scope

- Data-layer/component conversion (children 2–3), prop-types removal (child 4), React 19,
  normalizeResponse rework.
