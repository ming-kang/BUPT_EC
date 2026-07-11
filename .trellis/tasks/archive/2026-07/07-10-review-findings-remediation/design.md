# 复查发现整改与验收闭环设计

## Task Architecture

父任务作为整改路线图和最终集成门，不直接修改生产代码：

~~~text
review-findings-remediation
├─ metrics-endpoint-login-observability
├─ adaptive-refresh-backoff-jitter
├─ frontend-reload-deadline-jitter
├─ installer-mirror-url-safety
├─ upstream-message-unicode-safety
└─ dependency-spec-evidence-hygiene
~~~

每个 child 拥有一个清晰的运行时边界和测试集合。行为子任务在自身 commit 同步 owning
spec、用户文档和 CHANGELOG；最后一个 hygiene 任务只做全局一致性审计、依赖/CI
收口和残留修正。

## Cross-Layer Flow

### Metrics flow

~~~text
TokenManager / ClassroomService event
  → RuntimeMetrics low-cardinality interface
  → private Prometheus registry
  → HTTPServer.Metrics
  → exactly one compression owner
  → loopback scraper
~~~

### Refresh protection flow

~~~text
refresh outcome + injected Clock
  → base ladder policy
  → bounded injected jitter
  → nextRefreshAllowed under refreshMu
  → request and warmup suppression
~~~

### Frontend scheduling flow

~~~text
response metadata + now + one random sample
  → base state delay
  → bounded positive jitter
  → stale_until hard clamp
  → visibility-aware timer
  → background fetch
~~~

### Installer trust flow

~~~text
explicit/saved DOWNLOAD_BASE_URL
  → scheme/authority validation
  → safe display identity
  → package and checksums download
  → existing transaction boundary
~~~

## Shared File Coordination

- main.go、router.go、handler tests 由 metrics 子任务先稳定。
- service/classroom_service.go 与 refresh_coordinator.go 由 backend jitter 子任务在
  metrics 接口稳定后修改。
- frontend reload helper 与 lifecycle tests 不依赖后端接口，可随后独立完成。
- scripts/install.sh 和 install_test.sh 只由 installer 子任务修改。
- jw_error.go 只由 Unicode sanitizer 子任务修改。
- 每个行为 child 更新其 owning spec/docs/CHANGELOG；go.mod、quality workflow、跨任务
  冲突清理和最终 negative audit 由最后的 hygiene 子任务负责。

## Compatibility Strategy

- 所有新增注入点均提供 nil/default 生产实现，现有构造调用保持可编译。
- 指标 label 只使用既有固定枚举，不新增 token、URL、错误文本或 log_id。
- jitter 只影响下一次允许时间，不影响 refresh outcome、cache payload 或 readiness。
- 前端只改变自动调度时间，不改变 hook 返回结构或 API normalization。
- 安装器拒绝此前文档未承诺支持的危险 URL 形式；HTTPS/显式 HTTP 镜像合同保留。

## Evidence Strategy

历史归档不重新解释。新的任务闭环以三层证据为准：

1. 定向回归测试证明具体缺陷。
2. 完整门禁证明跨层兼容。
3. task.json commit、implement.md 勾选和任务日志证明实际执行。

任何一层缺失都不能归档 child。父任务只有在所有 child 已归档且集成检查完成后才归档。

## Rollback Shape

- 每个 child 单独提交，允许按功能边界回滚。
- metrics 编码和登录埋点应在同一 child 内完成，但可分两个 commit 时必须保持 endpoint
  始终可抓取。
- backend/frontend jitter 可以独立回滚，不影响 metrics。
- installer 和 Unicode sanitizer 属于安全加固，回滚必须同时恢复对应文档声明。
- hygiene 任务只在代码接口稳定后执行；若文档发现设计冲突，回到对应 child 修正规格。
