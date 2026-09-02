# 执行计划：v0.3.0 生产金丝雀发布与 nightly 下线

任务：`08-22-ops-experience`
状态：in_progress（需求变更后回到规划审阅 gate）

## 0. 规划、子任务与远程基线

- [x] 四个 child 均完成归档，父任务是唯一 active Trellis task。
- [x] PRD/design/implement 与 implement/check context manifests 完成并通过 validate。
- [x] 用户批准第一阶段本地集成、普通 push main 与 GitHub dry-run。
- [x] 记录 local/remote refs、GitHub release、nightly 与 release script baseline。

## 1. 本地跨子任务集成审计

- [x] 建立四个 archived child 的 acceptance-to-evidence 表。
- [x] 复核 parent 八项跨子任务验收。
- [x] 分类所有 `nightly` 匹配；无当前生产发布/推广路径。
- [x] `[Unreleased]` 完整覆盖四项交付与 nightly migration。
- [x] 验证 release composition、generated Installer、CLI/Go/API/UI version flow 和
  exact four-asset contract。
- [x] 记录于 `research/2026-09-02-local-integration-audit.md`。

## 2. 本地全量发布前门禁

- [x] `task check` 全绿。
- [x] `task test` race suite 全绿。
- [x] `task installer:check` 全绿：Installer 41 scenarios / 44 test functions、CLI 10
  scenarios、release-layout stable/main simulation 与 recursive ShellCheck。
- [x] frontend 18 files / 127 tests、fresh build 与 bundle `164,862 / 230,888 B` 全绿。
- [x] tagless/embed Go builds 全绿。
- [x] pinned Go 1.25.13 `govulncheck` 无 reachable finding。
- [x] actionlint、generator drift、`git diff --check` 与 task validate 全绿。

审计修复：

- [x] `3969e56 fix(frontend): pin browserslist security update`。
- [x] `1cd87b2 fix(test): use Shanghai dates in cache fixtures`。
- [x] `40f0b46 fix(ci): suppress dynamic test override lint`。

## 3. 集成证据提交

- [x] `4f60f8c chore(task): record ops release preflight`。
- [x] `37926e6 chore(task): record main release dry-run`。
- [x] `c555d03 chore(task): record production canary plan`。
- [x] 所有已提交证据 secret-free。

## 4. Push main 与 GitHub dry-run

- [x] normal non-force push main；production-canary planning HEAD
  `c555d0372981022ef992560e11c5722adc623e21` 已同步到 `origin/main`。
- [x] Release run `33583463349` 对 exact planning HEAD 成功。
- [x] quality、amd64/arm64 builds、composition、attestation 与 dry-run upload 全绿；
  tag-only notes/publication steps skipped。
- [x] dry-run exact four assets、checksums、tar layout、Installer parity、
  `main-c555d03` CLI/Go injection 验证通过。
- [x] 未发布 GitHub release；无 v0.3.0 tag；v0.2.0 仍为 Latest，nightly 未修改。
- [x] 证据记录于 `review/2026-09-02-production-plan-preparation.md`。
- [x] 独立终检 zero finding；无需新项目级 spec。

## 5. 风险接受与生产方案重规划

- [x] 用户明确无独立测试条件，决定直接在生产环境执行首次真实升级。
- [x] 用户选择云主机/VM 快照作为 Installer transaction 外部恢复兜底。
- [x] PRD/design 改为发布后生产金丝雀：禁止生产 fault injection、clean install、
  pre-v0.3 removal/restore 演练。
- [x] 记录接受的证据缺口：clean-host/真实故障 rollback 仅由 repository mocks 覆盖，
  production 只验证正常 update 和只读 smoke/observation。
- [x] 向用户提交更新后的最终规划摘要和精确远程 mutation 边界。
- [x] 用户在该摘要后明确重新批准发布、生产升级与成功后 nightly 删除。

**Gate**：本次风险模型和执行顺序是 material plan change。重新批准前不得运行 release
script、创建/push v0.3.0、连接生产执行变更或删除 nightly。

## 6. 生产恢复与访问 preflight

在 stable tag 创建前完成：

