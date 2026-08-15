# PRD: nightly 发布覆盖与 release.sh 可移植化（release-hygiene）

来源：`08-07-project-audit-optimize` backlog 优先级 2（E-04 + E-05，轻量）。

## 问题陈述

1. **E-04 nightly 发布删建空窗**：`.github/workflows/release.yml:150-169` 在 main push 时
   `gh release delete nightly --cleanup-tag` 后由 `action-gh-release` 重新创建。
   删→建之间 `releases/download/nightly/<asset>` 短暂 404；而 install.sh 首装默认
   解析到 `nightly`（`scripts/install.sh:188`），消费者会撞上竞态失败。
2. **E-05 release.sh 的 sed 不可移植**：`scripts/release.sh:65-69` 两处 `sed -i`：
   - BSD/macOS sed 要求 `-i` 带后缀参数，无参形式把脚本当后缀，静默损坏文件；
   - 替换串里的 `\n` 是 GNU 扩展，BSD 下写出字面 `n`。
   macOS 维护者跑一次就把 CHANGELOG.md / frontend/package.json 写坏。

## 目标

- nightly 发布改为**同 tag 资产覆盖**（`gh release upload --clobber`），消除 404 空窗；
  首次（release 不存在）仍走 create。
- `scripts/release.sh` 去掉全部 `sed -i`，改用仓库既有 awk 流式改写模式（与 CHANGELOG
  标题滚动同风格），并加强事后校验（版本号确实写入、Unreleased 链接确实改写）。

## 非目标

- 不改 stable tag 发布路径（tag push 分支保持 `action-gh-release` 原样）。
- 不改 install.sh 的版本解析/下载逻辑。
- 不动 `.goreleaser`（已关闭）、不引入 perl/python 依赖（awk/bash 已够用）。
- 不处理 E-06 golangci-lint、E-07 size 跳过等其余工程项。

## 验收标准

1. nightly 路径无 delete 步骤：release 存在时 `gh release upload --clobber` 覆盖同名
   资产；不存在时 `gh release create`（保持 `prerelease` + generate notes + 四个产物）。
   全程无窗口使资产 404。
2. nightly tag 尽量指向新 commit（force-move tag 或等价手段），保持 release 页面
   source 链接新鲜；若实现代价过高可在 PR 中说明并接受固定 tag（以 clobber 无空窗为硬标准）。
3. `scripts/release.sh` 无任何 `sed -i`；CHANGELOG compare 链接改写与 package.json
   版本号 bump 用 awk 实现，语义与现行 GNU sed 输出完全一致（多行 `[X.Y.Z]:` 链接插入）。
4. 事后校验：package.json 中确实出现 `"version": "<bare>"`；CHANGELOG 中确实出现
   `## [<bare>] - <date>` 与新 `[<bare>]:` 链接行；任一失败立即报错退出（防静默损坏）。
5. 脚本仍通过 `bash -n`；现有发布文档 `docs/release.md` 与 workflow 行为描述同步
   （如提及删建语义则更新）。
6. CHANGELOG.md Unreleased 记录 nightly 发布不再有空窗（运维可见）。
7. 不新增第三方 action / 依赖；workflow 权限不放宽（release job 已有 contents: write）。

## 约束

- workflow 改动限于 nightly 分支条件块；stable 路径与 build-go 矩阵不动。
- `softprops/action-gh-release` 版本 pin 不变（stable 路径仍用）；nightly 路径改用
  gh CLI 步骤（action 对已存在 release 的资产覆盖语义不透明，gh `--clobber` 明确）。
- 锁定与仓库 AGENTS.md 一致的脚本风格：bash 严格模式、错误即退、可读优先。
