# ShellCheck CI failure and fix — 2026-09-02

- Failed run: `33542355744` (`https://github.com/ming-kang/BUPT_EC/actions/runs/33542355744`) for HEAD `1cd87b25c63889ee7a0789d56d69ade15ce39f9a`.
- Go tests, frontend gates, embed build, govulncheck, generator/syntax checks, and Installer/CLI behavior suites passed. The quality job then failed only in recursive ShellCheck lint; `build-go` and `release` were skipped, so it did not publish a release or create release assets.
- CI reported `SC2317` on test-only function overrides in `scripts/cli_test.sh` and `scripts/installer_test/{modes,policy,render_config}.sh`. Those functions are deliberately invoked dynamically by sourced test subjects, which static reachability analysis cannot infer. Existing headers already suppressed the related dynamic-call warning `SC2329`.
- Fix: add targeted file-level `SC2317` suppressions next to the existing test-only ShellCheck suppressions. No production Installer/CLI behavior changes.
- The entire local preflight must be rerun after this correction, followed by a fresh normal `main` push and dry-run. Stable tag/release creation and nightly cleanup remain prohibited.
