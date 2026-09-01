# Main release dry-run failure and fix — 2026-09-02

- Failed run: `33540870129` (`https://github.com/ming-kang/BUPT_EC/actions/runs/33540870129`) for pushed HEAD `4f60f8cdf15babe54ef654928d9858054c07665c`.
- The reusable quality job failed at `go test -race ./...`; `build-go` and `release` were skipped, so no release was published and no release-assets artifact was available to validate.
- Cause: three cache fixtures used host-local `time.Now().Format("2006-01-02")`, while production compares cache dates in `Asia/Shanghai`. At 17:59 UTC the runner was already the next Shanghai day, so same-instant fixture dates were incorrectly treated as cross-day and then caused stale-refresh assertions to fail.
- Fix: the affected cache-policy/realtime tests now seed dates using `time.Now().In(businessLocation)`, matching the service's documented business-day contract. No production behavior changed.
- The full local release-preflight matrix was rerun after the fix: `task check`, `task test`, `task installer:check`, fresh frontend build/size (164,862 B gzip), tagless/embed builds, pinned govulncheck, pinned actionlint, diff check, and parent task validation all passed.
- The next normal `main` push must be monitored as a new non-publishing release dry-run. Stable tag/release creation and nightly cleanup remain prohibited.
