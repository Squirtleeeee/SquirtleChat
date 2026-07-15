# Phase 114 完成 — 全局消息搜索

## 需求分析

- 现状：仅会话内搜索。
- 目标：跨所有已加入会话搜索文本消息，点击跳转。

## 设计计划

- `MessageStore.SearchGlobal`（JOIN conversation_members）
- `GET /messages/search?q=`
- 侧栏「搜记录」+ 结果列表 → `jumpToMessage`

## 交付

- backend store/service/handler
- frontend chat.searchGlobal + ChatView 侧栏 UI

## 测试

- `go build ./...` + `npm run build` + 后端重启 ✅

## 下阶段

Phase 115：@所有人 + 提及感知通知
