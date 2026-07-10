# 可靠性审计修复总任务 — Implementation Plan

## Ordered Child Execution

1. `07-10-installer-version-policy`
   - 先修复会直接误导生产部署的 stable/nightly 命令。
2. `07-10-refresh-outcome-contract`
   - 建立后续 warmup 和前端降级行为所依赖的刷新契约。
3. `07-10-frontend-cache-validity`
   - 消费刷新诊断并统一跨日快照与轮询策略。
4. `07-10-warmup-lifecycle`
   - 基于三态结果和 backoff 实现可取消恢复调度。
5. `07-10-token-refresh-coordination`
   - 收敛并发登录和 caller cancellation 语义。
6. `07-10-installer-transaction-safety`
   - 在版本解析稳定后完成安装事务和回滚测试。

## Parent Review Gates

- [x] 每个子任务在 `task.py start` 前具有收敛后的 `prd.md`、`design.md`、`implement.md`。
- [x] 每个子任务完成 focused tests 后再运行其相关全量检查。
- [x] 后端公开模型变更同步检查 frontend consumer、handler tests、docs 和 changelog。
- [x] installer 变更同步检查 release workflow 的资产名称与归档布局。
- [x] 最后一次集成检查从 clean baseline 区分本任务改动与用户已有未提交文件。

## Final Validation

```powershell
$bad = gofmt -l .
if ($bad) { $bad; exit 1 }
go vet ./...
go test -race ./...
cd frontend
pnpm lint
pnpm test
pnpm build
cd ..
bash -n scripts/install.sh scripts/release.sh scripts/extract-changelog.sh
shellcheck scripts/*.sh
git diff --check
```

如果本机缺少 `shellcheck` 或 `govulncheck`，必须明确记录，并确保 CI workflow 覆盖相同命令。

## Integration Review Checklist

- [x] 没有跨日旧数据进入教室表格或选择器。
- [x] partial/full/failed 的日志、runtime status、payload 和 backoff 相互一致。
- [x] shutdown 不遗留可继续启动 worker 的 warmup scheduler。
- [x] token 恢复不会因迟到的旧请求执行第二次强制登录。
- [x] stable/nightly/tag 三种安装路径与文档一致。
- [x] 安装 preflight 失败零持久化修改，commit 失败可恢复。
- [x] README、docs、CHANGELOG、AGENTS 和 `.trellis/spec/backend/` 描述最终真实行为。

## Commit Boundaries

- `fix(install): align stable and nightly version selection`
- `refactor(service): model full partial and failed refresh outcomes`
- `fix(frontend): enforce business-day snapshot validity`
- `fix(service): make warmup lifecycle cancellable and retryable`
- `fix(service): coordinate concurrent auth token refresh`
- `fix(install): make upgrades transactional`
- 必要的 docs/spec 更新随所属行为提交，不单独积压到最后。
