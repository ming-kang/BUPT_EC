# v0.3.0 production success and nightly cleanup — 2026-09-02

## Operator-supplied production evidence

The operator ran the documented direct current-Installer transition on the existing v0.2.x production host with explicit `VERSION=v0.3.0` and `--mode=update`.

The supplied terminal output established:

- official `ming-kang/BUPT_EC` v0.3.0 amd64 download;
- successful package checksum verification;
- successful Nginx configuration validation;
- one transient loopback `curl: (7)` before the service bound port 8080;
- `BUPT_EC update completed`, version v0.3.0, and the expected public URL;
- `bupt-ec version` reported configured selector, running version, and CLI version all as v0.3.0;
- a subsequent no-argument CLI update reused v0.3.0 and completed successfully, demonstrating the installed CLI delegation path.

The supplied browser screenshot showed the production UI loading current classroom data and the settings row `当前运行版本：v0.3.0`. Together with the CLI output and public release, this establishes the intended release/running/UI version consistency for the normal production path.

The operator then explicitly confirmed that production was running normally with no issue and stated that no backup or rollback was needed. Formal VM snapshot/recovery evidence, deep downloaded-asset revalidation, target permission inspection, and the originally planned timestamped two-checkpoint observation were not produced. These remain accepted evidence gaps and must not be described as completed checks.

## Nightly cleanup

After that explicit success confirmation and cleanup instruction:

1. deleted the GitHub `nightly` prerelease;
2. deleted remote `refs/tags/nightly`;
3. deleted local `refs/tags/nightly` (previously `8cd4594f08c3461b0320ecb7ca0ad5badda68728`);
4. confirmed GitHub release lookup, remote tag lookup, and local tag lookup no longer find nightly;
5. confirmed v0.3.0 remained GitHub Latest immediately after cleanup.

No stable release/tag was deleted or moved. No production command was run during cleanup.

## Inline follow-up

The transient curl line is not a failed deployment: `wait_for_health` retries up to ten times and the transaction reports success only after a later successful probe. It is nevertheless misleading operator output. Per explicit user instruction, the smallest stderr-suppression regression fix and immutable v0.3.1 publication are folded into this active parent task without creating a new Trellis task. Production will not be automatically upgraded to v0.3.1.
