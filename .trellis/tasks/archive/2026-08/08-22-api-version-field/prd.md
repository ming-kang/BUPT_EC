# API 信封暴露运行版本并在设置弹窗显示

父任务：`08-22-ops-experience`

## Goal

让用户在网页 UI 上直接看到当前正在运行的后端版本，无需 SSH 登录服务器查 `/readyz`。

## Requirements

### R1 后端：信封暴露 version

- `/api/get_data` 的响应信封新增顶层字段 `version`，取值为 `main.go` 的 `version` 变量（release 构建为 tag，本地构建为 `dev`）。
- **成功与失败信封都要带**。失败态下设置弹窗仍可打开，用户此时更需要知道版本用于报障。
- 字段与 `log_id` 平级，属于信封元信息，不进入 `data` 内部——`data` 是业务快照，版本是服务端元数据，混入会污染缓存快照的语义。

### R2 前端：数据通路

复刻既有 `logId` 的透传路径，不新建机制：

- `api/types.ts` 的 wire 类型加 `version?: string`
- `useTodayClassrooms.ts` 的 `HookEnvelope` 加 `version?: string`，在构造信封时从 `record?.version` 透传
- **注意陷阱**：`useTodayClassrooms.ts` 中 `mergeFetchResult` 会重建 hard-empty 信封并丢失 `logId`（见该文件既有注释「re-attach」）。`version` 必须在同一处一并重新附加，否则空数据态下版本会消失。

### R3 前端：设置弹窗渲染

- 在 `CampusSettingsModal.tsx` 的「项目已开源」行**上方**插入一行，与既有两行使用同一套 `Typography.Text type="secondary"` 加 `lineHeight: "1.9em"` 样式。
- 文案：`当前运行版本：v0.3.0`
- **缺失时整行不渲染**（不显示「未知」）。老后端不返回该字段时留空洞对用户无意义；升级后自然出现。
- 本地构建显示 `dev` 属预期行为，对开发有用，不特殊处理。

### R4 不做的事

- 不让前端调用 `/readyz`（v0.2.0 刚将其收敛为运维端点，公网前端不应依赖）
- 不用 Vite 构建期注入版本（前端在 `quality-gate` job 构建、tag 在 `build-go` job 注入，两者分离；且「运行版本」语义上应来自正在运行的后端）
- 不改动 `data` 内部任何字段

## Acceptance Criteria

- [ ] `/api/get_data` 成功响应信封含 `version`，值等于二进制注入的版本
- [ ] `/api/get_data` 失败响应（503 冷启动、无缓存）信封同样含 `version`
- [ ] Go 测试覆盖成功与失败两条路径的 `version` 存在性
- [ ] 前端设置弹窗在「项目已开源」上方显示「当前运行版本：<版本>」
- [ ] 服务端未返回 `version` 时该行不渲染，且无 React 警告
- [ ] 前端测试覆盖：有版本时渲染、无版本时不渲染、空数据态（hard-empty 重建后）版本不丢失
- [ ] `task check`、`task test` 全绿；bundle 仍在 230,888 B 预算内
- [ ] CHANGELOG `[Unreleased]` 的 `Added` 记录该用户可见变更

## Notes

- 带宽代价：每次轮询多约 20 字节，与既有 `log_id` 同量级，可忽略
- 与 backlog 中挂起的 `api-etag-preserialize`（B-04）不冲突：`version` 是进程生命周期内的常量，不影响未来按 snapshot×variant 键控的预序列化方案
- 无前置依赖，可最先实施
