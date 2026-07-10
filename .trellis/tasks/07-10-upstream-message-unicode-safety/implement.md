# 上游消息 Unicode 清洗加固实施计划

## 0. Characterization

- [ ] 新增 U+0085、U+2028、U+2029 保留的失败测试。
- [ ] 新增 Unicode whitespace 绕过 token/password regex 的失败测试。
- [ ] 固定现有普通中文、Bearer、rune limit 和 SafeErrorMessage 基线。
- [ ] 搜索所有 JW response message 进入 error/log 的调用点。

## 1. Unicode normalization

- [ ] 提取 isUnsafeRemoteRune 或等价集中 helper。
- [ ] 使用 Unicode category/space 判断覆盖 C0/C1/Zl/Zp/Cf。
- [ ] 连续 unsafe runes 折叠为一个 ASCII space。
- [ ] 保持普通 rune 和标点不变，并在 normalization 后 trim。

## 2. Redaction and bounds

- [ ] 保持 normalize → redact → truncate → fallback 顺序。
- [ ] 验证 ASCII/CJK key/value 与 Bearer pattern 在规范化空格上工作。
- [ ] 覆盖 replacement 靠近 256 rune 边界。
- [ ] 确认 fallback 不包含原始 payload。

## 3. Regression tests

- [ ] C0/DEL/C1 和 line/paragraph separator matrix。
- [ ] Unicode spaces 和 format/bidi controls matrix。
- [ ] Unicode whitespace sensitive-key bypass matrix。
- [ ] 普通中文、emoji、标点兼容。
- [ ] Fuzz seed output invariants。
- [ ] slog JSON 单行、可解析、无 secret integration test。
- [ ] API error body 仍只返回固定安全文案。

## 4. Validation

~~~powershell
$env:GOTOOLCHAIN='go1.25.12'
gofmt -l .
go vet ./...
go test -race ./service ./logs ./...
go build ./...
git diff --check
~~~

## 5. Contract sync and evidence

- [ ] 更新 error/logging specs、operations/AGENTS 的 Unicode sanitizer 合同。
- [ ] 在同一安全修复 commit 更新 CHANGELOG [Unreleased]。
- [ ] 记录 table/fuzz/logging/race 验证结果和实现 commit 后再归档。

## Review Gates

- Unicode 判断必须集中，不能在多个 regex 中复制字符集合。
- redaction 必须发生在 truncation 前。
- 测试必须检查 rune category invariant，不只检查 CR/LF substring。
- 不把真实 runtime credentials 传给 sanitizer。
- 不改变 SafeErrorMessage、JW error kind 或 auth retry。

## Rollback Points

- unsafe rune classifier。
- normalization/redaction ordering。
- fuzz/logging integration tests。
- C1/Zl/Zp 清洗属于不可回退的最低安全边界。
