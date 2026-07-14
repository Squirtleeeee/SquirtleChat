# Phase 12 完成

## 需求分析
- PRD MVP：未读数/已读
- 后端已有 `POST /sync/read` 与 `last_read_seq`，前端未用

## 交付
- 会话列表计算 `unread_count = last_seq - last_read_seq`
- 前端打开会话后调用 `markRead`
- 好友/群聊列表显示未读角标

## 测试
- `go build ./...` 通过
- `npm run build` 通过

## 验证
- 收到新消息后角标 +1
- 点开会话后角标清零

## 下阶段
Phase 13: 群聊完整流程
