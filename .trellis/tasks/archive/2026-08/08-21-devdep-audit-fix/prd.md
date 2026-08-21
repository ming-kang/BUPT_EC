# Fix devDependency audit findings via lockfile patch bumps

## Goal

Clear the 3 remaining high-severity `pnpm audit` findings in the frontend
devDependency chain so that `pnpm audit:dev` (and therefore `task check`)
passes without manual skipping. Production dependencies are already clean
(`pnpm audit --prod` reports no known vulnerabilities); this task only
touches transitive dev-tool versions in `frontend/pnpm-lock.yaml`.

## Background

`pnpm audit` currently reports 3 high findings, all dev-only:

| Package | Installed | Patched | Introduced via | Fix within range? |
|---------|-----------|---------|----------------|-------------------|
| brace-expansion | 1.1.16 | >=1.1.18 (GHSA-rgw5-rvv9-x895) | eslint → minimatch@3.x | yes, minimatch declares `^1.1.7`, 1.1.18 published |
| js-yaml | 4.3.0 | 4.3.1 (CVE-2026-59870 advisory) | eslint → @eslint/eslintrc (`^4.1.0`) | yes, 4.3.1 published |
| nanoid | 3.3.16 | >=3.3.18 (GHSA-2v37-7h3g-55p8) | vite/vitest → postcss (`^3.3.x`) | yes, 3.3.18 published |

A fourth advisory (GHSA-mh99-v99m-4gvg) is already explicitly ignored via
`pnpm.auditConfig.ignoreGhsas` and is out of scope.

All three fixes are satisfiable inside existing semver ranges, so a scoped
lockfile refresh (`pnpm update <pkg>`) should suffice — no `pnpm.overrides`,
no major upgrades, no source changes.

## Requirements

1. Bump the three vulnerable transitive packages to patched versions by
   updating `frontend/pnpm-lock.yaml` only:
   - brace-expansion >= 1.1.18
   - js-yaml >= 4.3.1
   - nanoid >= 3.3.18
2. Do not change direct dependency ranges in `frontend/package.json`
   (no version bumps of vite/eslint/etc., no new overrides section).
3. Do not modify any application source, tests, or backend code.
4. If a bump turns out NOT to be resolvable within the declared ranges,
   stop and report back instead of forcing overrides — that would be a plan
   change requiring user consent.

## Acceptance Criteria

- [ ] `pnpm -C frontend audit` reports 0 vulnerabilities (only the one
      pre-existing ignored GHSA remains suppressed).
- [ ] `pnpm -C frontend audit:prod` passes.
- [ ] `pnpm -C frontend audit:dev` passes.
- [ ] `pnpm -C frontend lint` passes with zero warnings.
- [ ] `pnpm -C frontend test` passes (all existing tests green).
- [ ] `pnpm -C frontend build` succeeds and `pnpm -C frontend size` stays
      within budget.
- [ ] `git diff` shows only `frontend/pnpm-lock.yaml` (plus CHANGELOG entry
      if added) — no source changes.
- [ ] CHANGELOG: dev-only dependency bumps are not user-visible; add nothing
      under Unreleased unless the team convention requires it (default: no
      CHANGELOG change).

## Out of Scope

- Major-version upgrades of vite / eslint / vitest or any direct dependency.
- The broader audit backlog items (frontend-typescript migration,
  classroom-display-contract, utils-into-service, ETag/cold-path).
- Backend Go dependencies (`go.mod`) — separate concern, currently clean.
