# 类型化缓存与统一时钟设计

## Typed Cache Boundary

在 `service` 定义领域接口：

```go
type TodayClassroomCache interface {
    Load() (*model.TodayClassrooms, bool)
    Store(value *model.TodayClassrooms, expiration time.Duration)
}
```

`cache` package 提供实现并内部拥有 `*gocache.Cache`。它可以 import
`service/model`，但不能 import `service`，避免循环依赖。service 继续负责：

- Shanghai-day validation；
- fresh/stale 判断；
- cache metadata；
- expiration 计算。

adapter 不再暴露 `TodayCacheKey` 或 generic type assertions。底层默认 expiration 使用
`gocache.NoExpiration`，业务写入必须显式 duration。

## Clock Boundary

建议接口：

```go
type Clock interface {
    Now() time.Time
}
```

`ClassroomServiceOptions.Clock` 可选；真实实现调用 `time.Now()`。service helper 将 instant
转换到 `businessLocation`。同一 Clock 传给 TokenManager，供 login timestamps 和 elapsed
计算使用。

`time.Since(start)` 改为 `clock.Now().Sub(start)`；测试 clock 必须单调前进且 thread-safe。

## Ownership

```text
main
 ├─ real Clock
 ├─ cache.TodayClassroomCache
 └─ ClassroomService(options{Clock}, cache, JWClient)
       └─ TokenManager(clock)
```

只有 service instance 持有可变运行状态；不引入全局 setter。

## Migration

1. 新增 typed adapter 与 tests。
2. 切换 service constructor/interface 和 cache helpers。
3. 注入 Clock 并替换 direct time calls。
4. 删除旧 CacheStore、key/type assertions/default TTL assumptions。

## Compatibility and Rollback

- 内存值和过期时间保持原样，API 无迁移。
- adapter/clock 变更应在同一可构建提交中完成。
- 如需回滚，恢复旧 interface 时不得保留一半 direct clock 调用。
