# Phase 131 完成 — 会话文件夹（侧栏自定义分组）

## 需求分析

- 对标 Slack / 企微分组：侧栏按自定义文件夹筛选好友/群聊。
- 验收：新建/删除文件夹；右键移入/移出；芯片筛选；云端同步 chat-prefs。

## 设计计划

- `user_chat_prefs.folders_json`：`[{id,name,conversation_ids}]`
- 复用 `GET/PUT /users/me/chat-prefs`，不另开 API

## 交付

- Migration `018_chat_folders.sql`
- Store `ChatFolder` + prefs 读写
- ChatView 文件夹条/管理面板/右键菜单；chat store CRUD

## 测试

- `go build ./...` ✅
- `npm run build` ✅

## 下阶段

Phase 132：消息话题标签 / hashtag 筛选（或已读回执增强）
