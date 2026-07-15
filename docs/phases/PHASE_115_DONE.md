# Phase 115 完成 — @所有人 + 提及通知

## 需求分析

- 现状：群 @ 仅成员列表，无 @所有人；免打扰会吞掉被 @ 的通知。
- 目标：群聊可插入 `@所有人`；被 @ / @所有人 时即使会话免打扰仍弹桌面通知。

## 设计计划

- 提及面板首项「所有人」
- `messageMentionsMe` 解析文本（含回复信封）
- 通知门禁：`!muted || mentioned`

## 交付

- ChatView mention UI + Enter 选择
- chat store `messageMentionsMe` + notify 逻辑

## 测试

- `npm run build` + `go build ./...`

## 下阶段

Phase 116：链接预览卡片（OG unfurl）