- [ ] 获得生产连接方式和明确主机标识；确认命令目标是预期生产 VM，不把凭据写入仓库。
- [ ] 确认维护窗口内可接受短暂 restart/readiness warmup。
- [ ] 创建 VM/系统盘 snapshot，等待 provider 状态达到 `available/completed`。
- [ ] 记录 snapshot ID/time/state（不记录 access secret），验证 provider console 和 SSH。
- [ ] 采集 secret-free baseline：
  - [ ] `systemctl is-active/is-enabled bupt-ec`；
  - [ ] `nginx -t`；
  - [ ] healthz/readyz HTTP status 和 running version；
  - [ ] installed target owner/mode/hash，但不显示 private env 内容；
  - [ ] 当前 API/UI 可用性；
  - [ ] 当前 saved selector/trust source 通过安全方式确认。
- [ ] 确认 snapshot restore 操作路径和负责执行者。

**Gate**：snapshot 仅 requested/in-progress、console/SSH 不可用、baseline 异常或 custom
mirror/saved nightly 未厘清时不得发布 tag。

## 7. v0.3.0 release-critical preflight

```bash
git fetch --force origin main --tags
git status --short --branch
bash scripts/generate-install.sh --check
bash scripts/install_test.sh
bash scripts/cli_test.sh
bash scripts/release_layout_test.sh
scripts/extract-changelog.sh Unreleased
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.10 \
  .github/workflows/ci.yml .github/workflows/quality.yml .github/workflows/release.yml
git diff --check
```

- [ ] clean `main == origin/main`，latest green dry-run 属于 current HEAD。
- [ ] local/remote/GitHub 都无 `v0.3.0` tag/release。
- [ ] `v0.2.0` 仍为 Latest；nightly release/tag 仍存在。
- [ ] release-critical suites、generator、actionlint、diff check 全绿。
- [ ] 人工审阅 `Unreleased` 输出作为最终 release notes。

## 8. 创建并发布 v0.3.0

- [ ] 运行 `scripts/release.sh v0.3.0`。
- [ ] 检查 release commit 仅按脚本契约修改 `CHANGELOG.md` 与
  `frontend/package.json`，日期、compare links、version 正确。
- [ ] 检查 local `v0.3.0` 指向 release commit。
- [ ] normal push `main` + `v0.3.0`；不 force、不移动/reuse tag。
- [ ] 监控 tag-triggered Release workflow 到成功。

失败规则：

- commit/tag 未 push：确认远程无 ref 后才可使用文档 local rollback；
- tag workflow 失败：不动生产、不删 nightly；transient 可 rerun，代码缺陷必须新版本
  fix-forward，不移动 tag。

## 9. GitHub release/资产验证

在生产变更前完成：

- [ ] `v0.3.0` release 非 draft/prerelease，`latest` 指向它。
- [ ] notes 与 CHANGELOG `0.3.0` section 一致。
- [ ] 顶层 assets exact：两个 tarball、`checksums.txt`、`install.sh`。
- [ ] protected temp dir 下载并验证 checksums、tar exact members/modes。
- [ ] packaged/top-level/generated Installer byte parity。
- [ ] amd64/arm64 CLI 与 Go binary version injection 均为 `v0.3.0`。
- [ ] `releases/latest/download/install.sh` 和 stable assets 可访问。

**Gate**：任一 mismatch 都不得升级生产，nightly 保留。

## 10. 生产金丝雀正常升级

### 10.1 Live command review

- [ ] 根据 baseline 确认生产 host 是 pre-v0.3 还是已有 CLI。
- [ ] 确认 saved source 是 official GitHub；若 custom mirror 或 saved nightly，先按文档
  选择明确路径，不静默改变 trust source。
- [ ] 向用户展示最终主机、snapshot 与命令摘要；确认后执行。

Pre-v0.3 official-source host 推荐下载后运行 current Installer，使 stdin 真正关闭：

```bash
session="$(mktemp -d /tmp/bupt-ec-release.XXXXXXXX)"
chmod 0700 "${session}"
curl --fail --show-error --silent --location \
  --proto '=https' --proto-redir '=https' \
  --connect-timeout 10 --max-time 60 \
  -o "${session}/install.sh" \
  https://github.com/ming-kang/BUPT_EC/releases/latest/download/install.sh
sudo VERSION=v0.3.0 bash "${session}/install.sh" --mode=update < /dev/null
rm -rf "${session}"
```

