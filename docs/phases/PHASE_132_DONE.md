# Phase 132 完成 — 消息话题标签（Hashtag）

## 需求分析

- 对标 Slack / Telegram 话题：文本中 `#标签` 可点击筛选本会话相关消息。
- 验收：发送/编辑索引标签；搜索面板展示热门标签；点击气泡 `#tag` 筛选。

## 设计计划

- 表 `message_hashtags`（msg_id, tag）
- 发送/编辑写入；撤回删除
- `GET /conversations/:id/hashtags` · `GET .../messages/by-hashtag?tag=`

## 交付

- Migration `019_message_hashtags.sql`
- Store / Service extract + sync；Handler 两接口
- ChatView 可点击 hashtag + 搜索面板 chips

## 测试

- `go build ./...` ✅
- `npm run build` ✅

## 下阶段

Phase 133：会话内文件媒体库（按类型浏览图片/文件）
