# Phase 146 完成 — 消息翻译（LLM）

## 需求分析

- 对标微信/Telegram：文本消息一键翻译。
- 验收：会话成员可翻译文本消息；气泡下展示译文；可关闭；无 LLM 时明确错误。

## 设计计划

- `AgentService.Translate`（低温度 Chat）
- `POST /conversations/:id/messages/:msg_id/translate`
- FE：消息操作「翻译」+ 右键菜单；`translations` 缓存

## 交付

- `backend/internal/agent/llm.go` — `ChatWithTemp`
- `AgentService.Translate` / `MessageHandler.translate`
- `chat.translateMessage`；ChatView 译文区

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- API smoke（有文本消息时）

## 下阶段

Phase 147：已读回执详情增强，或群欢迎语