- [ ] Installer 返回 0 并报告 deployment success。
- [ ] 若返回非零，确认是否 `Rollback completed.`，对比 baseline，保留 recovery dir，停止
  后续动作且不删 nightly。

### 10.2 Immediate smoke

- [ ] `bupt-ec version/status/health/logs -n 50`。
- [ ] systemd active/enabled；`nginx -t` 通过。
- [ ] healthz 2xx；readyz 在 warmup 后 2xx。
- [ ] CLI root:root 0755；metadata root:root 0644 exact 两字段；private env 仍 root:root
  0600，绝不输出其值。
- [ ] selector/running/CLI/API/UI version 均按契约为 v0.3.0。
- [ ] 页面和今日数据正常；无重复新错误或 secret disclosure。

### 10.3 Observation

- [ ] 第一个完整成功 checkpoint。
- [ ] 至少五分钟后、且不早于升级后十分钟完成第二个成功 checkpoint。
- [ ] 两次均要求 active/enabled、Nginx valid、双 probe 2xx、v0.3.0 version consistency、
  API/UI 正常和无持续新错误。

## 11. 生产异常处理

- [ ] Installer transaction 失败时先等待/验证自动 rollback，不在 rollback 运行中 restore VM。
- [ ] rollback incomplete 时保留 root-only recovery dir 并使用 provider console/snapshot plan。
- [ ] Installer 成功但 smoke/observation 失败时停止变更、保存 safe evidence、恢复已验证
  VM snapshot。
- [ ] 已发布 v0.3.0 有缺陷时不移动/删除/reuse tag；恢复 production，保留 nightly，
  fix-forward 新 immutable version。
- [ ] direct v0.2.x fallback 仅作为二级事故恢复，执行前重新取得明确授权；不用于测试。

## 12. nightly cleanup

仅在步骤 9 和 10 全绿后：

- [ ] 删除 GitHub `nightly` prerelease。
- [ ] 删除 remote `refs/tags/nightly`，再删除 local nightly tag。
- [ ] 验证无 nightly release/tag。
- [ ] 验证 v0.3.0 仍为 Latest，latest/stable asset URL 正常。
- [ ] 保留 snapshot 至 cleanup evidence 完成，随后交还 operator retention policy。

Partial failure 只重试缺失 cleanup，不触碰 v0.3.0。

## 13. 最终终检、spec 判断与父任务归档

- [ ] 将 parent 八项 acceptance 映射到 repository/dry-run/production evidence；明确记录
  waived clean-host/fault-injection gap 后再勾选。
- [ ] 独立 `trellis-check` full-scope review：release、production checkpoints、version、
  assets、nightly absence、git state。
- [ ] 运行 `trellis-update-spec` 判断；无新 executable contract 时记录无需更新理由。
- [ ] 提交最终 task/workspace/spec evidence。
- [ ] `task.py archive 08-22-ops-experience`。
- [ ] 确认无 active task、工作区 clean、local/remote refs 符合预期。

## Remote Mutation Boundaries

重新批准后授权范围将是：

1. normal push release commit 和 immutable `v0.3.0` tag；
2. 对已确认生产主机执行一次正常 v0.3.0 update；
3. 仅在 release + production 两次 checkpoint 全绿后删除 nightly release/tag；
4. 不 force push、不移动 tag、不执行 production fault injection、不主动 direct rollback
   到 pre-v0.3。

## Ready-to-Resume Gate

- [x] 第一阶段审计、门禁、main push/dry-run、独立 check 已完成。
- [x] 用户明确选择 production canary 并接受无独立 E2E 的剩余风险。
- [x] 用户选择可恢复 VM snapshot。
- [x] 更新后的 PRD/design/implement 已写明 mutation ordering、rollback 和禁测边界。
- [x] 最新 materially changed planning summary 已提交用户。
- [x] 用户在该摘要后明确批准继续执行。
