# 刷新三态与部分数据契约

## Goal

让课堂数据刷新以明确的完整成功、部分成功、全量失败三态运行，使缓存、backoff、错误提示、日志和 `/readyz` 诊断表达同一事实。

## Requirements

- R1：内部 refresh outcome 必须包含状态、成功 payload、失败校区及全量错误，不能仅依赖 `TodayClassrooms.Error` 推断协调状态。
- R2：完整成功更新全部校区缓存、清除 refresh warning/error，并取消失败 backoff。
- R3：部分成功必须缓存可用校区，失败校区复用同日旧数据或空骨架，并设置 30 秒 backoff。
- R4：全量失败不得覆盖现有缓存；没有可用同日缓存时返回安全的 503 错误。
- R5：过期的部分缓存在最新刷新全量失败后，客户端必须看到最新的 stale/upstream failure，而不是旧的“部分校区失败”提示。
- R6：部分结果必须记录 Warn 日志，包含失败校区 ID 和内部分类错误；不得记录 token、密码或原始响应体。
- R7：运行状态必须区分 cache age freshness 与 completeness；部分缓存仍可 ready，但不能被诊断为完整成功。
- R8：公开 payload 可新增向后兼容的 `partial_campuses`，用于前端指出受影响校区；现有字段和 HTTP 状态保持兼容。
- R9：`updated_at` 定义为最近刷新尝试完成时间；部分复用旧校区时文档和 UI 不得将其描述成所有校区数据的新鲜时间。

## Acceptance Criteria

- [x] 完整成功、部分成功、全量失败均有独立单元测试。
- [x] “部分缓存过期 -> 最新全量失败”测试返回 stale failure 提示。
- [x] 部分结果测试验证 cache、backoff、`partial_campuses` 和失败校区数据合并。
- [x] `/readyz` 测试验证部分缓存 ready 且 runtime 标记 incomplete/partial。
- [x] 日志测试或可注入观测验证部分结果不会只记录为普通成功。
- [x] 并发请求仍共享一个 refresh attempt，`go test -race ./service` 通过。
- [x] API、operations/development 文档、CHANGELOG 和 backend specs 同步更新。

## Out of Scope

- 不改变 JW 查询协议或增加新的校区。
- 不对每个房间增加独立 freshness 元数据。
- 不引入持久化缓存。
