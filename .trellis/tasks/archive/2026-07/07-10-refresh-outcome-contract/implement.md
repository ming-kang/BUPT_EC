# 刷新三态与部分数据契约 — Implementation Plan

## Implementation

- [x] 在 `service/refresh_coordinator.go` 或新的 focused file 中定义 refresh kind/result/failure 类型。
- [x] 重构 `doRefreshTodayClassrooms`，返回三态 outcome 并保留 campus ID 对应错误。
- [x] 在 partial payload 中填充 `partial_campuses`，保持 campuses 配置顺序。
- [x] 重构 `refreshTodayClassrooms` 的 status/log 更新，分别处理 full/partial/failed。
- [x] 重构 `finishClassroomRefresh`，根据 outcome 设置 backoff 和 last error。
- [x] 删除 `preferAPIError`，按 attempt/backoff 状态选择最新错误。
- [x] 扩展 `RuntimeStatus`，区分 cache freshness 与 completeness。
- [x] 更新 handler/runtime tests、API model consumer 文档和 changelog。

## Focused Tests

- [x] full outcome clears warning/backoff。
- [x] partial outcome caches successful campus and marks failed campus IDs。
- [x] partial outcome reuses only same-day previous campus data。
- [x] partial stale cache followed by total failure exposes total stale error。
- [x] partial cache remains usable for readiness but runtime reports partial。
- [x] concurrent callers share one outcome under race detector。

## Validation

```powershell
gofmt -w service
go test ./service ./...
go test -race ./service
go vet ./...
```

在提交前还需运行父任务定义的全量前端和后端检查。

## Rollback Point

该任务不包含持久化迁移。若三态模型引起回归，可整体回滚本子任务 commit；新增 JSON 字段不得被后续子任务设为强制必需，直到父任务完成。
