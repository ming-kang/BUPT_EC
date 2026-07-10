# 运行指标与自适应上游保护设计

## Metrics Architecture

`main.go` 创建独立 Prometheus registry 和 service collector，并注入单个
`ClassroomService`/HTTPServer：

```text
ClassroomService events -> RuntimeMetrics interface -> Prometheus collectors
HTTPServer /metrics      -> promhttp.HandlerFor(private registry)
```

测试可注入 no-op 或 isolated registry，避免 package-level mutable metrics。

建议指标：

| Metric | Type | Labels |
| --- | --- | --- |
| `bupt_ec_refresh_total` | counter | outcome=`full|partial|failed|suppressed` |
| `bupt_ec_refresh_duration_seconds` | histogram | outcome |
| `bupt_ec_refresh_in_flight` | gauge | none |
| `bupt_ec_campus_query_failures_total` | counter | campus=`01|04`, kind |
| `bupt_ec_cache_serves_total` | counter | state=`fresh|stale|partial|miss` |
| `bupt_ec_login_total` | counter | outcome, source=`override|login` |
| `bupt_ec_login_duration_seconds` | histogram | outcome |

错误 kind 只能使用现有有限枚举；未知值归一为 `jw_unavailable`。

## Exposure Boundary

- Gin 注册 `GET /metrics`。
- 默认 `APP_ADDR=127.0.0.1:8080` 支持本机 scrape。
- installer Nginx 增加 exact `/metrics` deny/404，普通公网 location 不转发它。
- metrics 不携带 request log ID，也不记录 credentials configuration 值。

## Adaptive Protection State

在 refresh coordinator mutex 下维护：

```text
consecutiveTotalFailures
nextRefreshAllowed
lastOutcome
```

策略：

- full -> failures=0, no backoff；
- partial -> failures 不递增，30s backoff；
- total n -> delay=[30s,1m,2m,5m]，n>4 保持5m；
- delay 加入 bounded、可注入 jitter；
- suppression 产生 metrics，但不创建 worker。

这是一种小型 soft circuit，而不是引入通用 breaker library。缓存服务语义保持：open
期间仍返回可用 stale data。

## Warmup Integration

warmup 计算自己的 cache-state deadline，但启动 refresh 前仍必须经过 coordinator。
coordinator 的 `nextRefreshAllowed` 是最终下限，避免两个 backoff 相乘或绕过。

## Compatibility and Rollback

- `/readyz` 不删除字段。
- 新 `/metrics` 是只读运维端点且默认不公开代理。
- metrics 与 protection policy 分两个内部接口，若需要可独立回滚采集而保留 backoff。
