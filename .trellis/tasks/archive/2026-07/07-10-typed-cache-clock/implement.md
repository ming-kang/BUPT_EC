# 类型化缓存与统一时钟实施计划

## 1. Baseline and inventory

- [ ] 搜索 CacheStore、TodayCacheKey、Get/Set/Delete、type assertion 和所有 direct
      `time.Now/time.Since` production 调用。
- [ ] 固定 cache expiration、cross-day、status、login/refresh timestamp tests。

## 2. Typed cache

- [ ] 定义 `TodayClassroomCache` 领域接口。
- [ ] 在 cache package 实现独立 adapter，默认 NoExpiration、显式 Store expiration。
- [ ] 更新 main/service/tests 调用点，删除 generic key/type assertions/unused Delete。
- [ ] 验证不同 adapter 实例不共享数据。

## 3. Clock injection

- [ ] 定义 Clock/real implementation，加入 `ClassroomServiceOptions`。
- [ ] 将同一 clock 传给 TokenManager。
- [ ] 替换 cache、refresh/backoff、login/status 和 elapsed logging 的 direct time calls。
- [ ] 保持 business location conversion 集中。

## 4. Deterministic tests

- [ ] thread-safe fake clock。
- [ ] Shanghai midnight、expiration/backoff、login/refresh timestamps、elapsed duration。
- [ ] race tests 验证 clock/cache 在并发 refresh 下安全。

## 5. Docs/spec

- [ ] 更新 directory、runtime-state、logging、quality specs、AGENTS 和 development docs。
- [ ] 若无用户可见行为，不新增无意义 changelog；若 constructor API 影响贡献者则记录。

## Validation

```bash
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
govulncheck ./...
rg -n "CacheStore|TodayCacheKey|time\.Now\(|time\.Since\(" service cache
git diff --check
```

## Review Gates

- service 不得重新 import 具体 go-cache。
- cache adapter 不得承担 business-day policy。
- 无全局 clock setter 或共享 test cache。
- 所有 expiration 必须显式且与 StaleUntil 一致。

## Rollback Point

typed adapter + service migration + clock injection 形成一个原子重构；在通过全量 race tests
前不叠加 metrics/adaptive breaker 任务。
