# TS Cleanup: remove prop-types and finalize

Child 4 (final) of parent `08-21-frontend-typescript`.

## Goal

Remove the now-unused `prop-types` dependency, run the full parent acceptance sweep, and sync
documentation. No defensive-check removals beyond what is provably safe — with all modules typed
and every runtime boundary guard already verified by tests in children 1–3, this batch intentionally
does NOT delete any remaining `typeof`/`Array.isArray` guards (they remain load-bearing at network
boundaries; see parent design D3). Any such removal would need its own justification and test
changes, which contradicts the zero-behavior-change campaign goal — so none are performed.

## Requirements

1. `pnpm remove prop-types` (imports were already eliminated in children 2–3); lockfile updated.
Note: the lockfile keeps prop-types as a transitive dependency of eslint-plugin-react — only
our direct dependency entry is removed.
2. Full parent acceptance verification (below) + docs sync:
   - `docs/development.md`: frontend section mentions TypeScript where it describes file
     conventions/tooling; add typecheck to any gate lists.
   - AGENTS.md: frontend conventions line ("React components use PascalCase filenames ... hooks
     and ES modules") gains TypeScript mention if accurate.
   - Taskfile.yml / quality.yml already wired (child 1) — verify only.
3. No CHANGELOG entry unless something user-visible slipped (none did).

## Acceptance Criteria (parent final sweep)

- [x] src contains only .ts/.tsx; `rg PropTypes frontend/src` empty; `prop-types` absent from
      package.json dependencies (lockfile retains it only as a transitive dep of
      eslint-plugin-react / @types tooling — unavoidable while those are installed).
- [ ] `pnpm typecheck` strict; `pnpm lint --max-warnings 0`; `pnpm test` (119); `pnpm build`;
      bundle ≤ 230888 B.
- [ ] Taskfile check + quality.yml contain the Typecheck step (child-1 wiring intact).
- [ ] go build/vet/test green; embed build (`go build -tags embed_assets`) green.
- [ ] Docs synced as above.
