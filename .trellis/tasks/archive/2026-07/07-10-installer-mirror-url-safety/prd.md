# 安装器镜像 URL 安全加固

## Goal

收紧 DOWNLOAD_BASE_URL 的 scheme、authority、redirect 和日志合同，确保操作员显式镜像
仍可使用，但 URL 中的凭据、敏感查询参数或危险协议不会进入终端、日志或 curl 跳转链。

## Background

- scripts/install.sh:239 的 validate_download_base_url 只拒绝空白和分号。
- 默认允许任何以 https:// 开头的字符串，包括 userinfo、query、fragment 和空 host。
- ALLOW_INSECURE_DOWNLOAD_BASE_URL=true 时当前接受任意非 HTTPS scheme，不限于 HTTP。
- resolve_download_base_url:308 和 download_release:341/352 输出完整 base URL。
- 若 URL 为 https://user:password@host/path 或带 token query，secret 会直接显示。
- curl 使用 -L，但未显式限制初始及重定向协议集合。
- 原 provenance 任务明确要求镜像提示不得打印凭据或敏感 URL 参数。

## Dependencies

- 没有其他新 child 的代码依赖；按父任务顺序在 frontend jitter 后实施。
- 最终 hygiene 子任务依赖本任务锁定的 URL/日志/redirect 测试合同。

## Requirements

### R1 — 允许的 URL 形状

- 默认只接受绝对 HTTPS URL。
- ALLOW_INSECURE_DOWNLOAD_BASE_URL=true 只额外允许绝对 HTTP URL。
- file、ftp、scp、data、gopher 及其他 scheme 无论是否 opt-in 都必须拒绝。
- authority/host 必须非空；允许文档化的域名、IPv4、括号 IPv6 和可选有效端口。
- userinfo、query 和 fragment 必须拒绝，不作为镜像认证机制。
- path 可以为空或为普通层级路径；尾部 slash 统一去除。
- invalid URL 错误不得回显完整原始值。

### R2 — 安全日志

- explicit/saved mirror 只输出安全的 source label，例如 scheme + host，或固定
  operator-configured mirror 文案。
- download 和 checksum 错误不得输出完整 base URL。
- 测试 secret 必须在 stdout/stderr 全文中不可检索。
- 官方 GitHub source 可以继续输出 repo/version/arch，但不打印任何凭据。

### R3 — curl 协议限制

- HTTPS source 的初始请求和重定向只允许 HTTPS。
- 显式 HTTP break-glass source 可以允许 HTTP/HTTPS，但不得跳转到其他协议。
- package 与 checksums.txt 必须使用同一经过验证的 base 和相同协议策略。
- checksum fail-closed、SKIP_CHECKSUM break-glass 和 same-origin trust 语义保持。

### R4 — 保存配置与事务兼容

- 合法的已保存 HTTPS mirror 在升级时继续工作。
- 已保存但不再合法的 userinfo/query/non-HTTP URL 必须在任何下载和 snapshot 前失败。
- RELEASE_VERSION、DOWNLOAD_BASE_URL 优先级、release asset 名称和单文件 installer 不变。
- 所有新校验发生在 download/checksum/extract/render/snapshot 之前。

### R5 — 测试

- 覆盖 HTTPS、HTTP without/with opt-in、IPv4、IPv6、port、path 和 trailing slash。
- 覆盖 empty host、userinfo、query、fragment、whitespace、semicolon 和非 HTTP scheme。
- 覆盖 redirect protocol arguments 和 package/checksum 一致性。
- 覆盖显式及保存镜像错误输出中不出现 username/password/token 测试值。

## Acceptance Criteria

- [x] 合法 HTTPS mirror 和显式 HTTP break-glass mirror 可完成下载及 checksum。
- [x] 非 HTTP(S) scheme 即使设置 insecure opt-in 也在下载前失败。
- [x] userinfo、query、fragment 和空 host 在下载前失败。
- [x] stdout/stderr 不包含测试 URL 的 password、token 或完整敏感 path/query。
- [x] curl 初始和 redirect protocol 均被显式限制。
- [x] 官方 GitHub-only 默认、无自动第三方 fallback 和 same-origin checksum 合同不变。
- [x] upgrade/first-install transaction、rollback、asset layout 和 env mode 测试继续通过。
- [x] bash -n、installer tests、ShellCheck 和 git diff --check 通过。

## Out of Scope

- 为私有镜像新增 Basic Auth、OAuth、mTLS 或独立签名基础设施。
- 自动发现镜像、恢复第三方代理 fallback 或更换 release asset。
- 重写安装器为其他语言。
