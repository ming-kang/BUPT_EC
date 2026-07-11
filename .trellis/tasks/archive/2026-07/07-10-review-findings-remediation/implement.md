# 复查发现整改与验收闭环实施计划

## 0. Planning gate

- [x] 用户审阅并确认父任务及六个子任务范围。
- [x] 确认所有任务保持 planning，未运行 task.py start。
- [x] 记录基线 HEAD、工作区状态和当前受支持工具链。
- [x] 保持 .agents/、.codex/ 和模板哈希文件不进入提交。

## 1. Execute child tasks in order

- [x] 完成并归档 metrics-endpoint-login-observability。
- [x] 完成并归档 adaptive-refresh-backoff-jitter。
- [x] 完成并归档 frontend-reload-deadline-jitter。
- [x] 完成并归档 installer-mirror-url-safety。
- [x] 完成并归档 upstream-message-unicode-safety。
- [x] 完成并归档 dependency-spec-evidence-hygiene。
- [x] 每次只启动一个 child；父任务保持 planning，直到需要最终集成记录。

## 2. Cross-child integration review

- [x] 搜索所有 RuntimeMetrics、ObserveLogin、HandlerFor 和 /metrics gzip 调用点。
- [x] 搜索所有 nextRefreshAllowed、backoff ladder、Clock 和 jitter 定义，确认单一策略。
- [x] 搜索 frontend reload delay、stale_until 和 random 调用，确认没有重复实现。
- [x] 搜索 DOWNLOAD_BASE_URL 输出和校验分支，确认不存在完整敏感 URL 回显。
- [x] 搜索 safeRemoteMessage 和 secret regex，确认所有 JW 上游文本走统一清洗边界。
- [x] 搜索 CacheStore、旧轮询值和固定 total 30s 描述，清除过期合同。

## 3. Full validation

见 hygiene child verification（2026-07-11）：Go race/build/vet/tidy/verify/govulncheck、
frontend lint/test/build/audits、installer tests 通过；shellcheck/actionlint 由 CI
（本地 Windows 无二进制）。

## 4. Evidence and archive gate

- [x] 每个 child 的 implement.md 已按实际结果勾选，并附验证摘要。
- [x] 每个 child 的 task.json 记录实现 commit，且提交范围不混入其他 child。
- [x] 所有 user-visible 变化均在同一 commit 更新 CHANGELOG [Unreleased]。
- [x] 父任务记录最终集成检查结果和所有 child commit。
- [x] 只有在无未完成 acceptance item 时才提交并归档父任务。

### Child → behavior / archive commits

| Child | Behavior | Archive |
| --- | --- | --- |
| metrics-endpoint-login-observability | `efd0d31` | `4a1d4db` |
| adaptive-refresh-backoff-jitter | `ea76336` | `e5ac060` |
| frontend-reload-deadline-jitter | `c584b97` | `75b1cd4` |
| installer-mirror-url-safety | `30cbbad` | `e0afe5d` |
| upstream-message-unicode-safety | `66987d4` | `af288e0` |
| dependency-spec-evidence-hygiene | `f3a0d29` | `f6ea24f` |

### Historical evidence gap (do not forge)

Earlier 07-10 archives outside this parent may lack checked checklists or commit
SHAs. They remain as-is. Future tracking uses this parent’s six children only.

## Review Gates

- 不以“全量测试通过”替代缺陷定向回归。
- 不通过忽略 gzip、删除指标或放宽安全校验来绕过问题。
- 不让 jitter 越过绝对业务截止时间或改变 partial/full outcome。
- 不事后伪造历史归档清单。
- 每个 child 必须同步其已稳定的 owning 文档合同；hygiene 只整理跨任务冲突。

## Rollback Points

- 每个 child commit 是独立回滚点。
- 任一 child 暴露跨层合同冲突时，停止后续任务并返回其 design/prd 修订。
- 最终 docs/spec 收口失败时，不归档父任务，也不回滚已验证的独立代码修复。
