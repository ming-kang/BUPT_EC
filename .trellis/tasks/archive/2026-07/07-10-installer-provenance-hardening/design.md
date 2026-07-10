# 安装器下载来源与制品信任设计

## Boundary

本任务只改变 release 下载来源选择、提示、测试和文档：

```text
VERSION / RELEASE_REPO
        │
        ├─ no override ──> official GitHub release URL only
        │
        └─ DOWNLOAD_BASE_URL ──> explicitly trusted operator mirror
                                      │
                                      └─ package + checksums.txt
```

候选文件准备、snapshot、atomic commit、rollback 和发布资产布局不变。

## Trust Model

### Official path

- 默认仓库仍为 `ming-kang/BUPT_EC`，版本由现有版本选择合同解析。
- GitHub release HTTPS URL 是默认且唯一自动选择的信任边界。
- 可选的 reachability probe 只能用于产生更清楚的错误，不能改变来源。

### Explicit mirror path

- `DOWNLOAD_BASE_URL` 表示操作员已经选择并信任该镜像。
- `validate_download_base_url` 继续拒绝空白、分号和非 HTTPS URL；仅显式
  `ALLOW_INSECURE_DOWNLOAD_BASE_URL=true` 放宽到 HTTP。
- 默认从同一显式基址下载包和 `checksums.txt`。文档必须准确说明：这防止损坏或
  不一致下载，但镜像若同时被攻破，checksum 不是独立真实性证明。
- 本任务不添加新签名文件，因而不改变现有 release asset contract。

## Error Contract

| Condition | Result |
| --- | --- |
| GitHub 不可达且无覆盖 | 非零退出；提示显式镜像方式；不接触安装目标 |
| 自动代理常量/探测 | 不存在或不可到达 |
| 显式 HTTPS 镜像 | 验证 URL 后正常下载 |
| 显式 HTTP 镜像且无 opt-in | 非零退出 |
| 包或 checksum 失败 | 非零退出；不进入 snapshot |
| 哈希不匹配 | 非零退出；不进入 extract/commit |

## Compatibility

- 直接 GitHub 用户无行为变化。
- 以前依赖自动 `gh-v6.com` fallback 的环境需要显式传入可信镜像 URL；这是有意的
  安全收紧，必须在升级文档和 changelog 中突出说明。
- 已保存的镜像配置继续生效，因此已明确选择镜像的升级不会被破坏。

## Rollback

该变更发生在安装事务之前。若来源政策导致不可用，可回滚来源选择函数和文档，
不涉及已安装文件恢复。不得通过恢复隐式第三方 fallback 来处理普通可用性问题。
