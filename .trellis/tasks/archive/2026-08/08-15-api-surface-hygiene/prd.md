# PRD: readyz 公网收敛与成功信封 log_id（api-surface-hygiene）

来源：`08-07-project-audit-optimize` backlog 优先级 4（B-07 + B-11；B-08 独立另开）。

## 问题陈述

1. **B-07 `/readyz` 向公网返回完整 runtime 诊断**：`handler.go:65-79` 把
   `RuntimeStatus`（登录时间戳/错误、刷新状态、缓存明细、partial campuses、
   cache_date）加上 `jw_credentials_configured` 全量序列化给任何访问者，
   无差别暴露内部状态。
2. **B-11 成功信封缺 `log_id`**：`/api/get_data` 错误路径带 `log_id`
   （`handler.go:46-50`），成功路径只有 `code`/`data`（:55-58）；前后端
   排障不对称，前端已支持读 body `log_id`（`useTodayClassrooms.js:141`）。

## 运维依赖盘点（B-07 前置）

- `scripts/install.sh` / `install_test.sh`：无 readyz 字段依赖。
- `docs/operations.md:73-90, 255-259`：排障表引用 `last_login_error` /
  `last_refresh_error` / `cache_fresh` / `version`；`docs/upgrading.md:59`：
  `curl 127.0.0.1:8080/readyz | head -c 400` 冒烟。
- 结论：诊断字段确有运维用途 → 不能单纯删除，改为**显式开关**。

## 方案

### B-07：`READYZ_DIAGNOSTICS` 配置开关（默认关）

- 为什么不是 loopback 检测：主部署拓扑是同机 Nginx 反代 `127.0.0.1:8080`
  （deployment.md），到达 Go 的 `RemoteAddr` 恒为 loopback，检测形同虚设；
  X-Forwarded-For 可伪造。配置开关与拓扑无关、语义明确。
- 默认（false）：`/readyz` 只返回 `{"status": "...", "version": "..."}`。
- 开启（`READYZ_DIAGNOSTICS=1`，.env 或环境变量，随既有 config 模式）：
  返回现行为全量体（`jw_credentials_configured` + `runtime` + `version`）。
- readiness 判定逻辑与状态码不变（200/503 由 credentials+cache 决定）。

### B-11：成功信封补 `log_id`

- `GetData` 成功分支加 `"log_id": logs.GetLogIDFromContext(ctx)`，
  与错误信封对称；前端无需改动。

## 非目标

- 不做 B-08（utils HTTP 并入 service，独立 `utils-into-service` 任务）。
- 不改 `/healthz`、`/metrics`、gzip 探针豁免与 `X-Log-Id` 头。
- 不做 admin 鉴权路径（当前无鉴权基础设施，开关已满足收敛目标）。

## 验收标准

1. `READYZ_DIAGNOSTICS` 未设置/false 时，`/readyz` 响应体仅含
   `status` 与 `version`，不含 `runtime`/`jw_credentials_configured`；
   200/503 状态码行为与现状完全一致。
2. `READYZ_DIAGNOSTICS=true` 时响应体与现状逐字段一致（含 runtime 全量）。
3. 配置遵循现有模式：环境变量优先于 `.env`，非法值按 false 处理并留注释
   （对齐 `LogCaller` 解析风格），`config/` 有对应单测。
4. `/api/get_data` 成功信封包含非空 `log_id`（经 `Routes()` 中间件链请求），
   错误信封不变；handler 测试覆盖两条路径。
5. `go test -race ./...` 全绿；gofmt/vet 干净。
6. 文档同步：`docs/operations.md`（readyz 示例、排障表加「诊断需开
   READYZ_DIAGNOSTICS 或看日志」说明）、`docs/upgrading.md` 冒烟命令输出说明、
   `docs/development.md` / `.env.example` 若涉及配置表则补条目。
7. CHANGELOG.md Unreleased 记录两处变化（readyz 字段删减 + 成功信封 log_id）。

## 约束

- 契约变化仅限字段删减（有开关兜底）与加法（log_id），无端点增删。
- spec `api-contract.md` 与 `runtime-state-and-cache.md` 的 readyz 契约段落
  需同步更新（「Runtime Status and Readiness」节）。
