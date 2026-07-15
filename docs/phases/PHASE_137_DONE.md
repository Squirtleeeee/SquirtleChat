# Phase 137 完成 — 云端/本地草稿冲突处理

## 需求分析

- 云端同步曾无条件覆盖本地；本地与云端不一致时应提示用户选择。
- 验收：冲突弹窗展示两边摘要；「保留本地」/「使用云端」；可连续处理多处。

## 交付

- `GET /users/me/drafts` 增加 `items`（含 `updated_at`）
- ChatView `syncDraftsFromCloud` 冲突检测 + 选择弹窗

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- backend 重启 ✅

## 下阶段

Phase 138：会话内「查找下一条」搜索结果导航（↑↓）
