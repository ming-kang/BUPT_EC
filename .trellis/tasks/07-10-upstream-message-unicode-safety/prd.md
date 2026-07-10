# 上游消息 Unicode 清洗加固

## Goal

让所有进入内部 error 和结构化日志的 JW 上游消息在完整 Unicode 范围内保持单行、
有界并可靠脱敏，防止 C1 控制符、Unicode 行分隔符、格式控制符或特殊空白绕过现有
ASCII 清洗和敏感 key/value 匹配。

## Background

- service/jw_error.go:102 的 sanitizeRemoteMessage 按 rune 遍历消息。
- 当前只把小于 0x20、0x7f、普通空格和 NBSP 折叠为空格。
- U+0085 NEXT LINE、U+2028 LINE SEPARATOR、U+2029 PARAGRAPH SEPARATOR、其他
  Unicode 空白及 bidi/format controls 仍会保留。
- remoteASCIISecretKV 和 remoteCJKSecretKV 使用 RE2 的 ASCII \s 语义；Unicode
  空白位于 key 与分隔符之间时可能阻止脱敏匹配。
- 现有测试只覆盖 CR/LF/tab、ASCII key/value、Bearer、中文账号和 rune 截断。
- API 客户端已经只接收 SafeErrorMessage 固定文案，本任务只强化内部边界。

## Dependencies

- 没有其他新 child 的代码依赖；按父任务顺序在 installer URL 后实施。
- 最终 hygiene 子任务依赖本任务锁定的 Unicode output invariants。

## Requirements

### R1 — Unicode normalization

- 所有 Unicode whitespace、line/paragraph separator 和 control rune 折叠为单个 ASCII
  space。
- C0、DEL、C1、U+0085、U+2028、U+2029 和常见 Unicode spaces 必须覆盖。
- 会改变文本显示方向或隐藏字符的 format controls 应移除或折叠，不能原样进入日志。
- 连续 unsafe rune 只产生一个 space，首尾空白被移除。
- 普通中文、英文、数字和标点保持可读，不做不必要的全角/半角归一化。

### R2 — 脱敏顺序和覆盖

- 先完成 Unicode normalization，再执行 sensitive fragment redaction，使 Unicode
  whitespace 不能绕过 key/value regex。
- 保持 token、authorization、password、passwd、username、account、学号、账号、
  密码、令牌和 Bearer 覆盖。
- 支持现有冒号、等号和全角冒号分隔符。
- replacement 只保留安全 key label 与固定 REDACTED，不回显 value。
- 不对没有敏感 key 上下文的普通长中文/英文片段做激进 credential 猜测。

### R3 — 长度与 fallback

- redaction 后按 Unicode rune 限制为 256，不能切断 UTF-8 code point。
- empty、全 whitespace/control 或清洗后为空使用固定 fallback。
- 输出不得包含 CR/LF、Unicode line separator、control/format rune 或原始 secret。
- helper 保持纯函数，不读取 runtime credentials，也不接收真实 secret 作为参数。

### R4 — 调用边界

- JW login/query HTTP/business-code message 必须统一经过 safeRemoteMessage。
- SafeErrorMessage 继续返回固定中文客户端文案。
- error kind、auth detection、retry、cache warning 和 RuntimeStatus schema 不改变。
- slog 字段名、log_id 关联和 JSON logging 输出不改变。

### R5 — 测试

- 表驱动测试覆盖 ASCII、C1、Unicode spaces、line/paragraph separators、bidi/format
  controls 和连续混合序列。
- 覆盖 Unicode whitespace 包围敏感 key、separator、Bearer value 的脱敏。
- 覆盖 256 rune 边界、replacement 后截断和全 unsafe fallback。
- 增加 property/fuzz seed：任意输入输出保持 rune 上限，且不含禁止类别。
- 至少一个 slog JSON 回归证明一条 sanitized message 只产生一条日志记录。

## Acceptance Criteria

- [ ] U+0085、U+2028、U+2029、Unicode spaces 和 bidi/format controls 不原样保留。
- [ ] 连续控制/空白折叠为一个 ASCII space，普通文本保持可读。
- [ ] Unicode whitespace 无法绕过 token/password/account/Bearer 脱敏。
- [ ] 任意输出最多 256 runes，empty/all-unsafe 使用固定 fallback。
- [ ] API error body 仍不包含任何原始上游文本。
- [ ] 所有 JW 上游 message 调用点均经过同一个 safeRemoteMessage pipeline。
- [ ] table tests、fuzz seeds、slog JSON 回归、go test -race ./...、vet/build 通过。

## Out of Scope

- 对所有项目日志字段增加通用 DLP 或自然语言 secret 检测。
- 修改 JW protocol、auth classification、客户端文案或日志后端。
- Unicode NFC/NFKC 规范化普通业务文本。
