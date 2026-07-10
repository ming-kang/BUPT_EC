# 指标端点与登录可观测性修复

## Goal

确保 loopback GET /metrics 对标准 Prometheus 客户端始终可解析，并让登录及认证恢复
成功/失败指标真实反映共享 JW 登录操作，而不是只存在未使用的 collector 定义。

## Background

- router.go:68 为除 health/readiness 外的响应安装自定义 gzip middleware。
- main.go:59 使用 promhttp.HandlerFor 和默认 HandlerOpts；Prometheus handler 自身也会
  根据 Accept-Encoding 压缩。
- 本机验证显示 Accept-Encoding: gzip 响应解压一次后仍是 gzip 数据，证明双重压缩。
- service/metrics.go:12 定义 ObserveLogin，service/prometheus_metrics.go:124 实现它，
  但仓库没有任何调用点。
- service/token_manager.go:146 的 loginAndStore 是实际共享网络登录边界，当前只更新
  RuntimeStatus 和 slog。
- metrics 任务没有 isolated registry、endpoint 或 label assertion 测试。

## Dependencies

- 这是推荐的第一个实现子任务，没有其他新 child 的代码依赖。
- 完成后必须先稳定 RuntimeMetrics、TokenManager 和 HandlerFor 接口，backend jitter
  子任务再修改共享 ClassroomServiceOptions。

## Requirements

### R1 — 单一压缩所有权

- /metrics 的响应只能被压缩一次。
- 推荐由现有全局 gzip middleware 统一负责 gzip，Prometheus handler 禁用内部压缩。
- identity、gzip、gzip;q=0 和混合 Accept-Encoding 请求均不得返回损坏的 exposition。
- gzip 响应解压一次后必须直接得到 Prometheus text format，Content-Encoding 与 Vary
  必须和实际 body 一致。
- health/readiness 继续保持不压缩；其他 API/SPA gzip 行为不得改变。

### R2 — 登录指标语义

- ObserveLogin 只统计实际发生的共享 JWClient.Login 操作，不统计缓存 token 复用、
  override 安装或 singleflight waiter。
- outcome 固定为 success 或 failed。
- source 表示触发本次网络登录的 token 来源：
  - override：被拒绝的 token 确实来自启动注入的 JW_TOKEN；
  - login：首次/手动登录，或被拒绝 token 来自先前 HTTP 登录。
- 一个 singleflight 登录操作无论有多少 waiter，只记录一次 counter 和一次 duration。
- API URL 获取或 JW 登录任一步失败，都作为该共享登录操作 failed。
- duration 使用注入 Clock；不得使用 token、用户名、URL、错误文本或 log_id label。

### R3 — Registry 和 endpoint 测试

- 使用 NewPrometheusMetrics 创建隔离 registry，不使用 prometheus 默认全局 registry。
- 测试 refresh/cache/login/campus collector 的固定名称与有限 label 集合。
- 测试 login success、failure、override recovery、login-token recovery 和并发共享。
- 使用真实 promhttp handler 与 Gin route 测试 /metrics identity/gzip body 可解析。
- 测试 public Nginx exact /metrics 仍返回 404 的渲染合同。

### R4 — 兼容性

- 不改变 TokenManager token 选择、override 失效、singleflight 或 auth retry 行为。
- 不改变 /api/get_data、/readyz 或 RuntimeStatus schema。
- metrics 故障不能改变业务 outcome；nil/NoopMetrics 保持安全。
- 指标名称和既有低基数 labels 保持兼容。

## Acceptance Criteria

- [ ] Accept-Encoding: gzip 的 /metrics 响应解压一次即可解析，且不存在二次 gzip magic。
- [ ] identity 与 gzip;q=0 返回未压缩且可解析的 Prometheus text。
- [ ] bupt_ec_login_total 和 bupt_ec_login_duration_seconds 在真实登录边界产生 series。
- [ ] override 触发的 auth recovery 使用 source=override，其他网络登录使用 source=login。
- [ ] 并发 waiter 只对应一次 login counter/duration observation。
- [ ] login failure 不泄露错误、token、用户名、URL 或 log_id label。
- [ ] /metrics 后端可读、public Nginx 仍为 404。
- [ ] metrics 定向测试、go test -race ./...、go vet ./...、Go build 和 git diff --check 通过。

## Out of Scope

- 新增公网 metrics 认证、部署 Prometheus 或增加高基数业务指标。
- 更换 Gin gzip middleware 或增加 Brotli 等通用 API 压缩算法。
- 修改 login/token 协议、凭据解析或 RuntimeStatus 字段。
