# HTTP 协议与日志关联

## Goal

修正 gzip 协商、未知 API 路由 LogID 覆盖和共享刷新日志关联，同时保持现有 API 兼容。

## Background

- `router.go:91-109` 用 `strings.Contains` 判断 `Accept-Encoding`，因此会压缩
  `gzip;q=0`、忽略大小写/权重规则，也没有针对拒绝 gzip 的回归测试。
- `router.go:48-58` 只给已注册 `/api` group 安装日志中间件；`NoRoute` 生成的
  `/api` JSON 404 没有 `LogID` header 或 body `log_id`。
- `service/refresh_coordinator.go:66-69` 为共享刷新使用
  `context.Background()`，正确地避免请求取消杀死共享工作，但同时丢失发起请求的
  `log_id` 值。
- SPA fallback、`/healthz`、`/readyz` 和 `/api/get_data` 的现有公开语义必须保持。

## Requirements

### R1 — 正确协商 gzip

- 对 `Accept-Encoding` 按逗号分隔的 coding 和 `q` 权重解析，大小写不敏感。
- `gzip;q=0` 必须拒绝 gzip；显式 gzip 权重优先于 `*`。
- 缺失、格式错误或只允许其他编码时返回 identity，不应产生 5xx。
- 被压缩响应设置 `Content-Encoding: gzip` 和 `Vary: Accept-Encoding`，删除错误的
  `Content-Length`；health/readiness 继续不压缩。

### R2 — 所有 API 路径拥有日志关联

- `/api`、`/api/*` 的已知路由、未知路由和不支持方法都必须获得一个且仅一个 log ID。
- API 404/405 JSON 错误同时返回 `LogID` header 和 body `log_id`。
- 非 API SPA fallback 不应被改成 JSON，也不强制暴露 log ID。

### R3 — 共享刷新保留发起关联但不继承取消

- 请求触发的新 refresh worker 保留发起请求 context values，包括 log ID。
- worker 仍由 `ClassroomRefreshLimit` 控制，不继承请求取消或 deadline。
- 后续共享等待者不能覆盖该 attempt 的关联，也不能因自身取消而终止共享刷新。
- warmup 发起的刷新允许没有请求 log ID，但必须继续有结构化日志。

### R4 — 回归测试与合同同步

- 增加 gzip 权重/大小写/wildcard/拒绝矩阵、API 404/405 log ID 和共享刷新 context
  测试。
- 同步 API、logging、runtime spec、AGENTS、docs 和 changelog。

## Acceptance Criteria

- [ ] `Accept-Encoding: gzip;q=0`、`GZip;Q=0` 和显式拒绝场景不压缩。
- [ ] `gzip`、正权重 gzip 和允许 gzip 的 wildcard 场景按合同压缩。
- [ ] `/api` 与任意未知 `/api/*` 返回 JSON 404、非空且一致的 header/body log ID。
- [ ] 已知 API 请求只生成一个 log ID，不因中间件重组重复覆盖。
- [ ] 请求触发的 refresh 日志可以关联到发起请求，且请求取消不停止共享 worker。
- [ ] SPA fallback、health/readiness、公开 payload 和 refresh singleflight/backoff 不变。
- [ ] `gofmt`、`go vet ./...`、`go test -race ./...` 和 `git diff --check` 通过。

## Out of Scope

- 引入完整 HTTP content-negotiation 框架或新的压缩算法。
- 为 health/readiness 增加请求日志 ID。
- 改变刷新结果、缓存 TTL、API 字段或用户错误文案。
