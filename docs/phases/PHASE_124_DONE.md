# Phase 124 完成 — 个人收藏消息

## 需求分析

- 对标 Slack Later / Telegram Saved：个人收藏，不共享。
- 验收：收藏/取消；头部「收藏」列表；可跳转原文。

## 交付

- `013_user_starred_messages.sql`
- `GET /stars` · `POST .../messages/:id/star`
- 历史接口附带 `stars` map
- ChatView 收藏按钮与面板

## 测试

builds ✅ · backend 重启 ✅

## 下阶段

Phase 125：自定义状态（工作签名 / Away）
