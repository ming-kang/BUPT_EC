# 运行指标与自适应上游保护实施计划

## 0. Preconditions

- [ ] `07-10-typed-cache-clock` 已完成并提供 fake clock。
- [ ] 固定当前 refresh/warmup/backoff/runtime status 测试基线。

## 1. Metrics seam

- [ ] 定义低基数 `RuntimeMetrics` interface 和 no-op implementation。
- [ ] main 创建 private Prometheus registry/collectors 并注入。
- [ ] 在 refresh、campus query、cache serve、login recovery 和 suppression 边界埋点。
- [ ] 注册 `/metrics`，Nginx exact path 默认拒绝公网代理。

## 2. Adaptive policy

- [ ] 提取 total/partial/full backoff policy 为纯函数或小状态对象。
- [ ] 注入 jitter，阶梯固定为 30s/1m/2m/5m cap。
- [ ] coordinator 在 mutex 下原子更新 failure count/next allowed/metrics。
- [ ] warmup 继续通过 coordinator，不复制 breaker 状态。

## 3. Tests

- [ ] isolated registry 指标值和 label 集合。
- [ ] full/partial/total/suppressed duration/counter/gauge。
- [ ] adaptive escalation、cap、success reset、partial behavior、midnight interaction。
- [ ] concurrent request/warmup race 与 shutdown。
- [ ] `/metrics` backend 可读、Nginx public path 被拒绝。

## 4. Docs/spec

- [ ] 更新 operations 的 scrape/alert 示例和 deployment 的访问边界。
- [ ] 更新 runtime-state、logging、API、quality specs、AGENTS、README/changelog。

## Validation

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
bash scripts/install_test.sh
shellcheck scripts/*.sh
govulncheck ./...
git diff --check
```

## Review Gates

- 无 package-level mutable registry。
- 无 secrets/high-cardinality labels。
- metrics 采集失败不得改变业务 outcome。
- adaptive backoff 不得阻止 stale cache 返回或跨午夜恢复。

## Rollback Points

- Metrics interface/registry/endpoint。
- Adaptive policy/coordinator state。
- Nginx/docs/alerts。

每阶段需通过 race 与 deterministic clock tests。
