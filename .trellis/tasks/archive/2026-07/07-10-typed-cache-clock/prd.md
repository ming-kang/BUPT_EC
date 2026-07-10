# 类型化缓存与统一时钟

## Goal

用类型化今日教室缓存替代通用 CacheStore，并将缓存、刷新、登录和状态统一到可注入时钟。

## Background

- `service/classroom_service.go:28-34` 的 `CacheStore` 暴露 string key、`interface{}` 和
  未使用的 `Delete`，service 调用点需要类型断言。
- `cache/cache.go:9-11` 设置默认 5 分钟 TTL，但 classroom service 每次写入都传显式
  到上海午夜的 expiration；默认 TTL 对生产语义具有误导性。
- `ClassroomService` 已有内部 `now func()`，但 constructor options 不公开该依赖。
- `service/realtime_data.go:115-131` 与 `token_manager.go` 仍直接使用
  `time.Now/time.Since`，runtime timestamps、backoff 和测试时间源不完全一致。

## Requirements

### R1 — Service 只依赖类型化今日缓存

- 新接口只表达读取/写入 `*model.TodayClassrooms`，不暴露 cache key、interface{} 或未用
  Delete。
- `cache` package 提供独立实例 adapter；main 仍创建唯一生产实例并注入。
- 所有写入继续使用显式 expiration 到 `StaleUntil`，默认 TTL 不参与业务合同。
- 保持 process-local、单 key、指针值和跨日验证现有行为，除非设计明确增加安全 copy。

### R2 — 默认 TTL 语义明确

- 底层 go-cache 使用 `NoExpiration` 或其他不会伪装业务 TTL 的默认值；每次 store 必须
  显式传 expiration。
- cleanup interval 可以保留，但必须与业务 freshness 区分并在 spec 注明。

### R3 — 注入统一 Clock

- `ClassroomServiceOptions` 接受可选 Clock；nil 使用真实时钟。
- cache date/expiry、refresh/backoff、login/runtime timestamp 和 elapsed logging 通过同一
  clock 获取时间。
- 所有业务日期在读取 clock instant 后转换为 Asia/Shanghai；clock 本身不隐含时区。
- timer/cancellation 可继续使用真实 Go timer，本任务不构建完整虚拟 scheduler。

### R4 — 测试和兼容

- fake clock 驱动跨日、TTL、backoff、status、login/refresh timestamps，无需修改全局时间。
- 删除 CacheStore 类型断言和 unused Delete，搜索全仓确认无旧接口残留。
- API payload、TTL 数值、singleflight、warmup 和 no-database 架构保持。

## Acceptance Criteria

- [ ] service production/test code 不再使用 string-key/interface{} `CacheStore`。
- [ ] cache adapter 只能存取 `*model.TodayClassrooms`，实例之间不共享状态。
- [ ] 所有 production writes 显式使用到 `StaleUntil` 的 expiration。
- [ ] refresh、backoff、login、runtime status 和 cache policy 使用同一 injected clock。
- [ ] 跨上海午夜、elapsed/status 和 concurrency tests 可确定性运行。
- [ ] 没有 package-level mutable clock/cache 或 test-only production env 开关。
- [ ] gofmt、vet、race、build、govulncheck 和完整服务测试通过。

## Out of Scope

- Redis、磁盘缓存或跨进程共享。
- 完整 fake timer/scheduler 框架。
- 修改 fresh TTL、stale-until 或业务日定义。
