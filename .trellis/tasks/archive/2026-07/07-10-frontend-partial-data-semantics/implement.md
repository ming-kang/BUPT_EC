# 前端部分数据选择语义实施计划

## 1. Characterization

- [ ] 构造冷 partial、带旧缓存 partial、完整成功和全空 payload fixtures。
- [ ] 固定当前 App 自动选沙河的行为测试，确认缺陷可复现。

## 2. Pure selection helper

- [ ] 新增 campus snapshot/eligibility helper 和稳定选择函数。
- [ ] 覆盖当前选择保持、partial placeholder、旧数据、列表变更和空列表。
- [ ] 不依赖中文名称以外的隐含顺序；沙河偏好仅作为明确产品规则。

## 3. App integration and copy

- [ ] App effect 调用 helper，仅在目标 ID 变化时 dispatch。
- [ ] 验证 campus 变化仍清空 dependent buildings/class times。
- [ ] 将 settings 中 `updated_at` 文案改为准确的数据更新时间。

## 4. Docs and checks

- [ ] 更新 API spec、operations/user-facing docs 和 changelog。
- [ ] 运行 focused tests 后再跑完整 frontend gate。

## Validation

```bash
pnpm --dir frontend lint
pnpm --dir frontend test
pnpm --dir frontend build
pnpm --dir frontend audit:prod
pnpm --dir frontend audit:dev
git diff --check
```

## Review Gates

- 不把 `partial` 等同于“完全不可用”；必须考虑后端保留的旧 snapshot。
- 用户可用的当前选择不应抖动。
- 组件文件不得额外导出 helper。
- 不修改 API schema。

## Rollback Point

选择 helper、tests、App effect 与文案作为一个原子前端改动回滚。
