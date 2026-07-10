# Warmup 生命周期与跨日重试

## Goal

让 startup/午夜 warmup 具备明确的启动、重试和停止生命周期，在没有用户请求的情况下也能从临时上游故障恢复当日缓存。

## Requirements

- R1：warmup scheduler 必须接受可取消 context，不得使用无法中断的永久 `time.Sleep`。
- R2：启动时立即尝试 warmup；之后按 Asia/Shanghai 下一午夜加小幅 jitter 调度。
- R3：午夜尝试因 refresh backoff 未启动时，必须在允许时间到达后重试，而不是等待下一天。
- R4：当日无缓存且上游持续失败时采用 30s、1m、2m、5m 上限的后台退避，直到获得可用当日缓存或进程停止。
- R5：部分缓存可以使 readiness 为 true，但 scheduler 应以低频（不快于 fresh TTL）继续尝试恢复完整缓存。
- R6：shutdown 必须先停止 scheduler，再等待已启动 refresh worker；停止后不得再 `WaitGroup.Add` 新 worker。
- R7：多次调用 StartWarmup 不得创建多个 scheduler。
- R8：计时决策必须可确定性测试，不依赖等待真实午夜。

## Acceptance Criteria

- [x] backoff 横跨午夜时，测试证明 scheduler 在 `nextRefreshAllowed` 后重试。
- [x] 当日无缓存时连续失败使用有上限退避，成功后切换到下一午夜调度。
- [x] 部分缓存后不会每 30 秒永久打 JW，但会在合理间隔尝试补全。
- [x] cancel context 会停止 sleeping scheduler，并且 `WaitBackground`/替代接口可靠返回。
- [x] StartWarmup 重复调用只运行一个 scheduler。
- [x] race detector 不报告 WaitGroup Add/Wait 竞态。
- [x] main.go graceful shutdown、operations/development 文档和 backend runtime spec 更新。

## Out of Scope

- 不引入 cron、外部队列或独立 scheduler 服务。
- 不让 `/healthz` 或 `/readyz` 主动访问 JW。
- 不绕过 refresh coordinator 的 single-flight 和 backoff。
