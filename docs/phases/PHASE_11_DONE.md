# Phase 11 完成

## 需求分析
- API 文档 `GET /conversations` 未实现
- 打开聊天仅依赖 WS/sync，无历史拉取
- 验收：离线后上线应能看到历史

## 交付
- `GET /conversations` 会话摘要（含 last_seq、last_content）
- `GET /conversations/:id/messages` 按时间正序返回
- 前端打开会话时 `loadHistory()`
- 前端启动时 `loadConversations()`

## 测试
- `go build ./...` 通过
- `npm run build` 通过

## 验证
- 重新打开会话可看到历史文本消息
- 会话列表 API 返回已参与会话

## 下阶段
Phase 12: 已读/未读
