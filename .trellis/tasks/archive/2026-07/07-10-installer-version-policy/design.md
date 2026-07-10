# 安装器版本选择语义 — Design

## Version Resolution

安装器启动时读取现有环境文件中的：

```bash
CURRENT_RELEASE_VERSION="${RELEASE_VERSION:-}"
```

最终选择：

```bash
version="${VERSION:-${CURRENT_RELEASE_VERSION:-nightly}}"
```

允许值：

- `latest`
- `nightly`
- `v` 开头的受限 tag（沿用/新增明确 validator）

禁止版本值包含路径分隔符、空白或 shell metacharacters。

## URL Mapping

```text
latest  -> https://HOST/REPO/releases/latest/download
other   -> https://HOST/REPO/releases/download/VERSION
override DOWNLOAD_BASE_URL -> exact validated override
```

下载前输出 resolved version 和 base URL，帮助用户发现选择错误。

## Persistence

`write_env` 增加 root-only 的 `RELEASE_VERSION`。该值供 installer 重跑使用，应用进程忽略它。固定 tag 因此保持 pin；`latest` 和 `nightly` 保持各自滚动通道。

## Script Testability

给 `install.sh` 增加 main guard：

```bash
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
```

测试脚本可 source installer 并直接验证纯解析函数，不执行 root/systemd 操作。CI 增加 `bash scripts/install_test.sh`。

## Documentation Commands

```bash
# stable latest
curl .../releases/latest/download/install.sh | sudo VERSION=latest bash

# nightly
curl .../releases/download/nightly/install.sh | sudo VERSION=nightly bash

# pinned
curl .../releases/download/v0.1.4/install.sh | sudo VERSION=v0.1.4 bash
```
