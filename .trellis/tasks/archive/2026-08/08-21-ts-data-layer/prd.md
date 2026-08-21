# TS Data Layer: response envelope, selection state, SWR hook

Child 2 of parent `08-21-frontend-typescript`. Riskiest batch: the API contract boundary and the
main SWR hook. Foundation (types.ts, strict tsconfig) landed in child 1.

## Goal

Convert the data layer to TypeScript with zero behavior change:
`todayClassroomsResponse` (+test), `selectionContext` (+test), `SelectionProvider.jsx`,
`useTodayClassrooms` (+2 test files).

## Requirements

1. `todayClassroomsResponse.ts`: `normalizeResponse(payload: unknown)` keeps EVERY runtime guard
   verbatim (typeof/Array.isArray/Number.isFinite) and its discriminated-result contract; the
   result becomes typed: `{ ok: true; resp: NormalizedEnvelope } | { ok: false; reason: string }`
   where `NormalizedEnvelope = { code: number; msg: string; data: TodayClassroomsData | null }`
   (post-validation shape — note `msg` is always present after normalization, unlike the wire).
   `classroomWarningMessage`, `extractMessage`, `readJson`, `loadingResponse`,
   `fallbackErrorMessage` typed faithfully.
2. `selectionContext.ts` / `SelectionProvider.tsx`: reducer + context + provider typed; campus
   selection state uses `CampusInfo`-compatible shapes where the code already assumes them;
   runtime guards stay where they exist today.
3. `useTodayClassrooms.ts`: SWR hook typed against the normalized envelope; fetcher pair, refresh
   schedule integration (`nextReloadDelay`), error track (`ApiError`), and returned object shape
   unchanged. No changes to scheduling logic, retry semantics, or state transitions.
4. Tests convert with byte-identical assertions; only mechanical diffs allowed (imports, renames,
   minimal annotations where noImplicitAny demands). The 607-line lifecycle suite must pass
   unmodified apart from such mechanics.
5. No new deps, no CHANGELOG, backend untouched.

## Acceptance Criteria

- [ ] `pnpm typecheck` (strict) green with all five modules as `.ts/.tsx`.
- [ ] `pnpm lint --max-warnings 0`, `pnpm test` (119 tests) green.
- [ ] `pnpm build` green; bundle within budget.
- [ ] Test diff vs HEAD shows only mechanical changes; zero assertion edits.
- [ ] `rg -n "todayClassroomsResponse.js|selectionContext.js|useTodayClassrooms.js" frontend/src`
      empty (extensionless imports elsewhere keep working).
