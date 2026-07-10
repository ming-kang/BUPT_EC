# 可靠性审计修复总任务

## Goal

修复 2026-07-09 至 2026-07-10 可靠性审计发现的跨层缺陷，使同日缓存、部分校区刷新、跨日切换、鉴权恢复和服务器安装升级具备一致、可诊断、可回滚的行为。

## Background

- 后端明确禁止跨上海业务日复用缓存，但前端当前会在跨日刷新失败后保留昨日快照。
- 刷新结果当前只有 Go `error` 的成功/失败二态，部分校区失败通过公开 payload 的 `error` 字段补充，导致日志、运行状态和错误优先级不一致。
- 午夜 warmup 只尝试一次；如果刷新协调器仍处于 backoff，会跳过当天后续自动恢复。
- 两个校区并发收到旧 token 的鉴权失败时，较晚返回的请求可能再次强制登录。
- 稳定版文档命令没有传 `VERSION=latest`，而安装器未设置 `VERSION` 时默认下载 `nightly`。
- 安装器在下载和 checksum 校验前覆盖现有环境文件，失败安装可能留下半更新配置。

## Requirements

- R1：后端刷新必须明确区分完整成功、部分成功和全量失败，且每种结果具有稳定的缓存、backoff、日志、运行状态和 API 语义。
- R2：前端只能保留当前上海业务日且未超过 `stale_until` 的旧快照；失效快照必须从教室 UI 中移除。
- R3：降级和失败状态的自动重试必须有界退避，不能在跨日失败时形成每秒轮询。
- R4：午夜 warmup 必须可取消，并在 backoff 或上游暂时失败后继续为当前业务日安排恢复尝试。
- R5：并发鉴权失败必须复用其他请求已经刷新的 token，单次旧 token 失效最多触发一次有效登录。
- R6：稳定版、nightly 和固定标签的安装选择必须与文档命令一致，并在后续重跑安装器时保持所选通道或标签。
- R7：下载、校验或预检失败不得修改现有二进制和配置；提交阶段失败必须尽可能恢复上一可运行版本。
- R8：公开 API 的变化只允许向后兼容的字段新增；不得暴露原始 JW 错误、响应体、凭据或 token。
- R9：所有用户可见变化必须同步更新 `CHANGELOG.md`、README 和相关 `docs/`；架构变化同步更新 `.trellis/spec/backend/`。
- R10：保留现有未提交的 `.agents/`、`.codex/` 和 `.trellis/.template-hashes.json` 改动，不将其误判为本任务实现内容。

## Child Task Map

1. `07-10-refresh-outcome-contract`：刷新三态、部分校区契约、错误优先级和运行诊断。
2. `07-10-frontend-cache-validity`：跨日快照有效性、失败保留策略和轮询退避。
3. `07-10-warmup-lifecycle`：可取消的 warmup 调度与跨日恢复。
4. `07-10-token-refresh-coordination`：旧 token 并发鉴权失败协调。
5. `07-10-installer-version-policy`：安装版本选择、持久化和文档一致性。
6. `07-10-installer-transaction-safety`：安装预检、原子替换和失败回滚。

## Acceptance Criteria

- [ ] 六个子任务均完成各自 PRD 的验收标准并归档。
- [ ] 昨日数据在午夜后刷新失败时不会继续驱动教室筛选 UI。
- [ ] 部分成功后发生全量失败时，对外提示和运行状态反映最新全量失败，而不是旧的部分警告。
- [ ] 没有真实 `/api/get_data` 流量时，午夜 warmup 仍可在临时失败恢复后建立当日缓存。
- [ ] 两个校区并发使用同一失效 token 时，测试证明只执行一次必要登录。
- [ ] README 中的 stable、nightly、固定标签命令分别下载预期 release。
- [ ] checksum 或预检失败测试证明已有 env 和二进制保持不变。
- [ ] `gofmt -l .` 无输出，`go vet ./...`、`go test -race ./...`、前端 lint/test/build 全部通过。
- [ ] 脚本通过 `bash -n` 和 `shellcheck scripts/*.sh`；新增的脚本行为测试在 CI 中执行。
- [ ] 完成一次父任务级跨层审查，确认 API、前端、运维文档和 release 资产保持一致。

## Out of Scope

- 不引入数据库、Redis 或跨实例共享缓存。
- 不改变 JW 协议、AES 密钥、校区列表或空教室业务规则。
- 不重做整套部署系统，也不引入新的包管理器或第三方发布平台。
- 不改变现有 release 资产名称和归档布局。
