# 废除 nightly 发布轨道并将默认频道改为 latest

父任务：`08-22-ops-experience`

## Goal

收敛为单一稳定发布轨道。消除「新用户不传 `VERSION` 会被静默装上预发布版」这一坏默认，并移除 `release.yml` 中一整块 rolling-tag 维护逻辑。

## 背景：nightly 现在的三笔成本

1. `release.yml` 有一整个发布分支：force-move `nightly` tag、`gh release upload --clobber`、生成 notes——纯维护负担
2. `install.sh:188` 的 `resolve_release_version` 兜底值是 `nightly`，**首装用户不传 `VERSION` 就装到预发布版**
3. `README.md`、`docs/deployment.md`、`docs/upgrading.md`、`docs/release.md` 四处都要解释「两条轨道」的心智负担

收益侧：单人项目，`main` 推送即发，但不存在真实的 edge 用户群消费它。

**已确认决策：立即删除，不做兼容映射**（用户确认无存量机器在跑 nightly）。

## Requirements

### R1 CI：main push 改为 dry-run，不再发布

- 删除 `release.yml` 的 `Publish nightly release (main push, clobber assets)` 整个 step（含 `git tag -f nightly` / `git push origin -f refs/tags/nightly`）
- **`main` push 仍然跑完整的 build-go 与打包流程，但作为 dry-run**：产物上传为 workflow artifacts，不发布任何 release
  - 理由：打包路径（压缩、`checksums.txt`、provenance attest）只在 release job 里被覆盖，若 `main` push 完全跳过它，打包脚本损坏要到发版当天才暴露
  - 实现上复用既有的 manual-dispatch dry-run 分支，把触发条件扩展到 branch push，改动量最小
- `release.yml:59` 的版本注入 fallback 由 `nightly-$(git rev-parse --short HEAD)` 改为 `main-$(git rev-parse --short HEAD)`，去掉 nightly 语义
- tag push 的稳定版发布路径**一行不改**

### R2 安装器：默认频道改 latest

- `resolve_release_version`（`install.sh:179`）首装兜底由 `nightly` 改为 `latest`
- `validate_version`（`install.sh:193`）移除 `nightly` 接受分支，只接受 `latest` 或 `vX.Y.Z`
- **错误消息必须显式说明 nightly 已废除并给出补救命令**，例如：
  ```
  VERSION must be latest or a stable tag such as v0.1.4: nightly
  The nightly channel was removed in v0.3.0. Use VERSION=latest instead.
  ```
  这是错误提示，不是兼容代码——存量 `RELEASE_VERSION=nightly` 的机器会在此处快速失败并被告知如何修复
- `install.sh:521` 的镜像用法提示中 `<latest|nightly|vX.Y.Z>` 去掉 nightly

### R3 测试同步

`install_test.sh` 需要修改的断言：

- `:432` `"first install defaults to nightly"` → 改为断言默认 `latest`
- `:437` 版本循环 `for version in latest nightly v0.1.4` → 移除 nightly
- `:450` nightly 下载 URL 断言 → 改为 latest 对应形式
- **新增**：断言 `validate_version nightly` 失败且错误消息包含迁移提示

### R4 文档收敛

| 文件 | 改动 |
|---|---|
| `README.md` | 删除整个「Edge — rolling nightly」小节及其命令 |
| `docs/deployment.md` | 删除 nightly 安装路径（`:56`、`:66`、`:69`），`:72`/`:76` 的频道说明改为单轨道 |
| `docs/upgrading.md` | 删除 nightly 升级块（`:21-25`），`:11`/`:31` 频道说明收敛，`:65` 的 `nightly-<short-sha>` 版本示例改为 `main-<short-sha>` 或删除 |
| `docs/release.md` | 「Release flavors」表删除 nightly 行，`:20-23`、`:77`、`:80-82` 相应段落重写 |
| `docs/operations.md` | `:110` 的 `version` 字段说明去掉 nightly 取值 |

### R5 Spec 契约同步（必须，易漏）

`.trellis/spec/backend/quality-guidelines.md` 的 **Scenario: Installer Release Selection** 把 nightly 写成了可执行契约，本任务直接推翻其中多条，必须同步改写：

| 位置 | 现有契约 | 改为 |
|---|---|---|
| Contracts | 「First install with neither value uses `nightly`」 | 首装兜底为 `latest` |
| Contracts | 「Valid values are `latest`, `nightly`, or `vMAJOR.MINOR.PATCH`」 | 移除 `nightly` |
| Validation & Error Matrix | 「`latest` / `nightly` → accepted」 | `nightly` 改为 rejected，并注明迁移提示 |
| Good/Base/Bad Cases | 「Base: no explicit or saved version resolves to `nightly`」 | 改为解析到 `latest` |
| Scope / Trigger | 「prevents a stable installer URL from silently downloading the rolling nightly package」 | 该风险随轨道移除而消失，改写为单轨道语境 |

同时检查 `Convention: CI Workflow Editing Rules`（同文件）是否约束了 release 工作流的编辑方式，若涉及 nightly 一并更新。

**这条容易被漏掉**：spec 不是文档而是契约，留着旧描述会让未来的会话按错误前提写代码。

### R6 不做的事

- 不动 `quality.yml`（可复用门禁与发布轨道无关）
- 不动 tag push 的稳定版发布逻辑
- 不在本子任务内删除 GitHub 上现存的 nightly release 与 `nightly` git tag——该动作在**父任务的最终集成评审**中、**v0.3.0 发布之后**执行，避免出现窗口期

## Acceptance Criteria

- [ ] `release.yml` 中不存在 `nightly` 字样（除非在注释里解释历史）
- [ ] `main` push 后 CI 仍完整构建并打包，但不创建、不更新任何 GitHub release
- [ ] tag push 的稳定版发布行为与 v0.2.0 完全一致（回归验证）
- [ ] 首装不传 `VERSION` 时解析为 `latest`
- [ ] `VERSION=nightly` 被拒绝，错误消息含迁移指引
- [ ] `install_test.sh` 全绿，且新增了 nightly 拒绝的断言
- [ ] `shellcheck scripts/*.sh` 全绿
- [ ] 全仓库检索：`README.md` 与 `docs/` 下不再有 nightly 的生产路径描述（CHANGELOG 历史条目除外）
- [ ] `.trellis/spec/backend/quality-guidelines.md` 的 Installer Release Selection 场景已按 R5 表格全部改写，spec 中不再有 nightly 为合法值的描述
- [ ] CHANGELOG `[Unreleased]` 的 `Removed` 段落记录 nightly 轨道移除，并说明存量机器应改用 `VERSION=latest`

## Notes

- 与 `08-22-installer-modes` 的顺序：**建议本任务先做**。否则模式拆分会在即将删除的版本解析逻辑上重复施工，改两遍。
- 本任务是纯删除 + 默认值变更，无新增功能，风险集中在「漏删」而非「改错」，故验收标准以清单形式穷举耦合点。
