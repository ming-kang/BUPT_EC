# 安装器镜像 URL 安全加固设计

## Validation Pipeline

DOWNLOAD_BASE_URL 在任何网络调用前经过单一 helper：

~~~text
raw explicit/saved value
  → reject whitespace/semicolon
  → parse and normalize scheme/authority/path
  → reject userinfo/query/fragment
  → enforce HTTPS or explicit HTTP opt-in
  → derive safe display label
  → derive curl protocol allow-list
~~~

不要在 resolve_download_base_url、download_release 和 error branch 中分别实现不同校验。
一个 validation result 应提供 normalized base、safe label 和 allowed protocols。

## URL Contract

支持：

~~~text
https://mirror.example
https://mirror.example/releases/v0.1.4
https://127.0.0.1:8443/releases
https://[::1]:8443/releases
http://mirror.local/releases  only with explicit opt-in
~~~

拒绝：

~~~text
https://user:secret@mirror.example/releases
https://mirror.example/releases?token=secret
https://mirror.example/releases#fragment
https:///missing-host
file:///srv/releases
ftp://mirror.example/releases
~~~

Bash 实现不得依赖 Python、Node 或额外 package。可以使用严格 regex 和小型分段 helper，
但要为 IPv6/port/path 建立表驱动 shell tests。

## Safe Display

日志不再回显 normalized base。建议只输出：

~~~text
Using operator-configured HTTPS mirror host mirror.example.
Downloading repo version arch from configured mirror.
Failed to download checksums.txt from configured mirror.
~~~

invalid input 只报告违反的规则，不附原值。合法 URL 仍保存到 root-owned mode 0600 env，
供升级复用。

## Curl Protocol Policy

根据 validated scheme 生成参数：

~~~text
HTTPS source:
  --proto =https
  --proto-redir =https

HTTP break-glass:
  --proto =http,https
  --proto-redir =http,https
~~~

package 和 checksums curl 共享同一参数数组。host_reachable 的官方 GitHub probe 保持
HTTPS-only。

## Trust Boundary

该任务不改变信任模型：

- 默认自动来源只有 official GitHub releases。
- DOWNLOAD_BASE_URL 是操作员预先信任的镜像。
- 同源 checksums 只证明传输内容一致性，不提供独立 publisher identity。
- SKIP_CHECKSUM=1 仍是明确且高声警告的 break-glass。

## Rollback

- URL validation、safe logging 和 curl protocol flags 作为一个安全边界提交。
- 若某合法 IPv6/port 形式被误拒绝，应扩展 parser/test，不回退 userinfo/query 禁止。
- transaction 和 rollback 函数不应因该任务发生结构变化。
