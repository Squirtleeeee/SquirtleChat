# Phase 148 完成 — 群公告横幅可关闭

## 需求分析

- 公告条常驻占位；对标微信可关闭，公告变更后再次显示。
- 验收：关闭后本会话隐藏；公告内容变化后重新出现。

## 交付

- ChatView：`showGroupNoticeBar` / `dismissActiveNotice`（localStorage 按群+公告内容）

## 测试

- `npm run build` ✅

## 下阶段

Phase 149：单聊「标为未读」或会话内搜索按发送者过滤
