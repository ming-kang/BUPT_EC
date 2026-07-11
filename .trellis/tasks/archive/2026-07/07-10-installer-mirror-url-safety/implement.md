# 安装器镜像 URL 安全加固实施计划

## 0. Characterization

- [x] 新增包含 userinfo password 和 query token 的输出泄漏失败测试。
- [x] 新增 insecure opt-in 接受 file/ftp 的失败测试。
- [x] 记录合法 official/HTTPS/HTTP mirror 和 checksum 基线。

## 1. Central URL validation

- [x] 提取一个单一 parse/validate/normalize helper。
- [x] 只允许 HTTPS，或显式 opt-in 的 HTTP。
- [x] 校验非空 authority、host、port、IPv6 bracket 和普通 path。
- [x] 拒绝 userinfo、query、fragment、空白和分号。
- [x] invalid error 只输出规则，不回显 raw URL。

## 2. Safe source logging

- [x] 生成不含 path/query/userinfo 的 safe display label。
- [x] 修改 explicit mirror、download 和 checksum 输出。
- [x] 搜索所有 echo/printf 中的 base_url、override_url 和 DOWNLOAD_BASE_URL。
- [x] 测试 stdout/stderr 不包含测试 password/token。

## 3. Curl protocol enforcement

- [x] 为 HTTPS 和 HTTP break-glass 生成明确 proto/proto-redir 参数。
- [x] package 和 checksums curl 复用相同参数。
- [x] 保持 GitHub redirect 到 HTTPS release asset 正常。
- [x] 保持 connect/download/checksum failure 发生在 snapshot 前。

## 4. Regression matrix

- [x] official latest/nightly/stable URL。
- [x] explicit/saved HTTPS mirror。
- [x] HTTP reject/opt-in accept。
- [x] IPv4/IPv6/port/path/trailing slash。
- [x] empty host/userinfo/query/fragment。
- [x] file/ftp/data 等 scheme。
- [x] checksum missing/mismatch/SKIP_CHECKSUM。
- [x] first install/upgrade success 与 rollback。

## 5. Validation

~~~bash
bash -n scripts/install.sh scripts/install_test.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
rg -n "DOWNLOAD_BASE_URL|base_url|override_url" scripts docs .trellis/spec AGENTS.md
git diff --check
~~~

### Validation results (2026-07-11)

| Check | Result |
| --- | --- |
| `bash -n` | pass |
| `bash scripts/install_test.sh` | pass |
| `shellcheck` | not on PATH (Windows host); CI runs shellcheck |
| `git diff --check` | pass |

## 6. Contract sync and evidence

- [x] 更新 deployment、upgrading、quality spec、AGENTS 的镜像 URL 安全合同。
- [x] 在同一安全修复 commit 更新 CHANGELOG [Unreleased]。
- [x] 记录 secret-negative、protocol、transaction 测试结果和实现 commit 后再归档。

## Review Gates

- 测试必须断言 secret 不在完整输出中。
- insecure opt-in 只能放宽 HTTPS 到 HTTP，不能放宽到任意 scheme。
- package 和 checksum 不得使用不同协议策略。
- 不为通过测试而删除来源信任提示；提示必须安全且仍解释 trust boundary。
- 所有失败发生在 snapshot 前。

## Rollback Points

- URL parser/normalizer。
- safe display label。
- curl protocol flags。
- 若 parser 回滚，credential/query rejection 测试必须保留为阻断证据。
