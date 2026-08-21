# Design — Frontend TypeScript Migration (Parent)

## Architecture & Boundaries

This is a mechanical language migration with one new architectural seam (typed API models). No
runtime architecture changes: same components, same SWR flow, same bundle chunks.

```text
service/model/realtime_data.go   ← source of truth (Go structs, JSON tags)
        │  (manual mirror, doc-anchored)
        ▼
frontend/src/api/types.ts        ← NEW: public API types (envelope + TodayClassrooms tree)
        │  imports
        ▼
todayClassroomsResponse.ts       ← boundary: unknown → runtime validation → typed resp
        │
useTodayClassrooms.ts            ← SWR hook, typed data/error tracks
        │
components/*.tsx                 ← consume typed shapes; antd props from antd's own types
```

## Key Decisions

### D1. Handwritten `api-types.ts`, not codegen (tygo/quicktype)
8 public structs, stable API, single consumer. Codegen adds a Go tool dependency + generation step
+ CI drift check for marginal benefit; the existing response-handling tests already pin the wire
shape behaviorally. File carries a header comment pointing at the Go file with a "update both"
contract. Revisit tygo only if the API surface grows materially.

### D2. strict from day one; skip the checkJs phase
The audit's original sketch (`allowJs` + gradual `checkJs`) would mean JSDoc-typing files that are
about to be renamed — throwaway work for a 20-module codebase. Instead: `tsconfig.json` with
`strict: true`, `allowJs: true` (so mixed batches compile mid-migration), `noEmit: true`
(Vite/esbuild owns emit), `moduleResolution: "bundler"`, `jsx: "react-jsx"`, lib
ES2020 + DOM. JS files simply aren't type-checked until converted; strictness applies to every
`.ts/.tsx` the moment it lands.

### D3. Types describe post-validation shapes; boundaries stay runtime-checked
`normalizeResponse(input: unknown)` keeps every existing `typeof`/`Array.isArray` guard and its
discriminated-result contract; its return type becomes
`{ok:true, resp: ApiEnvelope<TodayClassrooms>} | {ok:false, reason:string}`. Downstream modules
consume typed shapes without re-checking. Redundant defensive checks INSIDE already-validated
regions may only be dropped in child 4, and only where a test proves the behavior.

### D4. Conversion order = dependency order, tests travel with their subject
Pure helpers → data layer → components (see prd.md child map). Each test file converts in the same
batch as its subject so suites never straddle a batch boundary. Extensionless relative imports mean
renames require zero import-path edits.

### D5. ESLint: additive TS support in the existing flat config
Add `typescript-eslint` (parser + plugin, recommended, non-type-checked ruleset to keep lint fast —
`tsc --noEmit` is the type gate, lint stays structural). File pattern widens to
`**/*.{js,jsx,ts,tsx}`; react/react-hooks plugins apply to tsx unchanged. `eslint.config.js` itself
stays JS (ESLint TS-config support adds tooling risk for zero value).

### D6. typecheck wiring mirrors the existing gate topology
`package.json`: `"typecheck": "tsc --noEmit"`. Wired into `Taskfile.yml` check (next to
frontend lint/test) and `.github/workflows/quality.yml` frontend job (new step after lint), keeping
the Taskfile↔CI sync rule stated in Taskfile comments.

## Compatibility / Migration Notes

- Mixed JS+TS compilation is supported by Vite/Vitest out of the box; each child leaves the tree
  green, so any batch can ship independently and rollback = revert that child's commit(s).
- antd v5 ships its own types; component conversion should prefer antd's inferred prop types over
  hand-annotating wrappers.
- Known friction points: ErrorBoundary class component (`Component<Props, State>`), SWR generics on
  the fetcher pair in useTodayClassrooms, `lazy()` import typing in App.jsx, jsdom test utilities
  (render/getBy* typings from @testing-library/react, already version-matched).
- prop-types removal happens LAST (child 4) so intermediate batches never break unconverted
  siblings that still declare PropTypes.

## Trade-offs Accepted

- Handwritten types can silently drift from Go if someone changes the API without reading the
  header contract — mitigated by response tests + the spec's JSON-model-boundary rule that already
  forces cross-layer sync on contract changes.
- Non-type-checked eslint ruleset misses some inference-based catches; bought for lint speed and
  a smaller rule-surface diff. Can tighten later without migration risk.

## Rollback Shape

Per-child commits on main; each child is independently revertible. The parent archives only after
child 4 passes the full parent acceptance list. If a batch stalls, later children simply don't
start — the tree stays green at the last completed batch.
