# 运行指标与自适应上游保护

## Goal

提供可告警的刷新、缓存、登录与延迟指标，并以确定性的自适应退避或断路保护 JW 上游。

## Background

- `service/runtime_status.go` 只暴露最后成功/错误和当前 cache 状态，没有累计计数、延迟
  分布或可供告警系统抓取的指标。
- `service/refresh_coordinator.go:84-94` 对 total/partial 结果统一设置固定 30 秒 backoff；
  连续上游故障不会逐级降载。
- warmup 已有 30s/1m/2m/5m missing-cache 重试，但 request-triggered refresh 与其策略
  分离，且没有统一连续失败状态。
- 当前单实例服务适合先采用进程内 metrics 和小型 deterministic protection state，不需要
  分布式 breaker。

## Dependencies

- `07-10-typed-cache-clock` 先完成统一 clock 和 typed cache，供 backoff/metrics 测试使用。
- HTTP/startup safety 子任务先稳定路由与错误分类，再固定 metric labels。

## Requirements

### R1 — 可抓取、无敏感信息的运行指标

- 提供 Prometheus-compatible `/metrics`，使用独立 registry 而不是不可隔离的全局测试状态。
- 至少覆盖：refresh attempt/outcome/duration、failed campus、cache serve state、login/auth
  recovery outcome/duration、backoff suppression 和 in-flight refresh gauge。
- labels 必须是固定低基数枚举或固定 campus ID；不得包含 token、用户名、原始错误、URL、
  log ID 或房间名。
- 现有 `/readyz` RuntimeStatus 保留面向人工诊断的 snapshot，不复制高基数时序数据。

### R2 — 指标默认不通过公网 Nginx 暴露

- 后端 loopback 地址可供同主机 Prometheus 抓取。
- 生成的 Nginx 配置对外部 `/metrics` 明确拒绝或不代理；文档给出本机 scrape 示例。
- 不因为 metrics 不可用而阻止核心 API 启动，除非 registry 构造本身失败。

### R3 — 自适应 refresh backoff

- full success 重置连续 total-failure 计数和 open 状态。
- partial success 保持可用数据并使用至少 30 秒固定退避，不按 total failure 无限升级。
- total failures 使用 30s、1m、2m、5m 封顶的确定性阶梯，并加入可注入的小 jitter。
- backoff 内请求继续返回同日 stale/partial cache；无 cache 时返回现有安全错误。
- 所有 request、warmup 和 concurrent callers 继续共享 singleflight/backoff 状态。

### R4 — 测试与运维合同

- 使用 fake clock/jitter 和 isolated registry 测试计数、duration、reset、suppression、partial
  和 concurrency，不使用 sleep 驱动核心状态测试。
- 更新 operations/deployment/development、AGENTS、runtime/quality specs 和 changelog。
- 给出建议告警：连续 total failure、长时间无 fresh cache、登录失败和刷新延迟接近 budget。

## Acceptance Criteria

- [ ] `/metrics` 输出低基数 refresh/cache/login 指标且不含任何秘密或原始上游消息。
- [ ] Nginx 外部路径不能直接读取 metrics，本机 loopback scrape 有文档和测试。
- [ ] 连续 total failures 按 30s/1m/2m/5m 封顶，full success 立即 reset。
- [ ] partial outcome 仍缓存并返回可用数据，不触发不合理长时间 open circuit。
- [ ] concurrent request/warmup 不绕过 singleflight 或重复计算 breaker 状态。
- [ ] existing API、readyz schema、cache day boundary 和 graceful shutdown 保持。
- [ ] race tests、metrics assertions、installer render tests 和完整质量门禁通过。

## Out of Scope

- 部署 Prometheus/Grafana/Alertmanager 服务。
- 分布式 rate limiter、Redis breaker 或多实例 leader election。
- 对每个教室、用户或 log ID 建立指标 label。
