# 刷新三态与部分数据契约 — Design

## Internal Types

在 `service/` 内定义不可序列化的刷新状态：

```go
type refreshKind int

const (
    refreshFull refreshKind = iota
    refreshPartial
    refreshFailed
)

type campusRefreshFailure struct {
    CampusID string
    Err      error
}

type classroomRefreshResult struct {
    value    *model.TodayClassrooms
    kind     refreshKind
    failures []campusRefreshFailure
    err      error
}
```

`err` 只表示全量失败；部分失败保留在 `failures`，避免 Go 调用方把可用 payload 当成错误丢弃。

## Public Contract

`model.TodayClassrooms` 增加：

```go
PartialCampuses []string `json:"partial_campuses,omitempty"`
```

`RuntimeStatus` 增加可选诊断字段，例如：

```go
CachePartial       bool     `json:"cache_partial"`
PartialCampuses    []string `json:"partial_campuses,omitempty"`
LastRefreshWarning string   `json:"last_refresh_warning,omitempty"`
```

安全的 `APIError` 继续面向客户端；内部 `failures` 只用于日志和状态更新。

## State Transitions

```text
full:
  cache = new complete payload
  lastRefreshErr = nil
  nextRefreshAllowed = zero
  runtime full-success timestamp updated

partial:
  cache = new partial payload
  lastRefreshErr = nil
  nextRefreshAllowed = now + 30s
  runtime warning + partial campus IDs updated

failed:
  cache unchanged
  lastRefreshErr = joined classified error
  nextRefreshAllowed = now + 30s
  runtime failure updated
```

## Response Error Precedence

- in-flight refresh 超过 300ms：返回缓存自身 warning；尚无最新失败结论。
- attempt 完成且 failed：返回 `staleAPIError(attempt.err)`，覆盖缓存旧 warning。
- backoff 且 `lastRefreshErr != nil`：返回 `staleAPIError(lastRefreshErr)`。
- partial attempt 完成：直接返回新的 partial payload。

因此移除通用的 `preferAPIError(primary, fallback)`，改为按 refresh state 显式选择。

## Logging

- full：`InfoContext("classroom refresh succeeded", ...)`
- partial：`WarnContext("classroom refresh partially succeeded", "failed_campuses", ids, "errors", joinedErr, ...)`
- failed：保留现有 failure Warn。

日志内容不得包含请求 header、token 或原始 JW body。

## Compatibility

新增 JSON 字段为 additive。旧前端继续依赖 `error` 和 `campuses`；新前端可使用 `partial_campuses` 提示具体校区。
