# Warmup 生命周期与跨日重试 — Design

## Service Lifecycle

推荐 API：

```go
func (s *ClassroomService) StartWarmup(ctx context.Context)
func (s *ClassroomService) WaitBackground(ctx context.Context) error
```

`StartWarmup` 用 `sync.Once` 保护单 scheduler。main 使用 `signal.NotifyContext` 或等效应用 context；收到退出信号时 cancel，再 shutdown HTTP server，最后等待 background workers。

## Scheduler State Machine

```text
start
  -> refresh now
  -> full cache: sleep until next Shanghai midnight + jitter
  -> partial cache: sleep max(nextRefreshAllowed, freshTTL)
  -> no cache/failed: sleep max(nextRefreshAllowed, exponential retry)
  -> context canceled: exit
```

无缓存退避：30s、1m、2m、5m cap。成功后 retry counter 清零。部分缓存采用至少 5 分钟间隔，避免无人访问时持续高频请求。

## Timer and Testability

将“下一次等待多久”抽成纯函数，例如：

```go
func nextWarmupDelay(now time.Time, cacheState warmupCacheState, nextAllowed time.Time, failures int) time.Duration
```

loop 使用 `time.NewTimer` + `select` 等待 timer 或 context。纯函数覆盖跨午夜/backoff/退避测试；取消测试只需短期 context，不等待真实业务时间。

## Coordination Safety

- scheduler 自身使用单独 WaitGroup 或 goroutine completion channel。
- refresh worker WaitGroup 只在 scheduler 尚未停止时 Add。
- shutdown 顺序保证 Wait 开始前不会再创建新 worker。
- `startClassroomRefresh` 保持 single-flight；scheduler 可 join 已在进行的请求刷新。

## Runtime Behavior

readiness 继续只读取状态，不触发外部 I/O。scheduler 恢复缓存后，下一次 `/readyz` 自然变为 200。
