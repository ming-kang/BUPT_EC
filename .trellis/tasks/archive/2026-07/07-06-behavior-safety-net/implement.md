# Behavior Safety Net Implementation Plan

## Pre-Implementation Gate

- The `GetData` error-envelope scope decision is resolved: this child covers the
  deterministic success envelope only, and defers deterministic error-envelope
  coverage to the injected HTTP boundary child task.
- Keep this as an inline implementation task unless a later step explicitly
  chooses sub-agent dispatch. The `implement.jsonl` and `check.jsonl` manifests
  are not required for the inline workflow.
- Do not add tests that are not tied to an approved regression risk.

## Implementation Checklist

### Backend

1. Add a fresh-cache short-circuit test in `service/realtime_data_test.go`:
   - seed a same-day unexpired `TodayClassrooms` value;
   - use a `mockJWClient` that records unexpected campus queries;
   - assert `GetTodayClassrooms` returns a non-stale copy and performs no JW
     campus query.
2. Add a JW fixture characterization test in `service/realtime_data_test.go`:
   - provide representative rows through `mockJWClient`;
   - exercise the current service refresh/query path;
   - assert only the public payload fields that protect normalization behavior.
3. Add deterministic `GetData` success-envelope coverage in `handler_test.go`:
   - seed the package-level `classroomService` with a same-day fresh cache;
   - call `GetData` through a Gin test router;
   - assert the public success envelope contains `code: 0` and `data`;
   - do not add a `GetData` error-envelope test in this child.

### Frontend

4. Add a minimal frontend test runner:
   - add the smallest Vitest-related dev dependencies needed for the approved
     tests;
   - add `pnpm test` to `frontend/package.json`;
   - avoid runner-only smoke tests.
5. Extract API response normalization from `frontend/src/useTodayClassrooms.js`
   into a pure helper module while preserving hook behavior.
6. Add focused frontend tests for:
   - successful API envelope normalization;
   - non-zero backend envelope normalization;
   - malformed payload errors;
   - selection reducer campus reset behavior;
   - selection preference persistence to `localStorage`.

### Documentation and Task Updates

7. Update this task's PRD/design if implementation evidence changes the approved
   test scope.
8. Update `CHANGELOG.md` only if the final changes are contributor-visible
   beyond internal test coverage and task documentation.
9. Update the parent task after completion with validation results and child
   status.

## Validation Plan

Run these commands after implementation:

```bash
gofmt -l .
go test ./...
cd frontend && pnpm lint
cd frontend && pnpm test
cd frontend && pnpm build
```

If `pnpm install` is required because new frontend dev dependencies are added,
run it before frontend validation and commit the updated lockfile.

## Validation Results

- `gofmt -l .` passed with no output.
- `go test ./...` passed.
- `go vet ./...` passed.
- `pnpm --dir "frontend" lint` passed.
- `pnpm --dir "frontend" test` passed: 2 test files, 7 tests.
- `pnpm --dir "frontend" build` passed. Vite emitted a third-party Rollup
  annotation warning from `rc-field-form`, but the production build completed.

## Risk and Rollback Points

- Backend test-only changes should be low risk and can be reverted per test.
- Frontend dependency additions affect `package.json` and `pnpm-lock.yaml`; if
  the test setup becomes larger than expected, stop and reassess rather than
  expanding the scope.
- API normalization extraction should be behavior-preserving. If the hook starts
  to change loading, retry, abort, or refresh-timer behavior, revert and split
  the extraction into a smaller step.
- Do not introduce the injected HTTP server abstraction in this child. That is
  the next planned child task.
