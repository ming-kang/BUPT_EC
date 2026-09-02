# Production canary decision — 2026-09-02

## User decisions

- No disposable/staging Linux environment is available.
- The first real-host v0.3.0 validation will run on the production server.
- A cloud VM/system-disk snapshot is available and selected as the external
  recovery mechanism.

## Accepted evidence gap

The user accepts that release-preflight evidence for clean installation,
intentional late failure rollback, first-install cleanup, and pre-v0.3
CLI/metadata removal/restore comes from repository Installer/CLI mocks and
release-layout simulation rather than an independent real Linux host.

Production is not a fault-injection environment. It will receive one normal
upgrade to the already-published immutable v0.3.0 release and only read-only
service/filesystem/version/health/UI checks. No damaged archive, forced service
failure, or deliberate direct Installer rollback to v0.2.x will be used as a
test.

## Risk controls

- A provider snapshot must reach an explicitly recoverable state before the
  stable tag is created; provider console and SSH access must also work.
- The GitHub v0.3.0 release and all four assets are verified before the host is
  changed.
- The existing nightly release/tag remains until production passes two healthy
  checkpoints at least five minutes apart, with the second no earlier than ten
  minutes after upgrade.
- A failed Installer transaction first uses its automatic rollback. An
  incomplete rollback or a post-success production defect uses the VM snapshot.
- A published v0.3.0 tag is never moved, deleted for reuse, or overwritten. A
  defect requires production restore and a new fix-forward version.
- Direct current-Installer rollback to v0.2.x is a secondary incident-recovery
  action only and requires fresh explicit authorization.

## Execution dependency

The runtime production hostname/access method, maintenance timing, snapshot
identifier/status, and saved release trust source must be established before
release execution. They are operational inputs and must not be committed with
credentials or private env contents.
