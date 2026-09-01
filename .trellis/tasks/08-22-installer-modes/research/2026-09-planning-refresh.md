# Installer Modes Planning Refresh — 2026-09-01

## Repository evidence

- `scripts/install.sh`: 1,218 lines; `scripts/install_test.sh`: 1,104 lines.
- `scripts/install.sh:65-77` (`require_installer_environment`) unconditionally checks `/dev/tty` before current `main`, so update cannot be non-interactive unless mode parsing precedes/splits this gate.
- `scripts/install.sh:79-97` loads only ten `CURRENT_*` values.
- `scripts/install.sh:611-635` renders only ten persisted values and uses eleven positional arguments including destination.
- Runtime/config docs support `LOG_CALLER` and `READYZ_DIAGNOSTICS`: `config/config.go:13-21,39-46`, `.env.example:7-12`, `docs/operations.md`.
- `scripts/install.sh:753` `prepare_staging` has thirteen positional arguments; test fixture `scripts/install_test.sh:389-396` repeats the render arguments.
- Release workflow copies `scripts/install.sh` directly into both architecture tarballs and the top-level release asset (`.github/workflows/release.yml:96-108`).
- CI currently runs `bash scripts/install_test.sh` and `shellcheck scripts/*.sh` (`.github/workflows/quality.yml:118-123`); nested source/test modules require recursive command updates.
- `docs/release.md:25` requires a self-contained published installer but does not require hand-maintained source to be one file.

## Planning conclusions

1. Parse mode before TTY gating; root remains universal, TTY becomes interactive-mode-only.
2. Preserve invocation environment before sourcing installed env so documented precedence is executable.
3. Use one twelve-field deployment configuration registry and `CFG_*` consumers; do not persist one-shot security flags.
4. Remove configuration positional-argument fan-out from render/staging APIs.
5. Maintain focused source fragments and deterministic generation of tracked `scripts/install.sh`.
6. Split the test harness while preserving existing scenario/assertion semantics and testing the generated artifact.
7. Add drift checks to local quality, CI, and release composition.

## Deferred audit findings

- Durable recovery after SIGKILL/power loss remains out of scope.
- Timeout comments, Safari compatibility, frontend decoder, and service shutdown findings belong to separate tasks.
