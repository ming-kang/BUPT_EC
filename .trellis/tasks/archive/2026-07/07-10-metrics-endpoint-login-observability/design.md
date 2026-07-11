# 指标端点与登录可观测性设计

## Compression Boundary

采用一个明确的编码所有者：

~~~text
request Accept-Encoding
  → router gzipMiddleware negotiates gzip
  → promhttp HandlerFor with DisableCompression=true emits identity text
  → outer writer optionally gzip-encodes once
~~~

main.go 构造 promhttp.HandlerFor 时设置 DisableCompression。这样无需为 /metrics 添加
特殊路由分支，也继续复用项目已经测试过的 Accept-Encoding parser、Vary 和
Content-Length 清理逻辑。

handler endpoint 测试必须使用真实 Prometheus registry，而不是只写固定字符串的 fake
handler，否则无法发现 handler 内部再次压缩的问题。

## Login Observation Boundary

TokenManager 是唯一拥有共享登录 singleflight 的边界，因此埋点必须发生在
loginAndStore 的 singleflight function 内：

~~~text
EnsureToken / RefreshAfterAuthFailure
  → determine trigger source
  → tokenGroup shared operation
  → loginAndStore(ctx, triggerSource)
      → startedAt = Clock.Now
      → APIURL + JWClient.Login
      → ObserveLogin exactly once
  → N waiters receive shared result
~~~

TokenManager 新增 RuntimeMetrics 依赖，nil 时使用 NoopMetrics 或显式 nil-safe 调用。
ClassroomService 构造时把与 service 相同的 metrics 实例传入 TokenManager。

## Source Semantics

prepareAuthRecovery 需要在清除 token 之前保留 rejected token 的真实 tokenSource。
建议返回一个 recovery decision：

~~~text
reusable token result
needs login boolean
login trigger source
~~~

- rejected override → trigger source override；
- rejected login token、force login、无 token 登录 → trigger source login；
- delayed failure 复用已安装 replacement → 不创建登录、无 observation。

不为 overrideToken 的零网络安装记录 login success，避免 duration=0 的伪登录样本。

## Outcome and Duration

- startedAt 位于共享 loginAndStore 开始处。
- completedAt 始终从同一 injected Clock 读取。
- success 在 token 安装和 status callback 完成后观察。
- failed 在 APIURL 或 Login 返回 error 时观察。
- duration 小于零时防御性归零，避免错误 fake clock 污染 histogram。

## Test Design

1. PrometheusMetrics isolated registry：
   - 直接观察每种有限 label；
   - Gather 后检查 counter、histogram count/sum 和 label 集合。
2. TokenManager：
   - 首次登录 success/failed；
   - override 被拒绝后的 recovery；
   - login token 被拒绝后的 recovery；
   - 多 waiter 共享一次 observation；
   - delayed old failure 复用 replacement，不重复 observation。
3. HTTP：
   - identity body 含预期 metric family；
   - gzip body 解压一次是 text，不再以 1f 8b 开头；
   - gzip;q=0 identity；
   - health/readiness 仍不压缩。
4. Installer：
   - 继续断言 public location = /metrics return 404。

## Compatibility and Rollback

- 指标接口不新增高基数参数。
- 若登录 metrics 引发回归，可独立回滚 TokenManager 注入，但 endpoint compression 修复
  应保留。
- 若全局 gzip 行为以后变化，/metrics 测试仍负责阻止双重编码复发。
