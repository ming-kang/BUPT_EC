# 执行计划：v0.3.0 最终集成、发布与 nightly 下线

任务：`08-22-ops-experience`
状态：planning

## 0. 规划与远程基线

- [x] 确认四个子任务均已归档，父任务是唯一活跃 Trellis 任务。
- [x] 记录本地 `main`、`origin/main`、`v0.3.0`、`nightly` tag/release 与 GitHub
  authentication 的只读基线。
- [x] 确认 `scripts/release.sh` 要求 clean `main` 且 `HEAD == origin/main`，并记录其
  release commit/tag/push 及本地 rollback 契约。
- [x] 记录用户决定：暂不提供真实 Linux E2E 环境；该 gate 默认阻断 stable tag push。
- [x] 完成 PRD convergence、design、implement、真实 implement/check context，并通过
  `task.py validate`。
- [x] 向用户提交最终规划摘要；用户随后明确批准启动执行。

## 1. 本地跨子任务集成审计

- [x] 读取四个 archived child 的 PRD/design/implement/check 结果和工作/归档提交，建立
  acceptance-to-evidence 表，不重复实现已归档功能。
- [x] 复核 parent 八项跨子任务验收：默认 Installer 兼容、零问答 update、单稳定轨道、
  版本一致、事务完整、文档同步、质量门与 CHANGELOG。
- [x] 搜索 `nightly` 全仓匹配并分类：允许 released CHANGELOG 历史；拒绝 production
  workflow/script/current docs/config 路径或仍推广 nightly 的文案。
- [x] 提取 `[Unreleased]` 并逐项映射四个子任务；确认 `Added`/`Changed`/`Removed`、pre-v0.3
  fallback 与存量 nightly 迁移说明完整。
- [x] 验证 release composition source、generated Installer、CLI build marker、Go ldflags、
  readyz/UI version flow 与 exact four-asset contract 一致。
- [ ] 如发现真实缺陷，停止发布准备，修复 owning contract，更新 docs/CHANGELOG/spec，
  并从本步骤重新开始。

## 2. 本地全量发布前门禁

按顺序运行并保存摘要：

```bash
task check
task test
task installer:check
pnpm -C frontend build
pnpm -C frontend size
go build ./...
rm -rf web/dist && cp -r frontend/dist web/dist
go build -tags embed_assets ./...
GOTOOLCHAIN=go1.25.13 go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
actionlint .github/workflows/ci.yml .github/workflows/quality.yml .github/workflows/release.yml
git diff --check
python ./.trellis/scripts/task.py validate 08-22-ops-experience
```

- [x] `task check` 全绿，包括 Go/frontend/Installer/CLI/release-layout/ShellCheck。
- [x] `task test` race suite 全绿。
- [x] fresh frontend build 与 bundle budget 全绿。
- [x] tagless/embed builds 全绿。
- [x] pinned Go 1.25.13 vulnerability scan 无 reachable finding。
- [x] actionlint 与 whitespace checks 全绿。
- [x] 生成物无 drift，工作区无 credentials/logs/build artifacts/未知脏文件。

**Gate**：任一失败时不得 push `main`。

## 3. 集成证据提交

- [x] 更新 parent PRD acceptance checkboxes，只勾选已由证据满足的项；E2E 相关项保持未完成。
- [x] 将本地审计摘要写入 task research/review 文件，不写 secrets 或大段测试日志。
- [x] 若仅 task artifacts 变化，使用 scoped Trellis/planning commit；若修复产品缺陷，使用
  独立 Conventional Commit 并保留 CHANGELOG 同步。
- [x] 提交后确认 clean `main`，再次运行 `git diff --check` 与 generator drift check。

## 4. Push main 与 GitHub dry-run

这是第一个远程 mutation gate。只有在最新规划获批准且步骤 1–3 全绿后执行。

- [x] `git fetch --force origin main --tags`，确认没有意外 remote divergence。
- [x] push 当前 `main` 到 `origin/main`；不得 force push。
- [x] 使用 `gh run list/view/watch` 锁定该 HEAD 对应的 `release.yml` run。
- [x] 确认 quality-gate、build-go、release dry-run/attestation 全部成功。
- [x] 下载或检查 workflow artifacts：两个 tarball、`checksums.txt`、`install.sh`；验证
  exact layout、checksums、Installer parity 与 `main-<short-sha>` version injection。
- [x] 确认该 main run 未创建 GitHub release，且远程仍无 `v0.3.0` tag/release。
- [x] 记录 run URL/id、HEAD 与验证摘要。

**Rollback**：dry-run 失败时不创建 stable tag；在 `main` 上 fix forward 并重新通过完整
本地门和新的 GitHub dry-run。

## 5. 真实 Linux E2E（当前延期 / blocking）

- [ ] 获得可销毁或明确授权的 Linux 环境；记录发行版和必要工具版本，不记录访问秘密。
- [ ] 清洁首装 current/latest Installer，验证真实路径、owner/mode、systemd、Nginx、CLI
  与 metadata。
- [ ] 验证非 root `status`/`version`/`health`、root `config show` secrecy 与 UI/readyz/CLI
  version consistency。
