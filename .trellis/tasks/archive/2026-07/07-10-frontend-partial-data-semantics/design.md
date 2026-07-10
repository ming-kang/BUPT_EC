# 前端部分数据选择语义设计

## Pure Selection Contract

新增纯 helper，例如：

```js
chooseCampusId({ campuses, partialCampusIds, selectedCampusId })
```

并定义：

```js
hasCampusSnapshot(campus) = buildings.length > 0 || nodes.length > 0
```

选择算法：

1. 若当前 campus 存在，且未 partial 或仍有 snapshot data，则保持。
2. 从非 partial campuses 中选择“沙河”，否则第一个非 partial campus。
3. 若所有 campus 均 partial，选择第一个仍有 snapshot data 的 campus。
4. 最后 fallback 到列表第一项；空列表返回空 ID。

该规则同时覆盖冷 partial 占位和带同日旧缓存的 partial 数据，避免只看
`partial_campuses` 就丢弃仍可用信息。

## Component Integration

`AppContent` effect 只比较 helper 返回 ID 与当前 ID；不同才 dispatch
`SET_CAMPUS`。纯函数放在独立 `.js` 文件，保持 component 文件只导出 component，符合
ESLint react-refresh 约束。

## Metadata Label

不改 payload：

```text
updated_at -> 当前数据更新时间
error/stale alert -> 最近刷新结果/警告
```

这样 total failure 后仍准确表达当前展示数据的年龄。

## Compatibility

- 不改变 selection reducer action 和 localStorage keys。
- 不改变 campus order 或后端 partial merge。
- 用户主动选中的、有旧数据的失败 campus 继续可看。

## Rollback

helper、App effect 和文案可以作为单个前端提交回滚，不影响 API 或缓存。
