# Classroom display contract: capacity semantics + building display_name

Audit items F-05 + F-07 (from `archive/2026-08/08-07-project-audit-optimize`). Pre-task research
lives in `research/display-contract-research.md` (data lineage, live-probe results, contract
impact enumeration).

## Goal

Settle the capacity-zero question with a live JW probe (DONE — see Background), then move
building-name normalization backend-side as `BuildingInfo.display_name`, following the existing
`RoomInfo.DisplayName` precedent. User-visible labels stay identical (主楼 stays 主楼); the change
is about single-sourcing the alias rules.

## Background (verified 2026-08-21)

- Capacity lineage: JW has no seat-count field; capacity is the trailing `(N)` of each CLASSROOMS
  token (`classroom_builder.go:12,142`, Atoi error discarded). A suffix-less token structurally
  yields capacity 0 AND lands in the 未分组 building (`:140`).
- **Live probe (integration test against real JW, 2026-08-21)**: 721 raw tokens across both
  campuses — suffix-less = 0, `(0)` = 0; built model: 52 rooms, zero-capacity = 0, ungrouped = 0.
  ⇒ The wire contract guarantees capacity ≥ 1; 0 can only appear as a parse-degradation artifact,
  where rendering 未知 is the desired fallback.
- Frontend aliasing is label-only: selection values / table filters use raw `name`
  (`BuildingPicker.tsx:38-39`; test asserts dispatch payload uses raw names).
- Same-day cache stores serialized payloads → post-deploy cached data lacks any new field until
  next refresh → frontend MUST fall back to `name`.

## Requirements

1. **F-05 (close by specification, no behavior change)**:
   - Permanent integration test (evolve the probe): assert every real JW CLASSROOMS token matches
     the room pattern and no `(0)` suffix exists — the executable form of the wire guarantee.
   - Spec/comment: document in `.trellis/spec/backend/api-contract.md` that `capacity` is always
     ≥ 1 on the wire; a 0 value is only possible as parse degradation for malformed tokens
     (which also land in 未分组), where frontend 未知 fallback is intended.
   - No change to `|| "未知"` rendering.
2. **F-07 (implement)**:
   - `service/model/realtime_data.go`: add `DisplayName string \`json:"display_name"\`` to
     `BuildingInfo`.
   - `service/classroom_builder.go`: populate it with the two rules moved from the frontend —
     alias map (未来学习大楼→主楼) and numeric-name 教N prefixing; keep `Name` raw.
   - `frontend/src/api/types.ts`: mirror the field.
   - `BuildingPicker.tsx`: label from `option.display_name ?? name` (fallback for stale cached
     payloads); delete BUILDING_ALIASES + displayBuildingName munging.
   - Tests: builder unit tests for the two rules; BuildingPicker label assertions unchanged in
     outcome (labels identical) but may now read through display_name; update any fixture that
     needs the new field.
3. CHANGELOG `[Unreleased]` Added entry (public JSON field addition per JSON Model Boundary rule).
4. Docs/spec sync: `api-contract.md` building field docs.

## Acceptance Criteria

- [ ] Integration test proves the token-shape guarantee live (skips without credentials like the
      existing suite).
- [ ] `go test -race ./...` green incl. new builder tests; frontend typecheck/lint/119+ tests green;
      build within budget.
- [ ] Rendered labels for current data are byte-identical to pre-change (主楼/教N rules preserved);
      selection payloads still carry raw names.
- [ ] Stale-cache fallback verified by a test (payload without display_name renders raw name).
- [ ] CHANGELOG + api-contract.md updated; rg sweeps show no BUILDING_ALIASES remnants in frontend.
