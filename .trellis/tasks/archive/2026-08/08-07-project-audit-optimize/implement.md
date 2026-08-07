# Implement：审计执行清单

## 前置

- [ ] `task.py start` 仅在用户批准本规划摘要之后执行
- [ ] 工作树产品代码保持干净；只写入本任务目录

## 执行顺序

1. **基线对照表**
   - 通读 `archive/2026-07/07-26-arch-simplify-refactor/research/audit-{backend,frontend,engineering}.md`
   - 列出批次④遗留项 + 07-26 未做/部分做条目为核对清单
2. **Backend 只读审计** → `research/audit-backend.md`
   - 重点：`service/realtime_data.go`、`refresh_coordinator.go`、`token_manager.go`、`warmup.go`、`classroom_service.go`、`router.go`/`handler.go`/`main.go`、`utils/`、相关 `*_test.go`
3. **Frontend 只读审计** → `research/audit-frontend.md`
   - 重点：`useTodayClassrooms.js`、`reloadSchedule.js`、components、`vite.config.js`、`package.json`、bundle 预算脚本与测试分布
4. **Engineering 只读审计** → `research/audit-engineering.md`
   - 重点：`Taskfile.yml`、`.github/workflows/*`、`scripts/`、`docs/`、release 链路、依赖审计配置
5. **汇总 backlog** → `research/backlog.md`
   - 跨层排序；标注建议后续任务粒度与依赖（文字依赖即可）
6. **自检（对照 Acceptance Criteria）**
   - 每条有锚点与 R3 字段；批次④全覆盖；无产品代码 diff

## 验证命令（审计期可选，用于证据）

```bash
go test -race ./...
pnpm -C frontend lint
pnpm -C frontend test
# 需要体积证据时：
pnpm -C frontend build && pnpm -C frontend size
go list -m all
```

不要求本任务必须把 `task check` 跑绿作为验收（无代码改动）；若跑命令，结果写入报告作证据摘要即可。

## 风险与回滚点

- **风险**：把已落地项再当新债 → 用对照表强制标「已落地」。
- **风险**：建议违反「无本地课表 DB / 单进程拓扑」→ 标为架构变更，默认不进高优先级。
- **回滚**：删除或回退 `research/` 文件即可。

## start 前闸门

- [x] `prd.md` 已收敛；交付形态 A
- [x] `design.md` / `implement.md` 齐备
- [x] `implement.jsonl` / `check.jsonl` 含真实条目（非仅 `_example`）
- [ ] 用户显式批准本最终规划摘要
