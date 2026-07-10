# 前端部分数据选择语义

## Goal

部分校区成功时默认选择有可用数据的校区，并校正刷新时间等用户可见语义。

## Background

- `frontend/src/App.jsx:73-86` 只在选中校区 ID 不存在时重新选择，并无条件优先名称
  为“沙河”的校区。
- 后端部分刷新会保留失败校区的同日旧数据；冷启动没有旧缓存时则通过
  `service/realtime_data.go:201-205` 生成 buildings/nodes 为空的占位校区，并在
  `partial_campuses` 标记失败 ID。
- 当沙河冷启动失败而西土城成功时，当前逻辑仍选中空沙河，使有效部分结果看起来像
  全局无数据。
- `CampusSettingsModal.jsx:54-59` 把 payload `updated_at` 标为“最近刷新尝试时间”，
  但 total failure 不会更新缓存时间；它实际代表当前缓存内容的生成时间。

## Requirements

### R1 — 选择真正可用的校区

- 把默认/修复选择规则提取为纯函数并使用 ID，而不是依赖组件内临时分支。
- 当前选择仍有实际 buildings/nodes 数据时应保持，即使该校区本轮被标记 partial，
  因为它可能是后端合并的同日旧缓存。
- 当前选择为空占位且另一个非失败校区可用时，自动切换到可用校区。
- 新选择优先级：可保留的当前选择 → 非 partial 的沙河 → 第一个非 partial 校区 →
  有旧数据的校区 → 稳定 fallback。
- 不修改用户手动选择，只在当前选择缺失或已成为无数据失败占位时纠正。

### R2 — 时间文案与 payload 语义一致

- 将 `updated_at` 标为“当前数据更新时间”或等价准确文案，不再声称是最后一次尝试。
- total failure 上的 warning/error 与旧数据时间可以同时显示，不能伪造不存在的尝试时间。
- 本任务不新增后端时间字段；若产品需要 attempt timestamp，应另建 API 合同任务。

### R3 — 行为测试

- 覆盖冷 partial 首选校区失败、首选成功、当前失败校区有旧数据、全部 campus 空、
  campus 列表变化和用户现有选择保持。
- 使用纯函数单元测试和至少一个 App/Provider 行为测试保护 effect 集成。
- 同步用户文档、API spec 和 changelog。

## Acceptance Criteria

- [ ] 沙河失败且无旧数据、西土城成功时默认显示西土城。
- [ ] 沙河失败但携带同日旧 buildings 时，不会无理由清空/切走用户当前选择。
- [ ] 当前选择从 payload 消失时仍有稳定 fallback。
- [ ] `updated_at` 文案不再表示“刷新尝试时间”。
- [ ] Alert、campus warning、building/class-time selection 和 table filtering 保持兼容。
- [ ] 前端 lint、相关 Vitest、完整 44+ 测试和 production build 通过。
- [ ] 不修改后端 JSON schema、缓存或刷新逻辑。

## Out of Scope

- 请求 timeout、visibility/polling 和 reducer 持久化重构。
- 新增“最后一次刷新尝试”后端字段。
- 改变校区产品偏好（有可用数据时仍优先沙河）。
