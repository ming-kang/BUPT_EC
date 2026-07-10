# 可靠性审计修复总任务 — Design

## Cross-Layer Data Flow

```text
JW API
  -> TokenManager / JWClient
  -> campus query results
  -> refresh outcome (full | partial | failed)
  -> same-day in-memory cache
  -> GET /api/get_data
  -> frontend response normalization
  -> business-day validity check
  -> classroom selection UI
```

部署流独立但共享 release 契约：

```text
installer version selection
  -> release URL
  -> archive + checksums download
  -> verification and staging
  -> atomic installation transaction
  -> systemd/nginx validation and restart
```

## Architectural Decisions

### Refresh result is tri-state

内部刷新结果使用明确的完整成功、部分成功、全量失败状态。公开 `TodayClassrooms.error` 继续作为安全的用户提示，但不再承担内部协调状态的唯一来源。

### API compatibility

保留现有字段和 HTTP 语义。允许增加可选字段，例如失败校区 ID 和 cache completeness 诊断；旧前端忽略新增字段仍可工作。原始 campus 错误只进入结构化日志，不进入客户端 JSON。

### One owner for business-day validity

前端创建单一有效性 helper，由 fetch 合并逻辑和 reload scheduler 共用。不得在两个模块分别实现日期和 `stale_until` 判断。

### Scheduler belongs to ClassroomService

warmup 的 timer、取消状态和 worker 生命周期继续属于 `ClassroomService`，不增加 package-level mutable state。进程 shutdown 先取消 scheduler，再等待进行中的 refresh。

### Preserve release default compatibility

安装器首次运行且没有任何版本信息时仍默认 `nightly`，保持当前公开约定。stable 文档命令必须显式传 `VERSION=latest`。安装器持久化 `RELEASE_VERSION`，重跑时按“显式环境变量 > 上次选择 > nightly”解析。

### Transaction boundary

下载、checksum 校验、解包和候选文件渲染属于 preflight，不得修改现有安装。替换二进制和配置属于 commit；commit 后验证失败则 rollback 到事务开始前快照。

## Dependency Order

- 刷新三态先于 warmup，因为 scheduler 需要判断完整、部分和失败结果。
- 刷新三态先于前端最终集成，因为前端要消费可选的部分校区诊断。
- token 协调与刷新模型代码相邻，但可以在刷新三态稳定后独立实施。
- 两个 installer 子任务可独立实施，但版本策略应先完成，以便事务测试使用稳定的版本解析入口。

## Compatibility and Rollout

- 所有 API 字段新增使用 `omitempty` 或明确默认值，旧消费者保持兼容。
- 缓存为进程内存，不需要持久化迁移；部署新二进制后自然重建。
- 安装器先保留旧二进制和配置备份，健康验证成功后再清理。
- 每个子任务独立提交，便于按子系统回滚。

## Rollback Shape

- 后端/前端：按子任务 commit 回滚，不涉及数据迁移。
- 安装器：事务内自动恢复旧文件；代码级回滚保持 release 资产名称不变。
- 若新增 API 诊断字段造成问题，可回滚消费者使用而保留字段，避免破坏兼容性。
