# Phase 130 完成 — 消息稍后提醒（Remind me）

## 需求分析

- 对标 Slack Later /「Remind me about this」：对某条消息设个人提醒，到期 Toast + 可在列表取消。
- 验收：消息操作「提醒 / 明早」；头部「提醒」面板；到期 WS `reminder`；可取消。

## 设计计划

- 表 `message_reminders`（pending/fired/cancelled）
- `POST /conversations/:id/messages/:msg_id/remind` + `GET/DELETE /reminders`
- 复用 schedule worker 每 5s claim due → `BroadcastReminder`

## 交付

- Migration `017_message_reminders.sql`
- Store / Service / Handler / Dispatcher
- ChatView 操作按钮 + 提醒面板；chat store API + WS

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- migration 已应用到本地 MySQL

## 下阶段

Phase 131：会话文件夹 / 自定义分组（sidebar folders）
