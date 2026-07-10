# 前端跨日缓存与轮询退避 — Design

## Shared Validity Boundary

新增 focused helper（建议 `frontend/src/classroomDataValidity.js`）：

```js
export function isUsableBusinessDaySnapshot(data, nowMs = Date.now())
```

它负责：

- 校验 `campuses`；
- 用现有 `formatShanghaiDate` 比较业务日期；
- 解析并检查 `stale_until`；
- 对缺失/非法关键字段 fail closed。

`mergeFetchResult` 和 `nextReloadDelay` 都导入此 helper，不各自维护日期规则。

## Fetch State

hook 增加连续失败计数：

```text
successful full/partial HTTP payload -> reset transport failure count
fetch/non-OK/normalization failure   -> increment failure count
manual retry                         -> immediate, does not erase valid snapshot
```

`mergeFetchResult(prev, next, nowMs)`：

- next 成功且缓存元数据有效：直接替换；
- next 失败且 prev 仍是有效业务日：保留 prev，设置 client_refresh_failed；
- next 失败且 prev 无效：返回 hard error + `data:null`。

## Scheduling

调度优先级：

1. hard error/no usable data：`failureRetryDelay(count)`。
2. client refresh failure：相同退避。
3. partial payload：30 秒。
4. stale payload、刷新可能仍在进行：5 秒。
5. fresh payload：等待 `expires_at`，最少 1 秒。

无论处于哪种调度状态，delay 都不得越过 `stale_until`；到达边界时先
从 UI state 清除失效快照，再发起后台 reload。

退避为 5/10/20/30 秒，暂不加入随机 jitter，以保持小型单实例服务和测试确定性。

## UI

- hard-empty 时 `App` 已会清空 campuses，并通过 `GlobalEmpty` 展示错误。
- partial payload 使用 `partial_campuses` 生成更具体的 warning；字段不存在时使用现有通用 message。
- 设置弹窗的 `updated_at` 文案改成“最近刷新尝试时间”，避免部分数据时暗示全部校区已更新。

## Compatibility

保持 `useTodayClassrooms` 对外返回的 `resp/spinning/isError/retry`。内部可新增 retry state，不要求组件消费。
