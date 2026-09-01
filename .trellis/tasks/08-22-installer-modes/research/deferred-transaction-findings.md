# Deferred Transaction Findings

The final installer review reconfirmed two transaction concerns that are intentionally outside `08-22-installer-modes` because this task preserves transaction function bodies against the recorded baseline hashes:

1. **Durable interruption recovery** — session backups live under the process temporary directory and cannot recover automatically after SIGKILL/power loss.
2. **Pre-damaged systemd state** — if the unit file is absent while its enablement link already exists, rollback branching keys off unit-file presence and may remove the restored pre-existing link.

Neither condition was introduced by source generation, configuration persistence, or mode dispatch. Fixing either requires changing rollback semantics and adding dedicated failure-state fixtures, so it should be planned as a separate transaction-recovery task rather than hidden inside this entrypoint refactor.
