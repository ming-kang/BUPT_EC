# 指标端点与登录可观测性实施计划

## 0. Characterization

- [ ] 新增真实 Prometheus handler 的 /metrics gzip 回归，确认修复前解压一次仍是 gzip。
- [ ] 新增 login collector 缺少 series 的基线断言。
- [ ] 记录现有 API gzip、health/readiness 和 Nginx /metrics deny 测试基线。

## 1. Fix metrics encoding

- [ ] 在 main.go 的 promhttp HandlerOpts 中禁用内部压缩。
- [ ] 保持 router gzipMiddleware 为唯一 gzip owner。
- [ ] 测试 identity、gzip、gzip;q=0、wildcard 和混合 coding。
- [ ] 断言 Content-Encoding、Vary、Content-Length 和单次解压后的 text format。

## 2. Wire login metrics

- [ ] 为 TokenManager 注入 RuntimeMetrics，并保持 nil-safe/default 行为。
- [ ] 定义并实现 recovery decision，保留 rejected tokenSource。
- [ ] 让 loginAndStore 接收 trigger source 并在共享操作内记录一次 success/failed。
- [ ] 使用 injected Clock 计算 duration，并防御负 duration。
- [ ] 确认 override 安装、token cache hit、waiter 和 replacement reuse 不重复计数。

## 3. Metric assertions

- [ ] 新增 isolated registry gather helper。
- [ ] 覆盖 login source=override/login、outcome=success/failed。
- [ ] 覆盖 histogram sample count 和非负 duration。
- [ ] 覆盖并发 singleflight 只增加一次 counter。
- [ ] 覆盖 label 中不出现 raw error、token、username、URL 或 log_id。
- [ ] 覆盖 refresh/cache/campus 既有 metric family 未回退。

## 4. Validation

~~~powershell
$env:GOTOOLCHAIN='go1.25.12'
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
bash scripts/install_test.sh
git diff --check
~~~

## 5. Contract sync and evidence

- [ ] 更新 operations、logging/quality specs 和 AGENTS 的 metrics/login 合同。
- [ ] 在同一实现 commit 更新 CHANGELOG [Unreleased]。
- [ ] 记录定向测试和完整验证结果，commit 产生后再填写 task metadata。

## Review Gates

- /metrics 测试必须使用真实 promhttp.HandlerFor，不接受固定 fake body。
- 不通过完全关闭所有 metrics gzip 来掩盖双重压缩，除非设计文档同步说明。
- login metric 必须按网络操作计数，而不是按调用者或 waiter 计数。
- source 必须来自真实 token provenance，不能仅根据是否配置 JW_TOKEN 推断。
- 不增加高基数或秘密 label。

## Rollback Points

- promhttp compression ownership。
- TokenManager metrics injection 和 recovery decision。
- isolated registry/endpoint tests 应随对应行为保留，不能在回滚时删除失败证据。
