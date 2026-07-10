# 安装器版本选择语义

## Goal

使 stable、nightly 和固定 tag 的文档命令与安装器实际下载目标完全一致，并在升级重跑时保持用户之前选择的 release 通道或标签。

## Requirements

- R1：保持首次运行且未设置版本时默认 `nightly` 的现有兼容行为。
- R2：GitHub latest stable 的所有文档命令必须显式传 `VERSION=latest`。
- R3：nightly 文档命令显式传 `VERSION=nightly`，避免命令语义依赖默认值。
- R4：固定 tag 文档命令必须同时从该 tag 下载 installer 并传同一个 `VERSION=vX.Y.Z`。
- R5：安装器将最终选择写入 `RELEASE_VERSION`；后续运行的优先级为显式 `VERSION` > 已保存 `RELEASE_VERSION` > `nightly`。
- R6：开始下载前清楚打印 repo、version 和最终 base URL，不泄露凭据。
- R7：`latest`、`nightly`、tag 和自定义 `DOWNLOAD_BASE_URL` 的解析必须具有脚本测试。
- R8：release 资产名称和布局保持不变。

## Acceptance Criteria

- [x] README stable 命令实际解析到 `/releases/latest/download`。
- [x] README nightly 命令实际解析到 `/releases/download/nightly`。
- [x] 固定 tag 命令解析到完全相同的 tag。
- [x] 重跑安装器未传 VERSION 时复用保存的 RELEASE_VERSION。
- [x] 显式 VERSION 能覆盖已保存值。
- [x] 自定义 download base URL 继续优先，且 HTTPS/显式 insecure 校验保持不变。
- [x] CI 执行安装器版本解析测试、`bash -n` 和 shellcheck。
- [x] README、deployment、upgrading、release、AGENTS 和 CHANGELOG 保持一致。

## Out of Scope

- 不改变 GitHub release workflow 的 nightly/stable 发布触发方式。
- 不自动调用 GitHub API 解析具体 stable tag。
- 不重命名 release assets。
