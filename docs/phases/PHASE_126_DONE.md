# Phase 126 完成 — 定时发送

## 需求分析

- 对标 Slack scheduled messages：指定时间自动发出文本。
- 验收：创建定时消息；列表取消；到点自动发出。

## 交付

- `015_scheduled_messages.sql`
- `GET/POST/DELETE /scheduled-messages`
- HTTP gateway 5s worker `RunScheduleWorker`
- ChatView「定时」面板

## 测试

builds ✅ · backend 重启（含 worker）✅

## 下阶段

Phase 127：导出会话记录（文本）
