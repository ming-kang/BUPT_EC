# 前端跨日缓存与轮询退避 — Implementation Plan

## Implementation

- [ ] 搜索现有上海日期、stale window 和 response validity helper，复用 `formatShanghaiDate`。
- [ ] 新建共享 `isUsableBusinessDaySnapshot`，添加 focused tests。
- [ ] 修改 `mergeFetchResult` 接收可注入 `nowMs`，仅保留有效同日快照。
- [ ] 为 hook 增加连续失败计数和 hard-error 自动重试。
- [ ] 重构 `nextReloadDelay`/新增 `failureRetryDelay`，实现 5/10/20/30 秒退避。
- [ ] 将 partial polling 与后端 30 秒 backoff 对齐。
- [ ] 在 App warning 中使用可选 `partial_campuses`，更新刷新时间文案。
- [ ] 更新现有 Vitest，补充 merge + schedule 组合回归。

## Focused Tests

- [ ] same-day valid prev + failure => preserve。
- [ ] previous-day prev + failure => hard empty。
- [ ] stale_until elapsed + failure => hard empty。
- [ ] hard empty automatically schedules retry。
- [ ] repeated failures produce capped delays and success resets count。
- [ ] partial error uses 30-second poll；stale in-flight uses short poll。
- [ ] malformed date/timestamp fails closed without throwing。

## Validation

```powershell
cd frontend
pnpm lint
pnpm test
pnpm build
```

随后回到仓库根目录运行 `go test ./...`，确认嵌入前端构建未影响 Go 构建输入。

## Rollback Point

所有状态只存在浏览器内存。若自动重试出现问题，可回滚 hook/scheduler commit；后端新增字段保持 optional，不会阻断旧前端。
