# Phase 121 完成 — 云端草稿同步

## 需求分析

- 现状：草稿仅存 `localStorage`，换设备/清缓存丢失。
- 对标：微信/Slack 跨端恢复未发送正文。
- 验收：登录拉取云端草稿合并本地；输入防抖上传；空内容删除云端记录。

## 设计计划

- 表 `user_drafts (user_id, conversation_id, content)`
- `GET/PUT /users/me/drafts`
- ChatView：本地即时 + 800ms 防抖同步；启动时 merge

## 交付

- `010_user_drafts.sql`
- store/service/handler drafts APIs
- `chat.loadDrafts` / `persistDraft` + ChatView sync

## 测试

- migration ✅ · `go build` ✅ · `npm run build` ✅
- API PUT/GET smoke ✅

## 下阶段

Phase 122：会话书签 / 常用链接（header bookmarks）或消息编辑
