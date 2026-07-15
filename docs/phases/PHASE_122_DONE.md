# Phase 122 完成 — 会话书签

## 需求分析

- 对标 Slack Bookmarks：会话级常用链接（文档/看板），与消息置顶分离。
- 验收：成员可增删书签；头部「书签」面板打开外链；每会话最多 20。

## 设计计划

- 表 `conversation_bookmarks`
- `GET/POST /conversations/:id/bookmarks`，`DELETE .../bookmarks/:id`
- 仅 http/https；ChatView 头部面板

## 交付

- `011_conversation_bookmarks.sql`
- store/service/handler
- chat store + ChatView 书签面板

## 测试

- migration ✅ · builds ✅

## 下阶段

Phase 123：消息编辑（发出后短时编辑）或已读回执 DM 双勾增强
