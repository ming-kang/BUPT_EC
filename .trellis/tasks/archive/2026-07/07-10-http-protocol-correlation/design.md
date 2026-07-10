# HTTP 协议与日志关联设计

## Route Boundary

在 engine 级安装一个 path-aware API context middleware：

```text
request
  ├─ path == /api or /api/* -> ensure one log context + LogID header
  └─ other path             -> unchanged
             │
             ├─ registered API handler
             └─ NoRoute / method error -> JSON error with same log_id
```

移除 API group 上会造成重复生成的同类中间件，group 仍只负责路由组织。

## Accept-Encoding Parser

实现小型、无额外依赖的 helper：

```go
func acceptsGzip(header string) bool
```

- 以逗号拆分 coding，以分号解析参数。
- coding 与 `q` key 大小写不敏感。
- 默认 q=1；q 必须在 0..1，格式错误的 token 视为不可接受。
- 若存在显式 `gzip`，使用其 q；否则 `*;q>0` 可允许 gzip。
- identity fallback 永远可用，除非未来另有明确 406 需求（本任务不引入 406）。

`Vary: Accept-Encoding` 使用追加/去重语义，避免覆盖其他 Vary 值。

## Detached Refresh Context

启动 attempt 时以发起 context 为基底：

```text
request ctx --context.WithoutCancel--> values-only ctx
           --context.WithTimeout(ClassroomRefreshLimit)--> refresh ctx
```

这保留 log ID 等 values，同时去除请求 deadline、Done 和取消原因。共享 attempt 固定
使用首次发起者 context；等待者只等待 `attempt.done`。

## Error Envelope

未知 API 路由使用现有 JSON 404 code/msg，并增加 `log_id`。`LogID` header 和 body
从同一个 context 获取。若未来启用 405，也复用同一 helper。

## Compatibility and Rollback

- 成功 `/api/get_data` body 不新增 log_id，仅 header 合同保持。
- 未知 API body 是向后兼容的字段增加。
- 若 parser 有问题，可独立回滚 gzip helper；日志中间件和 refresh context 改动互不依赖，
  但最终提交前必须全量验证路由顺序。
