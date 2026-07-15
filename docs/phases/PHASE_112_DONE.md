# Phase 112 完成 — 消息表情回应

## 需求分析

- 目标：Slack/Discord 式 emoji reaction；白名单 👍❤️😂😮😢🎉；点击切换；WS 同步。

## 设计计划

- 表 `message_reactions`（008）
- `POST .../messages/:msg_id/reactions`；列表接口附带 `reactions`
- WS `reaction` 广播

## 交付

- deploy/init/mysql/008_message_reactions.sql
- store/service/handler/push/gateway-ws
- FE chat store + ChatView 表情条

## 测试

- `go build ./...` + migration + `npm run build` + 后端重启 ✅

## 下阶段

Phase 113：好友在线状态 / last seen
