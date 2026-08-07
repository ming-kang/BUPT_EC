# Design：审计方法与产物边界

## 任务边界

本任务是**只读审计**：读当前 `main` 与历史审计/规格，写任务目录下的 `research/` 报告。不改 `service/`、`frontend/`、`scripts/`、CI 等产品面。

## 输入

| 来源 | 用途 |
|------|------|
| 当前 Go/React/脚本/CI/docs | 新证据与 file:line 锚点 |
| `.trellis/spec/backend/*`、`guides/*` | 现行契约与质量门禁，避免建议违反非协商规则 |
| `archive/.../07-26-arch-simplify-refactor/research/audit-{backend,frontend,engineering}.md` | 对照基线；避免重复已落地结论 |
| 父任务 PRD 中批次④清单 | 遗留项再判定 |
| `go.mod` / `frontend/package.json` / `Taskfile.yml` / workflows | 依赖与工程链路现状 |

## 输出结构

```text
.trellis/tasks/08-07-project-audit-optimize/
  research/
    audit-backend.md
    audit-frontend.md
    audit-engineering.md
    backlog.md          # 跨三份报告的优先级总表（可选但推荐）
```

每条发现建议统一字段：

- **ID**（如 B-01 / F-01 / E-01）
- **标题**
- **状态相对 07-26**：已落地 | 仍有效 | 已过时 | 新发现
- **类别**：性能 | 可维护性 | 可靠性 | DX
- **证据**：`path:line` 或命令输出摘要
- **建议**、**收益**、**风险**、**工作量**（S/M/L）、**契约影响**（无 / 需 CHANGELOG / 破坏性）
- **建议后续任务切分**（一句话）

## 审计透镜（不漏项）

### Backend

- 缓存新鲜度模型与刷新协调（含手写 singleflight vs `x/sync/singleflight`）
- 生命周期 / 取消传播 / 冷启动阻塞
- HTTP 层：ETag/预序列化、SPA index 缓存、readyz 暴露面
- 包边界：`utils/`、根目录平铺、锁与状态分组
- 测试债与 `export_test` 白盒耦合
- 热路径分配与重复 Marshal/gzip

### Frontend

- SWR 落地后是否仍有重复手写逻辑或契约漂移
- antd 剩余体积 vs 实际组件用法；chunk/预算
- Picker 重复、派生状态、无障碍与 ErrorBoundary
- React 18→19 / TypeScript / prop-types 迁移债
- 测试厚度 vs 生产代码复杂度是否失衡

### Engineering

- Taskfile ↔ quality.yml ↔ docs 同步
- release/nightly、版本注入、可复现构建
- install.sh / 测试框架体量
- Dockerfile/goreleaser 等批次④工程项是否仍值得做
- agent/Trellis 入库占比（仅建议，不在本任务改）

## 优先级启发式

1. **高收益低风险、且仍未落地**优先（尤其批次④中可独立切的项）。
2. **性能**：优先减少上游 JW 压力与重复序列化；用户侧感知次之（已有 SWR/缓存时勿过度优化）。
3. **可维护性**：优先删重复与收敛模型，其次引入新工具链（TS、goreleaser）。
4. 违反 spec 非协商规则的建议一律降级或标为「需产品批准的架构变更」。

## 兼容与回滚

- 无运行时变更；「回滚」= 丢弃或修订报告文件。
- 报告中的实现建议必须标明契约影响，供后续实现任务引用。

## 不做的设计

- 不在本任务写具体补丁设计（那是后续实现任务的 `design.md`）。
- 不把 backlog 自动拆成子任务目录（用户批准实现时再开）。
