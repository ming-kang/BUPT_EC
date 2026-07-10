# 单实例部署与扩展路线设计

## Current Reference Topology

```text
Internet
   |
Nginx (TLS, headers, rate limit)
   |
127.0.0.1:8080
   |
one bupt-ec process
   ├─ one TokenManager
   ├─ one refresh coordinator/warmup scheduler
   └─ one process-local TodayClassrooms cache
```

这是当前唯一明确支持和推荐的生产形态。

## Current Failure Semantics

- process restart：token/cache/status 清空，warmup 重建；
- JW outage：同日 cache 可 stale 返回，跨日不可复用；
- host failure：服务不可用，当前没有 active-active 数据层；
- 多进程：各自 login/refresh/backoff，Nginx 轮询可能返回不同 snapshot/readiness。

## Future Option Matrix

| Option | Coordination | Advantages | Costs/Risks |
| --- | --- | --- | --- |
| Keep one active instance | systemd + optional standby | 最简单、符合当前代码 | host 级单点 |
| Shared typed cache + distributed lock | Redis/cache + lock TTL | 多 API 实例、复用 snapshot | lock fencing、secret/state split、Redis ops |
| Leader/fetcher + read replicas | elected fetcher writes snapshot | JW 压力最可控 | leader election、failover、snapshot schema/versioning |

未来设计必须保证 Shanghai-day、partial outcome、stale-until 和 token 不落入不安全共享存储。

## Documentation Placement

- README：一句部署定位和链接；
- deployment：参考拓扑和安装目标；
- operations：failure/restart/multi-instance 行为；
- development：进程内 ownership；
- specs：禁止把 local cache/singleflight 当作 distributed primitive。

## Compatibility and Rollback

纯文档任务，不改变运行时。若未来架构决定变化，应通过新的 Trellis design task 更新此路线，
而不是在普通修复中顺带引入共享状态。
