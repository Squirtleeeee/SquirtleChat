# Phase 144 完成 — 免打扰时段云端同步

## 需求分析

- 现状：设置页已有本地 `quietHours`，仅 localStorage，多端不一致。
- 验收：通知开关与免打扰时段写入 `chat-prefs`；登录拉取覆盖本地；变更即 PUT。

## 设计计划

- `user_chat_prefs.notify_json`（024）
- `ChatPrefs.Notify` 读写
- FE：`persistChatPrefs` / `loadChatPrefs` 带 `notify`；设置页变更后同步

## 交付

- Migration 024
- Backend store ChatPrefs.notify
- settings.applyCloudNotify / cloudNotifyPayload
- SettingsView 变更触发云端同步

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- API：PUT/GET chat-prefs 含 notify ✅

## 下阶段

Phase 145：群成员备注（仅自己可见）或消息全文翻译按钮
