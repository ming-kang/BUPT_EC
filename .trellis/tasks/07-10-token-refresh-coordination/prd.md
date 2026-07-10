# Token 并发刷新协调

## Goal

保证多个校区并发收到同一旧 token 的鉴权失败时只进行一次必要登录，并让迟到请求复用已刷新的 token。

## Requirements

- R1：鉴权恢复必须携带失败 token，只有缓存 token 仍等于该失败 token 时才清除它。
- R2：如果其他 goroutine 已经安装不同的新 token，迟到的旧请求必须直接复用新 token，不再 force login。
- R3：`JW_TOKEN` 只有在实际由 override 提供的 token 被鉴权拒绝时才标记为失效；登录获得的 token 过期不应错误改变 token source 状态。
- R4：singleflight 操作必须在 bounded context 中执行；某一个等待者取消不得使仍有效的其他等待者共享到被取消的登录。
- R5：每个调用者仍可根据自己的 context 提前退出等待。
- R6：queryCampus 仍最多重试一次，不增加无界鉴权循环。
- R7：日志不得包含 token 值或凭据。

## Acceptance Criteria

- [ ] 两个并发 campus 请求使用同一失效 token 时只调用一次 login。
- [ ] 第二个 auth failure 延迟到第一次 login 完成后返回时，仍复用第一次的新 token。
- [ ] JW 系统若在第二次登录时使第一次 token 失效，测试证明不会发生第二次登录，因此无互相失效。
- [ ] invalid `JW_TOKEN` 被拒绝后不会重新覆盖登录 token。
- [ ] 普通登录 token 过期不会错误恢复已失效 override。
- [ ] 一个 waiter cancellation 不会取消共享登录；该 waiter 自身及时返回 context error。
- [ ] `go test -race ./service` 通过，现有 integration tests 保持可跳过行为。

## Out of Scope

- 不持久化 token。
- 不改变 JW 登录请求格式或 AES 加密协议。
- 不增加超过一次的 query retry。
