# Token 并发刷新协调 — Design

## Token State

为内存 token 记录来源：

```go
type tokenSource int

const (
    tokenSourceNone tokenSource = iota
    tokenSourceOverride
    tokenSourceLogin
)
```

`setToken` 同时设置 value/source。这样只有 override token 被拒绝时才设置 `overrideInvalidated=true`。

## Auth Failure API

替代 queryCampus 中的“clear + EnsureToken(force=true)”组合：

```go
func (m *TokenManager) RefreshAfterAuthFailure(ctx context.Context, failedToken string) (string, error)
```

singleflight closure 内再次检查：

1. 当前 token 非空且不等于 `failedToken`：返回当前 token；其他请求已刷新。
2. 当前 token 等于失败 token：按 source 清除并在需要时失效 override。
3. 执行一次真实 login，存为 login source。

检查必须在 singleflight closure 内完成，避免进入 group 前后的竞态窗口。

## Cancellation

使用 `singleflight.DoChan`：

- shared operation 使用 `context.WithTimeout(context.WithoutCancel(ctx), jwRequestTimeout)`，保留 log values 但不继承首个 waiter cancellation；
- 每个调用者 `select` shared result 与自己的 `ctx.Done()`；
- API URL singleflight 采用相同模式，避免首个 caller cancellation 污染其他请求。

## Query Flow

```text
EnsureToken(false)
  -> QueryCampus
  -> auth failure?
       no: return
       yes: RefreshAfterAuthFailure(failedToken)
            -> retry QueryCampus once
```

## Security

错误和日志只记录操作、耗时和分类 error。任何测试失败输出也不得打印真实环境 token；测试使用固定假 token。
