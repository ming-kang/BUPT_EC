# 自适应刷新退避抖动与确定性测试

## Goal

在保持 30s/1m/2m/5m total-failure 基础阶梯和现有 cache/singleflight 语义的前提下，
增加有界、可注入、可确定性测试的 refresh jitter，并让业务时间测试真正经过公开
ClassroomServiceOptions.Clock 边界。

## Background

- service/refresh_coordinator.go:36 定义固定 totalFailureBackoffSteps。
- finishClassroomRefresh 在 refreshMu 下直接用 base delay 设置 nextRefreshAllowed。
- 原 observability 任务明确要求 bounded injectable jitter 和 fake clock/random 测试。
- ClassroomServiceOptions 已暴露 Clock，但 ClassroomService 同时保留可变 now func 字段。
- 现有测试主要直接覆盖 svc.now，而不是注入 thread-safe fake Clock，TokenManager 与
  ClassroomService 的统一时钟合同没有被实际验证。
- 现有 refresh_backoff_test.go 只测试纯基础阶梯，没有 suppression、reset、partial、
  concurrency 或 midnight interaction。

## Dependencies

- 必须在 metrics-endpoint-login-observability 完成后实施，因为两者共享
  ClassroomServiceOptions、TokenManager 构造和 RuntimeMetrics。
- 不依赖 frontend、installer 或 Unicode 子任务。

## Requirements

### R1 — 基础阶梯保持

- consecutive total failure 的 base delay 固定为 30s、1m、2m、5m，超过四次保持 5m base。
- full success 立即把 consecutiveTotalFailures 重置为 0 并清除 nextRefreshAllowed。
- partial success 不递增 total failure count，使用现有固定 30s soft backoff，不施加
  total-failure jitter。
- backoff 内 stale/partial cache 继续返回；无 cache 时继续返回安全错误。

### R2 — 有界 jitter

- jitter 策略由 refresh coordinator 的一个纯 helper 统一拥有。
- 注入边界只提供 0..1 随机样本，策略负责校验和限幅，调用者不能注入任意大 duration。
- 推荐映射为对称 offset：正负 min(base 的 10%，5 秒)。
- NaN、Inf、越界样本必须 clamp 或回退到 0.5，不能产生负 delay 或时间溢出。
- 每个完成的 total failure 只采样一次，并在 refreshMu 保护下原子计算 nextRefreshAllowed。
- production 默认使用并发安全的随机源；测试注入固定样本。

### R3 — 单一 Clock

- ClassroomService 的业务时间只能来自 options.Clock，nil 使用 systemClock。
- 移除或封装测试可直接替换的 now function 字段，避免 service 与 TokenManager 使用
  两套时间。
- 测试 fake Clock 必须 thread-safe，可显式 Set/Advance。
- refresh start/completion、backoff、cache day、RuntimeStatus 和 login timestamps 必须
  使用同一 injected Clock instant；真实 timer 只用于等待/cancellation。

### R4 — 确定性状态测试

- 使用 fake Clock 和固定 random，不使用 sleep 推进核心 backoff 状态。
- 覆盖基础阶梯、jitter 上下界、cap、无效样本、full reset 和 partial 行为。
- 覆盖 request、warmup 和 concurrent callers 共享同一 nextRefreshAllowed。
- 覆盖 total failure 发生在上海午夜前、nextAllowed 跨午夜以及新日 cache miss 的重试。
- 覆盖 suppression 不创建 worker，并准确发射一次 suppression metric。

## Acceptance Criteria

- [x] total failure base ladder 保持 30s/1m/2m/5m，effective delay 始终在文档化边界内。
- [x] 每次 total failure 只读取一次 random sample，非法样本不会产生非法时间。
- [x] full success 清零阶梯，partial 保持 30s soft backoff 且不使用 total jitter。
- [x] request/warmup/concurrent callers 不能绕过 coordinator backoff。
- [x] 跨午夜 nextAllowed 到达后能够刷新新业务日，昨天 cache 永不复用。
- [x] 所有业务时间测试通过 ClassroomServiceOptions.Clock，不再直接覆写生产 now seam。
- [x] race tests、定向 backoff tests、完整 Go tests、vet/build 和 git diff --check 通过。

## Out of Scope

- 引入通用 circuit-breaker library、持久化 breaker 状态或多实例协调。
- 修改 fresh TTL、stale window、partial payload、warmup 基础重试阶梯或 HTTP timeout。
- 给 frontend jitter 复用 Go 策略；两端只共享合同，不共享实现。
