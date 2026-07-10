# 协议与前端生命周期测试补强

## Goal

补充 JW AES 已知向量、JWClient 确定性协议夹具和真实 React hook 生命周期回归测试。

## Background

- `service/crypto.go` 实现 JSON marshal → PKCS#7 → AES-ECB → base64 两次编码，但没有
  独立已知向量测试；现有测试用同一个 `encryptJWPassword` 生成 expected，无法发现算法
  整体漂移。
- `service/jw_client_test.go` 主要覆盖 injected credentials/login 和 nil client，未完整固定
  QueryCampus method、path、query、token header 和真实响应 envelope。
- 前端现有 44 个测试集中在 pure helper/merge/schedule；`useTodayClassrooms` 的 mount、effect
  cleanup、真实 timer/fetch abort 和 visibility lifecycle 没有通过 React runtime 驱动。
- 真实 JW integration tests 因凭据缺失而正确 skip，但 CI 需要一条永不触网、永不 skip 的
  protocol contract 路径。

## Requirements

### R1 — 独立 AES 已知向量

- 为固定非敏感 plaintext 硬编码 ciphertext expected，覆盖 JSON quoting、PKCS#7、AES-ECB
  block iteration 和 double-base64 全链。
- expected 必须由独立实现（OpenSSL/Node/WebCrypto reference script）生成并在测试注释中
  记录方法，不能调用 production helper 生成 expected。
- 增加 block boundary、Unicode 和 empty password vectors；不修改协议 key。

### R2 — JWClient 本地协议夹具

- 使用 httptest 或 injected HTTPDoer 固定 Login、FetchAPIURL、QueryCampus 的 method、URL、
  form/query/header 和 JSON parse/error classification。
- QueryCampus 必须断言 `POST /todayClassrooms?campusId=...` 与 token header。
- 覆盖 success、auth failure、non-auth failure、invalid data、null data 和 context timeout。
- 所有 tests 本地运行，不需要 JW credentials 或公网。

### R3 — 真实 React hook lifecycle harness

- 使用 jsdom + React Testing Library（或同等最小工具）实际 mount 一个消费
  `useTodayClassrooms` 的 harness component。
- 覆盖 initial load、background reload、manual retry、unmount abort、timer cleanup、late
  response、last-good-data 和 StrictMode-compatible effect behavior。
- harness 可被后续 timeout/visibility 子任务扩展；避免 snapshot-only tests。

### R4 — 测试价值与边界

- 只增加保护协议、安全和 race-prone lifecycle 的测试，不追求覆盖率数字。
- production 代码只允许为可测试 seam 做最小、无行为变化调整。
- 同步 testing/quality specs、AGENTS 和 development docs。

## Acceptance Criteria

- [ ] AES 全链 known vector 在 production 算法任何关键步骤变化时失败。
- [ ] 至少一条 QueryCampus 完整协议测试永不触网、永不 skip。
- [ ] auth/parse/timeout 分类通过本地 fixture 确定性验证。
- [ ] hook tests 实际 mount/unmount，并断言 abort、timer 和 state 更新，而非只测纯 helper。
- [ ] 后续 frontend lifecycle task 可以直接复用 harness 添加 timeout/visibility cases。
- [ ] 现有 integration tests 仍在无凭据时清楚 skip。
- [ ] Go race、frontend lint/test/build 和 audits 全部通过。

## Out of Scope

- 在 CI 存储真实 JW 用户名、密码或 token。
- 浏览器 E2E、Playwright 或 screenshot 测试。
- 为低风险展示组件追求全面覆盖率。
