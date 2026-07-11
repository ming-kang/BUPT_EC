# 复查发现整改与验收闭环

## Goal

关闭 2026-07-10 全量复查确认的运行时、可观测性、前端调度、安装器安全、
日志清洗、依赖整洁性及规范同步缺口，并建立可追溯的验收证据。该父任务只组织
需求、子任务边界和最终集成验收，不直接承担业务代码实现。

## Background

复查基线为 56c4c10，复查目标为 7cc42bb。现有 Go、前端、安装器和工作流质量
门禁大多通过，但测试未覆盖到以下已确认问题：

- router.go:68 的全局 gzip 与 main.go:59 的 Prometheus 默认压缩叠加，导致
  GET /metrics 在 Accept-Encoding: gzip 下双重压缩。
- service/metrics.go:12 定义了 ObserveLogin，但 TokenManager 登录路径没有调用点。
- service/refresh_coordinator.go:36 使用固定阶梯，未实现原任务要求的可注入 jitter。
- frontend/src/reloadSchedule.js 在 stale_until 截断后再施加 jitter，并重复调用随机源。
- scripts/install.sh 会输出完整 DOWNLOAD_BASE_URL，且 insecure opt-in 放宽到任意 scheme。
- service/jw_error.go 只清洗 ASCII 控制字符、普通空格和 NBSP。
- go mod tidy -diff 不通过，规范、AGENTS、运维文档和 changelog 存在相互矛盾的旧合同。
- 旧归档任务缺少真实清单勾选、校验记录和 commit 证据；不得通过事后伪造历史修复。

## Child Task Map

| Child task | Owning deliverable | Priority |
| --- | --- | --- |
| 07-10-metrics-endpoint-login-observability | 单层 metrics 编码、登录指标及 isolated registry 测试 | P1 |
| 07-10-adaptive-refresh-backoff-jitter | 有界可注入 refresh jitter、统一 Clock 测试 | P2 |
| 07-10-frontend-reload-deadline-jitter | stale_until 硬上限、单次随机采样和生命周期回归 | P2 |
| 07-10-installer-mirror-url-safety | 镜像 URL scheme/authority 校验和无敏感日志 | P1 |
| 07-10-upstream-message-unicode-safety | Unicode 控制/空白清洗与脱敏回归 | P2 |
| 07-10-dependency-spec-evidence-hygiene | go.mod、CI、docs/spec/changelog 和验收证据收口 | P2 |

## Requirements

### R1 — 独立交付与顺序

- 每个子任务必须能独立实现、测试、提交和归档。
- 推荐顺序为：metrics → backend jitter → frontend jitter → installer URL →
  Unicode sanitizer → dependency/spec/evidence。
- 每个行为子任务必须在自身提交同步 owning spec、用户文档和 CHANGELOG；最后的
  hygiene 子任务基于最终接口做一致性审计和残留修正，不替代同提交义务。
- 父任务不得因子任务存在而进入 in_progress；实施时只启动当前交付子任务。

### R2 — 兼容性边界

- 不改变 GET /api/get_data 成功 payload、错误 envelope、LogID、readyz schema 或
  SPA fallback。
- 不改变同日缓存、partial/stale 语义、refresh singleflight、JW token singleflight、
  graceful shutdown 和单实例部署边界。
- public Nginx 仍必须对 /metrics 返回 404；loopback backend scrape 保持可用。
- 不改变 release asset 名称、安装路径、systemd 用户、env mode 0600 或事务回滚合同。
- 不重新跟踪 .agents/、.codex/ 或 .trellis/.template-hashes.json。

### R3 — 测试与证据

- 每个缺陷必须有在修复前能够失败的定向回归测试，不能只依赖全量门禁。
- metrics、jitter 和 Clock 测试必须使用 isolated registry、fake clock/random，
  不以 sleep 驱动核心状态。
- installer 测试必须断言输出中不出现测试 secret，而不仅是检查退出码。
- 每个子任务完成时更新 implement.md 勾选状态、记录实际验证命令和结果，并在
  task.json 中记录对应 commit 后再归档。
- 历史归档保持原样；新任务说明历史证据缺口，不追溯填写无法证明的结果。

### R4 — 最终集成

- 所有子任务完成后由父任务执行一次跨层合同复查和完整质量门禁。
- 最终 docs、AGENTS、.trellis/spec 和 CHANGELOG 必须描述同一组当前行为。
- 稳定发布前 go mod tidy -diff、actionlint、ShellCheck、govulncheck 和前后端完整
  门禁均须通过。

## Acceptance Criteria

- [x] 六个子任务全部拥有完整 prd.md、design.md 和 implement.md。
- [x] /metrics 对 identity/gzip 请求均能被标准客户端正确解析，且 public Nginx 不暴露。
- [x] login success/failure 与 auth recovery 指标按共享操作准确计数。
- [x] total refresh backoff 使用有界、可注入 jitter，partial/full 语义不变。
- [x] 前端任何自动 reload delay 均不超过 stale_until，随机源每次调度只调用一次。
- [x] 镜像 URL 凭据或敏感参数不会进入安装器输出，非 HTTP(S) scheme 始终拒绝。
- [x] 上游 Unicode 控制、分隔和空白字符不能形成多行或绕过敏感字段脱敏。
- [x] go mod tidy -diff 通过，CI 能阻止再次漂移。
- [x] AGENTS、docs、spec 和 changelog 不再保留 CacheStore、固定 total 30s 或旧前端
      轮询间隔等冲突描述。
- [x] 每个新子任务均有真实验证记录和 commit 关联，历史归档未被伪造性改写。
- [x] Go race/build/vet/govulncheck、frontend lint/test/build/audits、installer tests、
      ShellCheck、actionlint 和 git diff --check 全部通过。

## Out of Scope

- 部署 Prometheus/Grafana 服务或新增公网 metrics 认证层。
- 引入通用 circuit-breaker、前端数据请求框架或分布式缓存。
- 重写安装器语言、改变 release asset 布局或启用自动第三方镜像。
- 修改历史已发布版本说明或虚构旧任务的完成证据。
