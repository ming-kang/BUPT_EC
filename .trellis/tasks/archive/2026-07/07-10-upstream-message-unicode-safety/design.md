# 上游消息 Unicode 清洗加固设计

## Sanitization Pipeline

保持一个集中且确定性的顺序：

~~~text
raw upstream string
  → classify each rune as safe or unsafe
  → collapse unsafe runs to ASCII space
  → trim
  → redact sensitive key/value and bearer fragments
  → cap to 256 runes
  → trim
  → fixed fallback when empty
~~~

Unicode normalization 必须先于 regex redaction，使 regex 只需要处理规范化后的 ASCII
space，而不是在每个 pattern 中重复 Unicode whitespace 定义。

## Unsafe Rune Classification

建议提取纯 helper isUnsafeRemoteRune，集中判断：

- unicode.IsSpace；
- Unicode control categories，包括 C0/C1；
- line separator Zl 和 paragraph separator Zp；
- format category Cf，用于 bidi override、zero-width 和不可见格式控制。

所有 unsafe rune 统一折叠为空格。若后续证据表明某个普通语言所需 Cf 必须保留，应建立
显式 allow-list 和测试，而不是放宽整个类别。

## Redaction Boundary

normalize 后现有 regex 可继续使用 ASCII space：

~~~text
token = value
authorization: value
密码：value
Bearer value
~~~

redaction 在 truncation 前执行，避免 secret 因截断位置不同而只被部分保留。replacement
固定为 KEY=[REDACTED] 或 Bearer [REDACTED]。

## Output Invariants

safeRemoteMessage 返回值必须满足：

1. 非空。
2. RuneCount 不超过 256。
3. 不包含 unsafe rune 类别。
4. 不包含已识别 sensitive value。
5. 对相同输入输出确定。

sanitizeRemoteMessage 内部 helper 可以在清洗后返回空；只有 safeRemoteMessage 负责
fixed fallback，保持现有职责。

## Test Design

### Table cases

- CR/LF/tab/DEL；
- U+0085、U+2028、U+2029；
- EM SPACE、THIN SPACE、IDEOGRAPHIC SPACE；
- zero-width/bidi override/word joiner；
- 普通中文和 emoji；
- sensitive key 与 separator 之间使用 Unicode spaces；
- long Chinese、replacement near rune limit、all-unsafe。

### Property seeds

FuzzSafeRemoteMessage 提供上述 seeds。普通 go test 执行 seed corpus，并断言 output
invariants；持续 fuzz 可作为本地扩展，不加入不稳定 CI 时长。

### Logging integration

构造内存 slog JSON handler，记录包含 sanitized jwError 的一条消息，断言输出只有一个
JSON line，解析成功且不含原始 secret/control rune。

## Compatibility and Rollback

- 客户端不接触 upstream message，因此没有 public payload migration。
- 更严格的内部日志文本是有意安全变化，本任务在同一 commit 同步 owning
  changelog/spec；最终 hygiene 任务只做跨任务一致性审计。
- 若 Cf 全类别过严，只能通过精确 allow-list 回滚，不回退 C1/Zl/Zp 清洗。
