# 协议与前端生命周期测试补强实施计划

## 1. AES vectors

- [ ] 用独立脚本生成并人工核对固定 vectors，不提交脚本中的任何秘密。
- [ ] 新增 crypto_test 覆盖完整链和 block/Unicode/empty cases。
- [ ] 确认 expected 不是调用 production helper 得出。

## 2. JWClient fixtures

- [ ] 建立 local server/doer fixture 和安全 JSON testdata。
- [ ] 覆盖 Login/FetchAPIURL/QueryCampus 请求合同。
- [ ] 覆盖 success/auth/query/parse/null/timeout/cancellation。
- [ ] 保持 URL validation 和 redirect/token-header 安全测试。

## 3. React lifecycle harness

- [ ] 添加 audit-clean 的 Testing Library/jsdom dev dependencies。
- [ ] 配置仅相关文件使用 jsdom，不无意改变所有 pure tests 环境。
- [ ] mount hook harness，使用 fake timers/controlled fetch。
- [ ] 覆盖 initial/background/manual/unmount/late response/timer cleanup/last-good-data。
- [ ] 确保 tests 在 StrictMode-compatible 重放下稳定。

## 4. Docs/spec

- [ ] 更新 AGENTS、development 和 quality/testing specs，说明 fixture/harness 模式。
- [ ] 不为内部-only tests 添加用户 changelog，除非引入 contributor-visible runner 变化。

## Validation

```bash
gofmt -l .
go vet ./...
go test -race ./...
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend lint
pnpm --dir frontend test
pnpm --dir frontend build
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
git diff --check
```

## Review Gates

- Known vector 必须独立于 production helper。
- Local protocol tests 不得触网或依赖 credentials。
- Hook tests 必须 mount/unmount，不能只是另一层 pure helper test。
- 不添加低价值 snapshot 或 runner smoke tests。

## Rollback Points

- Go crypto/protocol fixtures。
- Frontend test dependencies/config。
- Hook lifecycle harness。

三部分可独立提交，但 frontend lifecycle 实现任务开始前 harness 必须完成。
