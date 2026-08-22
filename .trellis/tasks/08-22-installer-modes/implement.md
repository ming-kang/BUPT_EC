# 执行计划：install.sh 模式拆分

任务：`08-22-installer-modes`
前置：`08-22-drop-nightly` 必须先归档（否则版本解析要改两遍）

## 执行顺序

### 步骤 1：建立基线快照

在动任何代码前记录当前行为，作为「纯搬移」的比对基准。

- [ ] 运行 `bash scripts/install_test.sh`，记录通过的断言总数
- [ ] 把当前 `main()` 中从 `repo="${REPO:-...}"` 到 `download_base_url="${DOWNLOAD_BASE_URL:-...}"` 的整段复制到临时笔记，逐项列出每个 prompt 的：提示文案、默认值表达式、必填性、条件分支
- [ ] `shellcheck scripts/install.sh` 记录基线（应为 0 问题）

**验证**：`bash scripts/install_test.sh` 全绿

### 步骤 2：引入 `parse_mode` 与模式常量

- [ ] 新增全局 `INSTALLER_MODE="install"`
- [ ] 新增 `parse_mode()`：识别 `--mode=<v>` 与 `--mode <v>`，校验取值属于 `install|update|reconfigure`，其余参数拒绝并打印用法
- [ ] `main()` 开头（`load_current_config` 之后）调用 `parse_mode "$@"`
- [ ] 此步**不改变任何行为**：三种模式此时走同一条老路径

**验证**：
```bash
bash scripts/install_test.sh        # 全绿，断言数不变
shellcheck scripts/install.sh       # 0 问题
```

### 步骤 3：抽取 `collect_config_interactive`（纯搬移）

- [ ] 声明十个 `CFG_*` 全局变量
- [ ] 把问答段整体搬入 `collect_config_interactive()`，把原先的 local 变量赋值改为 `CFG_*` 赋值
- [ ] **不搬**其中的两处校验（JW 凭据组合校验留待步骤 5）
- [ ] `main()` 中原位置改为调用该函数
- [ ] 下游 `validate_*` / `prepare_staging` 的入参改为读 `CFG_*`

**评审关卡**：逐行 diff 对照步骤 1 的笔记，确认每个 prompt 的文案、默认值优先级、必填性一字未变。

**验证**：`bash scripts/install_test.sh` 全绿且**断言零修改**

### 步骤 4：新增 `adopt_current_config`

- [ ] 实现：十个 `CFG_*` 从对应 `CURRENT_*` 取值；`CFG_VERSION` 按 D1 走 `VERSION` 优先、否则 `CURRENT_RELEASE_VERSION`
- [ ] 缺失必需项（domain / ssl_cert / ssl_key / app_addr，以及 JW 凭据组合）时快速失败，错误消息指引 `--mode=reconfigure`
- [ ] `main()` 的 case 分支接入

**验证**：新增测试断言 update 模式在 env 完整/残缺两种情况下的行为

### 步骤 5：收拢校验到 `validate_all`

- [ ] 把 JW 凭据组合校验从问答段移入 `validate_all()`
- [ ] 把 `ssl_cert` / `ssl_key` 文件存在检查从执行段移入 `validate_all()`
- [ ] 既有 `validate_repo` / `validate_version` / `validate_domain` / `validate_absolute_path` / `validate_app_addr` / `validate_download_base_url` 一并纳入，全部读 `CFG_*`

**关键**：此步是 update 模式安全性的核心——它不经过问答段，校验必须在共用路径上。

**验证**：`bash scripts/install_test.sh` 全绿

### 步骤 6：抽取 `execute_deployment` 并接入模式差异

- [ ] 把 `detect_arch` 到 `perform_install_transaction` 整段搬入 `execute_deployment()`
- [ ] `install_packages` 调用加模式判断：update 跳过
- [ ] update 模式开始前检查 `curl` 与 `tar` 可用，缺失则报错指引 `--mode=install`
- [ ] `print_completion_summary()` 按模式给不同措辞（首装给 URL 与后续指引；升级给版本前后对比）

**验证**：
```bash
bash scripts/install_test.sh
shellcheck scripts/*.sh
```

### 步骤 7：补充新模式测试

- [ ] 三种模式的分派正确性
- [ ] `--mode=update` 零 prompt（断言未调用任何 `prompt*`）
- [ ] `--mode=update` 在 `< /dev/null` 无 tty 下成功
- [ ] `--mode=update` 不执行 `apt-get update`（mock 断言）
- [ ] `--mode=update` env 残缺时失败且消息含 `reconfigure` 指引
- [ ] `--mode=reconfigure` 不改变版本
- [ ] 非法 `--mode` 值失败并列出可用值

### 步骤 8：文档同步

- [ ] `docs/upgrading.md` 增加非交互升级说明与 `bash -s -- --mode=update` 传参形式
- [ ] `docs/deployment.md` 说明三种模式
- [ ] `docs/release.md` 若提及安装器行为则同步
- [ ] `CHANGELOG.md` `[Unreleased]` 的 `Added` 记录新模式

## 全量验证命令

```bash
bash scripts/install_test.sh          # 既有断言零修改 + 新增断言全绿
shellcheck scripts/*.sh               # 0 问题
task check                            # Go 侧不受影响，回归确认
```

## 回滚点

| 步骤 | 回滚方式 |
|---|---|
| 步骤 2 后 | 单个 commit revert，无行为变更故零风险 |
| 步骤 3 后 | 单个 commit revert；这是最大的一步，建议独立成 commit 便于回退 |
| 步骤 5 后 | 单个 commit revert；校验位置移动若引发回归，此处最可能 |
| 全部完成后 | 整体 revert；无 env 格式变更、无系统状态残留，回退无副作用 |

## 完成信号

**「既有 `install_test.sh` 断言零修改即全绿」是本任务最重要的验收信号。**

若在任何步骤不得不修改既有断言，立即停止并回到 `design.md` 重新审视——这说明抽取不是纯搬移，行为已经漂移。
