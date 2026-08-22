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
| CLI 实现语言 | **Shell，不用 Go** | `install.sh`（约 1079 行）+ `install_test.sh`（约 1000 行）的事务与回滚测试是核心资产，重写会清零覆盖率；且 update 需 root 而服务以非特权用户运行 |
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

- [ ] **向后兼容承诺**：`curl -fsSL .../install.sh | sudo VERSION=latest bash` 的首装与升级路径行为不变，老用户零感知
- [ ] **升级免问答**：在已安装机器上，升级操作不再要求重新回答域名、证书、凭据、监听地址中的任何一项
- [ ] **单一发布轨道**：仓库、脚本、文档、CI 中不再存在 nightly 的任何生产路径（CHANGELOG 历史条目除外）
- [ ] **版本三处一致**：GitHub release tag、`/readyz` 的 `version`、设置弹窗显示的版本，在同一次部署后三者相同
- [ ] **事务完整性**：CLI 与二进制在同一个原子事务中替换，任一步失败后回滚不残留半新半旧的组合
- [ ] **文档同步**：`README.md`、`docs/deployment.md`、`docs/upgrading.md`、`docs/operations.md`、`docs/release.md` 与新行为一致，且不再推广 nightly
- [ ] **质量门禁**：`task check`、`task test`、bundle 预算、embed + tagless 构建、`govulncheck`、`install_test.sh`、ShellCheck 全绿
- [ ] **CHANGELOG**：`[Unreleased]` 覆盖全部四项用户可见变更，含 nightly 移除的迁移说明

## 最终集成评审（父任务直接负责）

四个子任务归档后，父任务执行：

1. **端到端演练**：在干净环境验证「首装 → `bupt-ec update` → 回滚到旧版本」完整链路
2. **破坏性变更盘点**：确认 CHANGELOG 的 `Removed` / `Changed` 段落完整覆盖 nightly 移除，并给出存量 nightly 机器的处置说明
3. **发布 v0.3.0**：`scripts/release.sh v0.3.0`，验证 release 资产、release notes、`latest` 指向
4. **删除 nightly 残留**：发布后删除 GitHub 上的 nightly release 与 `nightly` git tag（顺序在稳定版发布**之后**，避免出现无任何可用安装源的窗口）

## 非目标

- 不把 CLI 改写成 Go 子命令（理由见上表）
- 不重写 `install.sh` 的事务内核（snapshot / atomic / rollback 一行不动）
- 不引入 goreleaser、Dockerfile、bats（沿用既有 backlog 判定：降级或关闭）
- 不改动 `/api/get_data` 除新增 `version` 外的任何字段语义
- 不借本次改动调整 `/readyz` 的诊断开关行为（v0.2.0 刚收敛，保持稳定）

## Notes

- 本任务为父任务，除「最终集成评审」外不承担直接实现工作
- 各子任务的技术设计与执行计划见其自身的 `design.md` / `implement.md`
