# 安装器版本选择语义 — Implementation Plan

## Implementation

- [x] 搜索 README/docs/scripts 中全部 stable、latest、nightly 和 `VERSION=` 示例。
- [x] 增加 `CURRENT_RELEASE_VERSION` 读取和版本优先级 helper。
- [x] 增加严格 version validator，并保持 custom download base URL 逻辑。
- [x] 将 `RELEASE_VERSION` 写入环境文件。
- [x] 给 install.sh 增加 source-safe main guard。
- [x] 新增 `scripts/install_test.sh`，覆盖 version 和 URL mapping。
- [x] 在 CI/release quality gate 中运行脚本行为测试。
- [x] 更新所有安装/升级文档、AGENTS 和 changelog。

## Focused Tests

- [x] no explicit/current => nightly。
- [x] explicit latest/nightly/tag => exact mapping。
- [x] current latest with no explicit => latest。
- [x] explicit tag overrides current channel。
- [x] invalid version rejected。
- [x] custom base URL preserved。

## Validation

```powershell
bash -n scripts/install.sh scripts/install_test.sh
bash scripts/install_test.sh
shellcheck scripts/*.sh
```

再检查 `.github/workflows/ci.yml` 与 `release.yml` 的 shell test step 一致。

## Rollback Point

`RELEASE_VERSION` 是新增且应用忽略的环境变量。回滚代码后它可安全保留；文档命令仍显式传 VERSION，不依赖环境文件。
