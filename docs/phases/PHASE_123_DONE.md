# Phase 123 完成 — 消息编辑

## 需求分析

- 对标 Slack/Discord：发送后短时窗口内可改文本。
- 验收：本人文本消息 15 分钟内可编辑；「已编辑」标记；WS 同步。

## 设计计划

- `messages.edited_at`
- `POST .../messages/:msg_id/edit`
- WS `type: edit`

## 交付

- `012_message_edited_at.sql` + store scan 全链路
- service/handler/BroadcastEdit
- ChatView 编辑 UI

## 测试

- builds ✅ · backend 重启 ✅

## 下阶段

Phase 124：个人收藏消息（Star / Later）