- [ ] 执行零 prompt `bupt-ec update`，验证完整配置保留和 transaction success。
- [ ] 验证 `bupt-ec update v0.2.x` 在 curl 前拒绝。
- [ ] 用 current/latest Installer direct fallback 验证 pre-v0.3 transaction removal/rollback
  契约（基于可用 immutable release/fixture，禁止污染生产主机）。
- [ ] 保存不含凭据的 E2E 结果，清理 VM/测试部署。

**当前决定**：用户暂时不测试。因此执行到这里必须暂停；不得默认勾选、不得用 mocks
冒充真实 E2E，也不得继续 stable tag push。若用户以后明确接受跳过风险，先更新 PRD/design/
implement 并重新提交最终规划摘要。

## 6. v0.3.0 release preflight

仅在步骤 5 完成或经过新的明确风险豁免后执行：

- [ ] fetch 后确认 clean `main == origin/main`。
- [ ] 确认最新 main dry-run workflow 对该 HEAD 全绿。
- [ ] 确认本地/远程均无 `v0.3.0` tag/release，`v0.2.0` 仍是 Latest，nightly 仍可用。
- [ ] 再跑 release-critical subset：generator drift、Installer/CLI/layout suites、CHANGELOG
  extraction、actionlint、`git diff --check`。
- [ ] 人工审阅 `scripts/extract-changelog.sh Unreleased` 输出作为最终 release notes。

## 7. 创建并发布 v0.3.0

- [ ] 运行 `scripts/release.sh v0.3.0`。
- [ ] 检查 `chore: release v0.3.0` 只修改预期 `CHANGELOG.md` 与
  `frontend/package.json`，日期/compare links/version 均正确。
- [ ] 检查 local `v0.3.0` tag 指向 release commit。
- [ ] 在脚本 push gate 确认后推送 `main` 与 `v0.3.0`；不得 force push/move tag。
- [ ] 监控 tag-triggered `release.yml` 到成功；失败时保留 nightly，不执行 cleanup。

## 8. 发布资产与版本一致性验证

- [ ] GitHub `v0.3.0` release 非 draft/prerelease，且 `latest` 指向它。
- [ ] release notes 与 `CHANGELOG.md` 的 `0.3.0` section 一致。
- [ ] 顶层 assets exact 为两个 tarball、`checksums.txt`、`install.sh`。
- [ ] 下载到 protected temp dir，验证 checksums 与两个 tarball exact member list。
- [ ] 验证 packaged Installer 与 top-level/generated Installer byte parity。
- [ ] 验证 amd64/arm64 packaged CLI bytes/version marker 都是 `v0.3.0`。
- [ ] 验证 Go binary `/readyz` version、CLI version 与 UI display 数据流同源；若真实部署
  尚未验证，不得声称三处运行时一致已满足。
- [ ] 验证 `releases/latest/download/install.sh` 与所有 stable assets 可访问。

## 9. nightly release/tag 删除

仅在步骤 8 全绿后执行，且操作前再次读取远程状态：

- [ ] 删除 GitHub `nightly` prerelease。
- [ ] 删除 remote `refs/tags/nightly`，再删除 local `nightly` tag。
- [ ] 确认 `gh release list/view` 无 nightly，`git ls-remote` 无 nightly tag。
- [ ] 确认 v0.3.0 仍为 Latest，latest/stable asset URL 正常。
- [ ] 搜索仓库 production paths 不再引用 nightly；CHANGELOG 历史引用保留。

**Partial-failure rule**：若 release 与 tag 只删除其一，仅重试缺失动作；绝不修改或删除
`v0.3.0`。

## 10. 父任务完成与归档

- [ ] 将 parent 八项 acceptance 全部映射到最终证据并勾选。
- [ ] 独立 `trellis-check` 对照 parent PRD/design/implement、GitHub release、E2E 证据、
  asset layout 与 nightly absence 做 full-scope review。
- [ ] 必要时运行 `trellis-update-spec`；若无新可执行契约，记录无需更新的理由。
- [ ] 提交最终 task/spec/workspace 记录。
- [ ] `python ./.trellis/scripts/task.py archive 08-22-ops-experience`。
- [ ] 确认无 active Trellis task、工作区 clean、未误 push 其他 refs。

## Commit and Remote Mutation Shape

预期提交/远程事件：

1. 可能的集成修复提交（仅检查发现缺陷时）；
2. parent integration evidence/task-artifact commit；
3. normal push `main`，触发 non-publishing dry-run；
4. `chore: release v0.3.0` + immutable `v0.3.0` tag；
5. normal push `main` + tag，触发 stable publication；
6. post-verification nightly release/tag deletion；
7. parent Trellis archive commit。

任何步骤都不使用 force push。发布 tag 一旦远程发布，不移动、不复用。

## Ready-to-Start Gate

- [x] 四个 child 均完成归档。
- [x] 远程 baseline 与 release script constraints 已研究。
- [x] 用户决定真实 E2E 当前延期；release blocking policy 已写清。
- [x] PRD convergence、design、implement 与 context manifests 完成并通过 validate。
- [x] 最新最终规划摘要已提交给用户。
- [x] 用户在该摘要后明确批准启动执行。
