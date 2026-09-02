# 运维体验改进：bupt-ec CLI、安装器分模式、废除 nightly、版本可见性

## Goal

把「运维」从「安装」中分离出来：让服务器上有一个 `bupt-ec` 命令承担日常运维（升级、查状态、看日志），让升级不再需要重答一遍安装问答，同时收敛发布轨道为单一稳定线，并让运行中的版本在网页 UI 上可见。

四项交付合并为一次发布：**v0.3.0**。

## 源需求集

来自用户的原始要求（2026-08-22 会话）：

1. **CLI**：「我希望安装后就有一个 bupt-ec 命令，-h 查看命令，bupt-ec update 就是升级。诸如此类，这样可以方便使用。」
2. **安装器**：「安装器是不是也可以重新设计一下呢。」
3. **发布轨道**：「nightly 版本似乎没什么保留的必要。」
4. **UI 版本可见性**：「网页 UI 的设置栏目是不是可以加一个当前运行的版本显示呢，在『项目已开源』这一行上面插入一个当前运行版本。」

## 已确认的关键决策

| 决策点 | 结论 | 依据 |
|---|---|---|
| 发布版本号 | **v0.3.0**（不是 0.2.1） | 含新增功能与公开频道移除；`docs/release.md` 已约定 pre-1.0 阶段 minor 承载破坏性变更 |
| nightly 处理 | **立即删除，不做兼容映射** | 用户确认无存量机器在跑 nightly |
| CLI 实现语言 | **Shell，不用 Go** | generated `install.sh` 与模块化行为测试的事务/回滚覆盖是核心资产；CLI 可直接委托三模式 Installer，且 update 需 root 而服务以非特权用户运行 |
| CLI 跨版本下限 | **`bupt-ec update` 拒绝 pre-v0.3 target** | v0.2.x 既无 CLI 资产也无三模式 Installer；CLI 在联网前给出 current/latest `curl | bash` 兜底，直接 Installer 保留旧版 rollback |
| 安装器改造方式 | **演进，不推倒重写** | 事务内核无设计缺陷，缺的只是入口分层；重写必然丢失既有回滚路径覆盖 |
| 发布节奏 | **四项全部完成后一次性发布** | 用户明确要求 |

## 任务地图

按落地顺序，每项独立可验证：

| 顺序 | 子任务 | 交付物 | 复杂度 | 依赖 |
|---:|---|---|---|---|
| 1 | `08-22-api-version-field` | `/api/get_data` 信封新增 `version`；设置弹窗显示「当前运行版本」 | 轻量 | 无 |
| 2 | `08-22-drop-nightly` | 停发并删除 nightly 轨道；默认频道改 `latest` | 轻量 | 无 |
| 3 | `08-22-installer-modes` | `install.sh` 拆出 `--mode=install\|update\|reconfigure` | 复杂 | 建议在 #2 之后，避免在待删的版本解析逻辑上重复施工 |
| 4 | `08-22-bupt-ec-cli` | `/usr/local/bin/bupt-ec` 管理 CLI，纳入原子事务 | 复杂 | **强依赖 #3**（`update` 子命令直接调用 `--mode=update`） |

顺序约束写入各子任务自身的 `prd.md` / `implement.md`；父子结构本身不表达依赖。

## 跨子任务验收标准

以下条件在**全部子任务完成后**统一校验，是发布 v0.3.0 的前置门槛：

- [x] **向后兼容承诺**：v0.2.x 生产主机通过原有 `curl ... | sudo VERSION=v0.3.0 bash -s -- --mode=update` 入口成功升级
- [x] **升级免问答**：生产升级和随后同版本 CLI update 均无安装问答
- [x] **单一发布轨道**：仓库生产路径已移除 nightly，GitHub prerelease 与本地/远程 tag 也已删除
- [x] **版本三处一致**：GitHub release、`/readyz`/CLI running version 与设置弹窗均为 v0.3.0
- [x] **事务完整性**：仓库故障测试覆盖原子替换/回滚，生产正常事务同时安装 v0.3.0 二进制、CLI 与 metadata
- [x] **文档同步**：`README.md`、`docs/deployment.md`、`docs/upgrading.md`、`docs/operations.md`、`docs/release.md` 与新行为一致，且不再推广 nightly（本地审计：`research/2026-09-02-local-integration-audit.md`）
- [x] **质量门禁**：`task check`、`task test`、bundle 预算、embed + tagless 构建、`govulncheck`、`install_test.sh`、ShellCheck 全绿（本地 preflight 与 main dry-run：`review/2026-09-02-main-dry-run-success.md`）
- [x] **CHANGELOG**：`[Unreleased]` 覆盖全部四项用户可见变更，含 nightly 移除的迁移说明（本地审计：`research/2026-09-02-local-integration-audit.md`）

