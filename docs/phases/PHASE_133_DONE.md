# Phase 133 完成 — 会话媒体库

## 需求分析

- 对标微信/Telegram「聊天文件」：按图片/文件/语音浏览本会话媒体。
- 验收：头部「媒体」面板；kind 筛选；图片可灯箱预览；文件可打开。

## 设计计划

- `GET /conversations/:id/media?kind=all|image|file|voice`
- 复用 messages 表按 msg_type 过滤，无需新表

## 交付

- Store `ListMedia` + Service/Handler
- ChatView 媒体面板 + chat store `loadMedia`

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- backend 重启 ✅

## 下阶段

Phase 134：消息已读详情增强（群聊已读成员列表可点击头像）
