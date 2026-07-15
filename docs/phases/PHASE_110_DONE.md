# Phase 110 完成 — 免打扰/置顶服务端同步

## 需求分析

- 现状：mute / pin 仅 localStorage，多端不一致。
- 目标：`user_chat_prefs` 云端存储；登录拉取；变更即 PUT。

## 设计计划

- 迁移 `007_user_chat_prefs.sql`
- `GET/PUT /users/me/chat-prefs`
- FE `loadChatPrefs` / `persistChatPrefs`；首次空云端上传本地

## 交付

- deploy/init/mysql/007_user_chat_prefs.sql
- store/service/handler auth chat-prefs
- frontend chat store + ChatView 启动拉取

## 测试

- `go build ./...` + migration 应用 + `npm run build` ✅

## 下阶段

Phase 111：跨会话真实转发
