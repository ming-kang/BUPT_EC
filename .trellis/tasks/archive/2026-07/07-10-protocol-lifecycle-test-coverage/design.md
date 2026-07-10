# 协议与前端生命周期测试补强设计

## AES Vector Strategy

新增 `service/crypto_test.go`，每个 vector 包含 plaintext、独立生成的 expected 和用途。
reference 生成过程写在注释或 `testdata` 说明中，但测试运行不依赖外部命令。

至少覆盖：

- 普通 ASCII password；
- 长度接近/跨 AES block boundary；
- Unicode；
- empty string（协议算法行为，不代表允许空凭据登录）。

## JW Protocol Fixture

优先使用 `httptest.Server` + 受控 HTTP client，使请求穿过实际 URL/form/header 编码层。
若 host validation 限制本地 URL，可对纯 protocol layer 使用 injected doer 捕获 request，且
不要放宽 production URL allowlist。

fixtures 放在 `service/testdata/` 或 inline small JSON；禁止真实 token/响应 dump。

## React Harness

增加 `jsdom` test environment 和一个只用于测试的 harness component：

```text
mount -> hook effect -> mocked fetch promise -> UI probe callbacks
rerender/retry -> new request
unmount -> abort + timer cleanup
```

使用 fake timers 和 controllable promises，断言调用次数、signal.aborted、可见 state 和 cleanup。
不依赖内部 hook state implementation detail。

## Dependency Choice

推荐 `@testing-library/react` + `jsdom`，版本与 React 18/Vitest 3 兼容并纳入现有 audit gate。
不引入完整 E2E runner。

## Compatibility and Rollback

- 测试任务默认不改变 production behavior。
- 若为 URL/fetch seam 做生产重构，必须独立说明并保持 public interfaces。
- 新 dev dependencies 可整体回滚，不影响 runtime bundle。
