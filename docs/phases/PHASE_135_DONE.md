# Phase 135 完成 — 跳到第一条未读

## 需求分析

- 对标 Telegram / 微信：打开有未读的会话定位到「以下为新消息」；可随时「跳到未读」。
- 验收：打开会话滚到未读分隔线；头部「未读」+ FAB；「回到底部」仍可用。

## 交付

- `#unread-sep` 锚点
- `jumpToFirstUnread` / `scrollToLatestOnOpen` 优先未读
- 头部按钮 + FAB「跳到未读」

## 测试

- `npm run build` ✅
- `go build ./...` ✅

## 下阶段

Phase 136：消息链接预览缓存刷新 / OG 卡片失败重试（或会话置顶排序微调）
