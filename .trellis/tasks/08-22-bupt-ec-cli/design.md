# 设计：bupt-ec 运维 CLI

任务：`08-22-bupt-ec-cli`

## 职责边界

CLI 是**薄分派层**，不含任何重逻辑：

| 能力 | 归属 |
|---|---|
| 下载、checksum 校验、解压 | `install.sh`（既有） |
| 快照、原子替换、回滚 | `install.sh` 事务内核（既有，不动） |
| 配置渲染（env / systemd / nginx） | `install.sh`（既有） |
| 交互问答 | `install.sh --mode=reconfigure`（由 installer-modes 提供） |
| **子命令分派、参数解析、信息展示、权限检查** | **CLI（本任务）** |

一旦发现要在 CLI 里写第二遍下载或事务逻辑，就是设计跑偏的信号。

## 组件关系

```
用户
 └─ bupt-ec (/usr/local/bin, shell, ~250 行)
     ├─ update ──→ 下载新 install.sh ──→ bash install.sh --mode=update
     ├─ config ──→ 下载新 install.sh ──→ bash install.sh --mode=reconfigure
     ├─ status ─┬→ systemctl is-active / is-enabled
     │          ├→ curl http://${APP_ADDR}/readyz  → version
     │          └→ curl http://${APP_ADDR}/healthz
     ├─ version ─→ env RELEASE_VERSION + /readyz version + CLI 自身版本
     ├─ logs ────→ journalctl -u bupt-ec
     └─ start/stop/restart ─→ systemctl
```

## 分发链路

```
scripts/bupt-ec-cli.sh
   │ release.yml: Compose release assets
   ↓
bupt-ec-linux-${arch}/bupt-ec-cli          ← tarball 内，被 checksums.txt 覆盖
   │ install.sh: stage_release
   ↓
${staging_dir}/bupt-ec-cli                 ← 暂存，未生效
   │ install.sh: commit_installation（原子）
   ↓
/usr/local/bin/bupt-ec                     ← 生效，0755 root:root
```

## 关键设计决策

### D1 tarball 内文件名必须是 `bupt-ec-cli`，不能是 `bupt-ec`

`stage_release` 定位服务二进制的方式是：

```bash
binary_path="$(find "${extract_dir}" -type f -name bupt-ec -print -quit)"
```

`-print -quit` 取**首个**匹配。若 tarball 里同时存在 CLI 脚本 `bupt-ec` 和服务二进制 `bupt-ec`，`find` 的返回顺序不确定，可能把 shell 脚本装成服务二进制——systemd 会启动一个脚本，症状诡异且难排查。

用 `bupt-ec-cli` 作为包内名、安装时改名为 `bupt-ec`，从根上消除这个歧义。

### D2 `stage_release` 需要扩展（本任务的唯一内核触碰点）

`installer-modes` 任务把 `stage_release` 列为不可改；本任务则必须扩展它，新增 CLI 提取：

```
find -name bupt-ec-cli → install -m 0755 到 ${staging_dir}/bupt-ec-cli
```

**这是本任务对事务内核的唯一改动**，且是纯增量（新增一个提取动作，既有二进制提取路径不变）。必须同步补测试：CLI 缺失时的报错、CLI 权限、CLI 属主。

### D3 事务目标从 6 个扩到 7 个

```
binary          /opt/bupt-ec/bupt-ec
cli             /usr/local/bin/bupt-ec      ← 新增
env             /etc/bupt-ec/bupt-ec.env
service         /etc/systemd/system/bupt-ec.service
systemd_enabled /etc/systemd/system/multi-user.target.wants/bupt-ec.service
nginx_site      /etc/nginx/sites-available/bupt-ec.conf
nginx_enabled   /etc/nginx/sites-enabled/bupt-ec.conf
```

`snapshot_installation` / `rollback_installation` / `commit_installation` 都基于 `transaction_targets` 迭代，因此**新增一行即自动获得快照与回滚能力**——这正是既有设计的价值所在，不需要为 CLI 写任何专用回滚代码。

需确认：`restore_snapshot_target` 对「快照时不存在、需要回滚为删除」的情况已有处理（首装 CLI 的场景）。既有 nginx_enabled 目标已覆盖同类语义，复用其路径。