## 最终集成评审（父任务直接负责）

四个子任务已全部归档；本地全量门与 `main` release dry-run 已通过。父任务余下流程：

1. **生产恢复前置**：在生产主机变更前创建云主机/VM 快照，并确认快照状态可恢复、控制台/SSH 可用；采集不含秘密的 systemd/Nginx/health/version 基线。
2. **发布 v0.3.0**：执行 release-critical preflight 后运行 `scripts/release.sh v0.3.0`，监控 tag workflow，并验证 release notes、四个精确资产、checksum、版本注入与 `latest` 指向。
3. **生产金丝雀升级**：在维护窗口使用已发布 current/latest Installer 将现有生产主机正常升级到显式 `v0.3.0`；只执行正常升级及只读 smoke checks，不做故障注入或 legacy rollback 演练。
4. **生产观察**：验证 systemd/Nginx、CLI、metadata、`/healthz`、`/readyz`、API/UI 与日志；至少观察两个检查点并确认版本三处一致。异常时优先依赖 Installer 自动 rollback；事务已成功但后续异常则使用已验证 VM 快照恢复。
5. **删除 nightly 残留**：仅在 v0.3.0 资产和生产观察都通过后删除 GitHub nightly release、远程 tag 与本地 tag，随后验证 v0.3.0 仍为 Latest。

当前远程状态：`v0.3.0` 已由 release commit `8f211a8` 发布为 Latest，tag workflow `33584926291` 成功；v0.2.x 生产主机已成功升级并正常运行；`nightly` GitHub prerelease、本地 tag 与远程 tag 均已删除。用户明确豁免重复 release/asset 验证、正式双 checkpoint 及备份/回滚准备，并接受对应剩余风险。证据见 `review/2026-09-02-v0.3.0-direct-release.md` 和 `review/2026-09-02-production-success-nightly-cleanup.md`。

## 发布风险接受与生产边界

- 用户明确决定不建立独立测试环境，直接在生产环境执行首次真实主机升级，并接受因此无法在发布前证明 clean-host、故障注入、pre-v0.3 removal/restore 与真实 systemd/Nginx rollback 的剩余风险。
- 用户最初选择云主机/VM 快照作为外部恢复手段，随后在生产升级成功并确认运行正常后明确表示无需备份/回滚；未完成快照与恢复演练作为已接受证据缺口记录，不追溯执行。
- 仓库 mocks、release-layout simulation、本地全量门与 `main` dry-run 是 clean-install、故障 rollback、legacy removal 的发布前证据；不得声称它们等同于独立真实 E2E。
- 生产环境禁止故意安装损坏候选、禁止为了测试执行 pre-v0.3 rollback、禁止测试 CLI/metadata 删除恢复。直接 Installer 的 v0.2.x fallback 仅保留为事故恢复选项，使用前需再次明确授权。
- 由于 exact `v0.3.0` 资产只有稳定 tag workflow 才会发布，生产验证发生在 tag 公开之后；若生产发现问题，不移动或复用 `v0.3.0` tag，不删除 nightly，恢复主机后以 fix-forward 新版本处理。

## v0.3.1 内联修复扩展

生产升级暴露一个不影响事务结果但会误导操作者的输出问题：服务重启后第一次 loopback 健康探测可能在监听端口就绪前失败，`curl -fsS` 会把瞬时 `curl: (7)` 写到终端；Installer 随后重试成功并正确报告部署完成。

用户要求不创建新 Trellis 任务，直接在本父任务内完成修复并发布 v0.3.1。验收标准：

- [x] 中间健康探测失败不再输出 curl 原始 stderr；
- [x] 最多 10 次、每次超时、间隔、成功条件与事务/回滚语义不变；
- [x] 全部探测失败时仍输出明确的 `Service health check failed` 并触发原有回滚；
- [x] generated `scripts/install.sh` 与源片段同步，回归测试覆盖“先失败后成功无噪声”和“最终失败仍有高层诊断”；
- [x] CHANGELOG 记录修复，immutable v0.3.1 发布成功；未自动升级生产。

## 非目标

- 不把 CLI 改写成 Go 子命令（理由见上表）
- 不重写 `install.sh` 的事务内核（snapshot / atomic / rollback 一行不动）
- 不引入 goreleaser、Dockerfile、bats（沿用既有 backlog 判定：降级或关闭）
- 不改动 `/api/get_data` 除新增 `version` 外的任何字段语义
- 不借本次改动调整 `/readyz` 的诊断开关行为（v0.2.0 刚收敛，保持稳定）

## Notes

- 本任务原为仅集成评审父任务；按用户明确 inline 指令，额外直接承担 v0.3.1 健康重试输出修复与发布
- 各子任务的技术设计与执行计划见其自身的 `design.md` / `implement.md`
