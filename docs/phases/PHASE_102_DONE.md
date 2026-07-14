# Phase 102 完成 — Toast 提示自动消失

## 需求分析

- 全局 Toast（如「群聊已创建，已向 1 位好友发送入群邀请」）会一直留在屏幕上
- 提示类弹窗应展示约数秒后自动消失

## 原因

- `setNotice` 只赋值、不设定时器
- `setTransientNotice` 才会 4s 后清除
- 建群等成功提示误用了 `setNotice`

## 修复

- `setNotice` 改为调用 `setTransientNotice`
- 统一约 3 秒后自动清除
- `clearNotice` / `setError` 时同步清掉定时器

## 验证

刷新前端后创建群聊，Toast 约 3 秒后消失。
