# Phase 120 完成 — 会话内消息置顶

## 需求分析

- 现状：仅有会话列表「置顶好友/群」；消息本身无法标记重要内容供全员查阅。
- 对标：Slack / Discord / 企业微信「Pin message」——会话级共享置顶列表 + 跳转原文。
- 验收：成员可置顶/取消；头部「置顶 N」面板；点击跳转；WS 同步其他在线端。

## 设计计划

- 表 `conversation_pins`（每会话最多 50）
- API：`GET/POST /conversations/:id/pins`，`DELETE .../pins/:msg_id`
- WS `type: pin` 推送完整 pins 列表
- ChatView：消息操作「置顶」+ 头部面板

## 交付

- `deploy/init/mysql/009_conversation_pins.sql`
- store/service/handler + `BroadcastPin`
- gateway-http 注册 `SetOnPin`（并补 `SetOnReaction` 使回应可跨端推送）
- `stores/chat.ts` + `ChatView.vue` UI

## 测试

- migration 本地应用 ✅
- `go build ./...` ✅
- `npm run build` ✅

## 验证

- [ ] 置顶文本/图片消息
- [ ] 头部面板跳转高亮
- [ ] 另一端 WS 同步取消置顶

## 下阶段

Phase 121：云端草稿同步（替代纯 localStorage drafts）
