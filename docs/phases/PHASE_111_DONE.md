# Phase 111 完成 — 跨会话真实转发

## 需求分析

- 现状：多选仅「复制转发文本」。
- 目标：选择好友/群后通过 WS 真实发送原文/图片/文件内容。

## 设计计划

- `chat.forwardMessages(target, msgs)`
- ChatView 转发选择弹层；保留「复制文本」

## 交付

- `frontend/src/stores/chat.ts` — forwardMessages
- `frontend/src/views/ChatView.vue` — 转发 picker

## 测试

- `go build ./...` + `npm run build` ✅

## 下阶段

Phase 112：消息表情回应（reactions）
