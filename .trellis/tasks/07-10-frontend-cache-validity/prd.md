# 前端跨日缓存与轮询退避

## Goal

保证前端只保留仍属于当前上海业务日的可用快照，并在服务降级时使用有界轮询恢复，而不是展示昨日教室或形成高频请求。

## Requirements

- R1：旧快照只有在 `date` 等于当前上海日期、`stale_until` 有效且尚未到期、`campuses` 为数组时才能在失败后继续展示。
- R2：跨日或超过 `stale_until` 的快照必须从 UI state 中移除，返回 hard-empty error envelope。
- R3：业务日有效性必须由一个共享 helper 所有，fetch 合并和 reload scheduler 不得重复实现日期判断。
- R4：首次加载失败、跨日失效后失败以及连续后台失败都必须继续自动重试。
- R5：失败重试采用 5s、10s、20s、30s 上限的退避；成功后计数重置。
- R6：部分校区 error 的轮询不得快于后端 30 秒 backoff；普通 stale-while-revalidate 可在短间隔检查刷新结果。
- R7：后台 reload 不得触发全页 Spin；没有可用快照时继续展示明确的错误/重试 UI。
- R8：如果后端提供 `partial_campuses`，警告应展示受影响校区名称或 ID；字段缺失时保持兼容。

## Acceptance Criteria

- [x] 同日有效快照在网络失败后保留并标记 stale。
- [x] 昨日快照在刷新失败后被清空，教室表格和筛选器不再显示昨日内容。
- [x] 已超过 `stale_until` 的快照在失败后被清空。
- [x] hard-empty 状态仍按失败退避自动重试，而不是停止调度。
- [x] 连续失败 delay 为 5s/10s/20s/30s，成功后恢复正常 fresh expiry 调度。
- [x] 部分 payload 不会每 5 秒向处于 30 秒 backoff 的后端发送无效请求。
- [x] Vitest 覆盖 merge 与 scheduler 的组合场景，前端 lint/test/build 通过。
- [x] 用户可见警告和缓存行为同步更新 CHANGELOG 与 operations 文档。

## Out of Scope

- 不增加 Service Worker 或浏览器持久化 classroom payload。
- 不重新设计选课/筛选组件。
- 不在前端猜测 JW 错误类型或解析原始上游错误。
