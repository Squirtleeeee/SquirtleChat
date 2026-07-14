# Phase 24 完成

## 需求分析
- 聊天页始终显示「在线」，与真实 WebSocket 状态不符
- 历史消息仅加载最近 50 条，无法查看更早记录

## 设计计划
- `WSClient` 暴露 `status`（connecting/open/closed）与 `onStatus` 回调
- `chat` store 同步 `wsStatus`，聊天页展示连接状态
- `loadMoreHistory(convId)` 使用 `before_seq` 分页加载更早消息

## 交付
- `frontend/src/api/ws.ts`：连接状态
- `frontend/src/stores/chat.ts`：`wsStatus`、`loadMoreHistory`、`historyHasMore`
- `frontend/src/views/ChatView.vue`：状态指示、「加载更早消息」

## 测试
```powershell
npm run build
# 手动：断网后状态变「未连接」；长会话点击加载更早
```

## 验证
- WS 连接成功显示「已连接」，断开显示「未连接」
- 超过 50 条消息时可加载更早历史

## 下阶段
Phase 25：消息已读回执展示 + 会话列表时间格式化