### D4 update 的自举：总是下载新脚本

```
读 env → 定版本 → 下载该版本 install.sh → bash 新脚本 --mode=update
```

**放弃的替代方案**：

| 方案 | 否决理由 |
|---|---|
| 用本地 `/opt/bupt-ec/install.sh` | 自举失败：升级逻辑自身的修复永远用不上 |
| 下载 tarball 校验后取出内部 install.sh | `install.sh` 随后会再下载一次 tarball，重复传输；收益仅是给脚本本身加校验，而现状 `curl \| bash` 同样没有 |

信任模型与今天的 `curl -fsSL ... | sudo bash` 逐字节一致，不新增攻击面。

### D5 URL 规则的重复实现

`install.sh` 的 `resolve_download_base_url` 区分两种形态：

```
latest      → releases/latest/download/
vX.Y.Z      → releases/download/vX.Y.Z/
镜像        → ${DOWNLOAD_BASE_URL}
```

CLI 需要同样的规则来定位 `install.sh`。这约 10 行逻辑会在两处各存一份。

**权衡结论：接受重复**。替代方案是让 CLI source `install.sh` 来复用函数，但那会把整个安装器的全局状态（含 `trap`、`TRANSACTION_*`）拉进 CLI 进程，为省 10 行引入巨大的耦合面。重复的 10 行以注释互指，并由测试覆盖。

### D6 版本语义分离

`RELEASE_VERSION` 是**频道或固定 tag**，`/readyz` 的 `version` 是**实际运行的构建**。二者在 `latest` 频道下必然不同。

```
$ bupt-ec version
配置频道:  latest
运行版本:  v0.3.0
CLI:       v0.3.0
```

CLI 自身版本由 release 构建时注入（与 Go 二进制同一 tag），或读取 `/opt/bupt-ec/bupt-ec` 的输出。取前者：Compose 步骤用 `sed` 把占位符替换为 tag，与 `-ldflags` 注入同源。

### D7 健康探测地址来自 env

`APP_ADDR`（形如 `127.0.0.1:8080`）从 env 读取，不硬编码。若 env 缺失则回退 `127.0.0.1:8080` 并提示。

## 权限模型

```
需要 root: update, config, restart, start, stop
不需要:    status, version, health, logs
```

在分派前统一检查：需 root 的子命令若 `EUID != 0`，打印

```
This command needs root. Try: sudo bupt-ec update
```

而不是让 `systemctl` 或文件写入抛出晦涩错误。

## 兼容性

- **纯新增**：不改变任何既有命令、端点、配置格式的行为
- `curl | bash` 的安装路径继续可用，且现在会额外装上 CLI
- 老版本升级到 v0.3.0 后自动获得 CLI（因为它是事务目标，随二进制一起装）
- 未安装 CLI 的旧机器仍可用 `curl | bash` 升级，升级后即拥有 CLI

## 回滚形状

| 层次 | 机制 |
|---|---|
| 单次升级失败 | 事务内核回滚，CLI 与二进制一并还原（D3 自动获得） |
| CLI 本身有 bug | 用 `curl \| bash` 绕过 CLI 直接跑安装器，或 `bupt-ec update <旧版本>` |
| 整个特性需要撤回 | `git revert`；已安装机器的 `/usr/local/bin/bupt-ec` 会在下次升级时被移除——**需在 `transaction_targets` 移除该行的同时处理遗留文件**，否则会留下一个指向旧逻辑的孤儿命令 |

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| `find -name bupt-ec` 抓到 CLI 脚本 | D1 的包内改名；测试断言解压后服务二进制是 ELF 而非脚本 |
| 路径常量在 CLI 与 install.sh 间漂移 | CLI 只硬编码路径，可变配置一律读 env；两文件互相注明「修改时需同步」 |
| CLI 装到 `/usr/local/bin` 与包管理器冲突 | `/usr/local` 按 FHS 就是本地管理员领地，不与 apt 冲突 |
| 用户在 CLI 失效时无法升级 | 文档保留 `curl \| bash` 作为兜底路径，不废弃 |
