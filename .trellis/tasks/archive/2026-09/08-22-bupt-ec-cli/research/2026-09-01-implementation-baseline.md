# CLI implementation baseline — 2026-09-01

Baseline was captured before changing Installer/CLI sources.

## Verification

`task installer:check` passed on the starting worktree:

- generated Installer drift check;
- recursive Bash syntax check;
- 36 Installer behavior scenarios / 39 test functions;
- recursive ShellCheck.

## Compatibility evidence

- Local stable tags end at `v0.2.0` before this task.
- `git show v0.2.0:scripts/install.sh | grep -- --mode` returned no matches.
- `git ls-tree -r --name-only v0.2.0` has no `bupt-ec-cli` member/path.
- Current release workflow asserts tar members `bupt-ec`, `.env.example`,
  `README.md`, and generated `install.sh`; installer parity is checked by
  extracting the packaged `install.sh` and comparing it with
  `scripts/install.sh`.

## Generated source hashes

```text
scripts/install.sh                         19ac8ef123da0211e9fbc5dcb632903e90fb23df0bc06d9ede8aa80b9f10c9c2
scripts/installer/00-preamble.sh           8da9c0438d95c6e80986cdc5dc4321088d4b33a4cbebd484a60b4c74af58c35f
scripts/installer/10-config.sh             b63f89ebd694a1140ef56e44c4676e59135b7304020e21583fb7577971a1a636
scripts/installer/20-release.sh            fe33b2b5742146063e3db8452aacee19c9139d60f9399e062f90d39f3032e6d7
scripts/installer/30-render.sh             03bf4f96225b19b6a73a561e5058884bbdbdd93dd1ef7a99c65c178f39c2532d
scripts/installer/40-transaction.sh        ca3fa3b35356cfb2dbccb65993bbac5363ba1699f67e9f4225e5a04efed4e8a9
scripts/installer/50-main.sh               74242b976fdd5c4ded54b883224a2ebee863b6df9bdd4166d44f185a26ad8a56
scripts/install_test.sh                    4e2f099cc4be4d282cde71e710c6af564af82fe89a320aea14dbf90568e0b0cf
```

## Protected transaction function body hashes

Only `transaction_targets` and `commit_installation` are approved for changes.
The remaining function bodies must remain byte-identical:

```text
write_runtime_snapshot           3de7b1508361ca8685b62d51e38a1ee07aa735286e102f95fb6e1a82360c6913
read_runtime_snapshot_value      d1aab453fd277087722c5170768e68fe3429397583c78485a75a09c6b3794479
snapshot_installation            e216ba6f9d1445459aed700ead2300b6f4bee9eec6ec18f377cedec4557af9fb
atomic_install_file              7fc62a12cea8b4896104bab33ff9f63a613622b4117f8b4ceb0ba4b0785c632c
atomic_install_symlink           7cfb8b16c9cb556559d91ed2284af994c764c1ee8112ed2459c9f8f7a9641e84
restore_snapshot_target          b0e393b9c6d1dc9d5058eb96e57e90d62aa5736abfdfd26f0e0b6806a68332a1
rollback_installation            b4da0f5a0ce1d87d709ad667e705ad624455add4dc8773905597f8b03d97bb07
local_health_url                 7bf2aabda8097565373a0a7b893d739c083457e1b17112c34a99d1e846c8cfe1
wait_for_health                  c871b912c5380c86ef6210d103e8ea5d8c3595255d67d7a1f9d02b75f5da99c6
perform_install_transaction      2c3e9dc4121a1cbc0ed700d4c7528997f7c37a5a0a98bdbf8cd8ebb430e47d99
installer_cleanup                43a2fcd9fe14a25fded5a3a9569d4be0a2d47b1a63fdb360e12844dc73a635d3
initialize_installer_session     b8fd208076923c796d77a1670845228e5be51f674251ce8ab1783935873f1fa2
```

## Post-implementation hash verification

After the CLI/metadata integration, every protected body above was recomputed
and matched its baseline hash byte-for-byte. The only changed existing
transaction bodies are the approved `transaction_targets` and
`commit_installation`; `read_cli_staging_action` is the new action validator.

Final generated-source evidence:

```text
scripts/install.sh                         84a4e64609ba224f5d8d9d48c1d1314c3a88287804a0bf01831a163dddbc4463
scripts/installer/00-preamble.sh           7f6f99cbd72660ec41f9226c2f7983ae5e09465907016c9849cd1000d1b0e59e
scripts/installer/10-config.sh             b63f89ebd694a1140ef56e44c4676e59135b7304020e21583fb7577971a1a636
scripts/installer/20-release.sh            f91c1fd658a90e7d14009bfd88d9d13de32177b7e05e2597773fbf1566d1c41f
scripts/installer/30-render.sh             a176e299e9efa2548f884a4d7a525361c493921a4f1e19646b367440698dec45
scripts/installer/40-transaction.sh        49e14fc5fea887284afcf9988f47dfe8f7d6028275460c1f8a3b98653288f7e2
scripts/installer/50-main.sh               74242b976fdd5c4ded54b883224a2ebee863b6df9bdd4166d44f185a26ad8a56
```

## Independent final review

A fresh full-scope review after the pause found no critical/high/medium/low
findings and made no code changes. It independently confirmed all twelve
protected transaction bodies still match their `HEAD` baselines and ran:

```text
task check
task test
pnpm -C frontend build
node frontend/scripts/check-bundle-size.mjs
go build ./...
go build -tags embed_assets ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.10 .github/workflows/quality.yml .github/workflows/release.yml
GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
git diff --check
python ./.trellis/scripts/task.py validate 08-22-bupt-ec-cli
```

Bundle evidence: `164,862 B` gzip against the `230,888 B` budget. The host Go
`1.26.4` scan reports standard-library issues fixed by `1.26.6`; the repository's
pinned required `go1.25.13` scan passes with no vulnerabilities.
